package pump

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/curtisnewbie/miso/miso"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"gorm.io/gorm"
)

const (
	PropSyncServerId     = "sync.server-id"
	PropSyncHost         = "sync.host"
	PropSyncPort         = "sync.port"
	PropSyncUser         = "sync.user"
	PropSyncPassword     = "sync.password"
	PropSyncPosFile      = "sync.pos.file"
	PropSyncMaxReconnect = "sync.max-reconnect"

	flavorMysql = "mysql"
	lastPosKey  = "event-pump:pos:last"

	TypeInsert = "INS"
	TypeUpdate = "UPD"
	TypeDelete = "DEL"
)

var (
	currPos mysql.Position = mysql.Position{Name: "", Pos: 0}
	nextPos mysql.Position = currPos
	posMu   sync.Mutex

	// posFile is flushed in every 500ms (at most)
	updatePosFileTicker *miso.TickRunner = miso.NewTickRuner(time.Millisecond*500, FlushPosFile)

	posFile      *os.File = nil
	logFileName           = ""
	handlers              = []EventHandler{}
	tableInfoMap          = make(map[string]TableInfo)
	conn         *gorm.DB = nil

	_globalInclude *regexp.Regexp = nil
	_globalExclude *regexp.Regexp = nil
)

var (
	resyncErrCount int32 = 0
)

func init() {
	miso.SetDefProp(PropSyncServerId, 100)
	miso.SetDefProp(PropSyncHost, "127.0.0.1")
	miso.SetDefProp(PropSyncPort, 3306)
	miso.SetDefProp(PropSyncUser, "root")
	miso.SetDefProp(PropSyncPassword, "")
	miso.SetDefProp(PropSyncPosFile, "binlog_pos")
	miso.SetDefProp(PropSyncMaxReconnect, 120)
}

type Record struct {
	Before []interface{} `json:"before"`
	After  []interface{} `json:"after"`
}

type DataChangeEvent struct {
	Timestamp uint32         `json:"timestamp"` // epoch time second
	Schema    string         `json:"schema"`
	Table     string         `json:"table"`
	Type      string         `json:"type"` // INS-INSERT, UPD-UPDATE, DEL-DELETE
	Records   []Record       `json:"records"`
	Columns   []RecordColumn `json:"columns"`
}

type RecordColumn struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

func (d DataChangeEvent) String() string {
	rs := []string{}
	for _, r := range d.Records {
		rs = append(rs, d.PrintRecord(r))
	}
	joinedRecords := strings.Join(rs, ", ")
	return fmt.Sprintf("DataChangeEvent{ Timestamp: %v, Schema: %v, Table: %v, Type: %v, Records: [ %v ] }",
		d.Timestamp, d.Schema, d.Table, d.Type, joinedRecords)
}

func (d DataChangeEvent) PrintRecord(r Record) string {
	bef := d.rowToStr(r.Before)
	aft := d.rowToStr(r.After)
	return fmt.Sprintf("{ before: %v, after: %v }", bef, aft)
}

func (d DataChangeEvent) getColName(j int) string {
	if j < len(d.Columns) {
		return d.Columns[j].Name
	}
	return ""
}

func (d DataChangeEvent) rowToStr(row []interface{}) string {
	sl := []string{}
	for i, v := range row {
		sl = append(sl, fmt.Sprintf("%v:%v", d.getColName(i), v))
	}
	return "{ " + strings.Join(sl, ", ") + " }"
}

type EventHandler func(c miso.Rail, dce DataChangeEvent) error

func HasAnyEventHandler() bool {
	return len(handlers) > 0
}

func OnEventReceived(handler EventHandler) {
	handlers = append(handlers, handler)
}

func callEventHandlers(c miso.Rail, dce DataChangeEvent) error {
	for _, handle := range handlers {
		if e := handle(c, dce); e != nil {
			return e
		}
	}
	return nil
}

func newDataChangeEvent(table TableInfo, re *replication.RowsEvent, timestamp uint32) DataChangeEvent {
	cn := []RecordColumn{}
	for _, ci := range table.Columns {
		cn = append(cn, RecordColumn{Name: ci.ColumnName, DataType: ci.DataType})
	}
	return DataChangeEvent{
		Timestamp: timestamp,
		Schema:    table.Schema,
		Table:     table.Table,
		Records:   []Record{},
		Columns:   cn,
	}
}

type TableInfo struct {
	Schema  string
	Table   string
	Columns []ColumnInfo
}

type ColumnInfo struct {
	ColumnName      string `gorm:"column:COLUMN_NAME"`
	DataType        string `gorm:"column:DATA_TYPE"`
	OrdinalPosition int    `gorm:"column:ORDINAL_POSITION"`
}

func FetchTableInfo(c miso.Rail, schema string, table string) (TableInfo, error) {
	var columns []ColumnInfo
	e := conn.
		Table("information_schema.columns").
		Select("column_name COLUMN_NAME, ordinal_position ORDINAL_POSITION, data_type DATA_TYPE").
		Where("table_schema = ? AND table_name = ?", schema, table).
		Order("ordinal_position asc").
		Scan(&columns).Error
	return TableInfo{Table: table, Schema: schema, Columns: columns}, e
}

func ResetTableInfoCache(c miso.Rail, schema string, table string) {
	k := schema + "." + table
	delete(tableInfoMap, k)
	c.Infof("Reset TableInfo cache, %v.%v", schema, table)
}

func CachedTableInfo(c miso.Rail, schema string, table string) (TableInfo, error) {
	k := schema + "." + table
	ti, ok := tableInfoMap[k]
	if ok {
		return ti, nil
	}

	fti, e := FetchTableInfo(c, schema, table)
	if e != nil {
		return TableInfo{}, e
	}

	tableInfoMap[k] = fti
	return fti, nil
}

func PumpEvents(c miso.Rail, syncer *replication.BinlogSyncer, streamer *replication.BinlogStreamer) error {
	for {
		select {
		case <-c.Context().Done():
			c.Info("Context cancelled, exiting PumpEvents()")
			return nil
		default:
			ev, err := streamer.GetEvent(c.Context())
			if err != nil {
				c.Errorf("GetEvent returned error, %v", err)
				if errors.Is(err, replication.ErrNeedSyncAgain) {
					if atomic.AddInt32(&resyncErrCount, 1) > 9 {
						return err
					}
				}
				continue // retry GetEvent
			}

			atomic.StoreInt32(&resyncErrCount, 0) // reset the err count
			evtLogBuf := strings.Builder{}
			ev.Dump(&evtLogBuf)
			c.Info(evtLogBuf.String())

			/*
				We are not using Table.ColumnNameString() to resolve the actual column names, the column names are actually
				fetched from the master instance using simple queries.

				e.g.,

					ev.Event.(*replication.RowsEvent).Table.ColumnNameString()

				It's not very useful, it requires `binlog_row_metadata=FULL` and MySQL >= 8.0

				https://dev.mysql.com/doc/refman/8.0/en/replication-options-binary-log.html#sysvar_binlog_row_metadata

				TODO: The code is quite redundant, refactor it

				About events:

					https://dev.mysql.com/doc/dev/mysql-server/latest/classbinary__log_1_1Table__map__event.html
			*/

			switch ev.Header.EventType {

			case replication.QUERY_EVENT:

				// the table may be changed, reset the cache
				if qe, ok := ev.Event.(*replication.QueryEvent); ok {

					// parse the table
					if table, ok := parseAlterTable(string(qe.Query)); ok {
						ResetTableInfoCache(c, string(qe.Schema), table)
					}
				}
				continue

			case replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:

				if re, ok := ev.Event.(*replication.RowsEvent); ok {

					schema := string(re.Table.Schema)
					if !includeSchema(schema) {
						goto event_handle_end
					}

					tableInfo, e := CachedTableInfo(c, schema, string(re.Table.Table))
					if e != nil {
						return e
					}

					dce := newDataChangeEvent(tableInfo, re, ev.Header.Timestamp)
					dce.Type = TypeUpdate
					rec := Record{}

					// N is before, N + 1 is after
					for i, row := range re.Rows {
						before := (i+1)%2 != 0
						if before {
							rec.Before = row
						} else {
							rec.After = row
							dce.Records = append(dce.Records, rec)
							rec = Record{}
						}
					}

					if e := callEventHandlers(c, dce); e != nil {
						return e
					}
				}

			case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:

				if re, ok := ev.Event.(*replication.RowsEvent); ok {

					schema := string(re.Table.Schema)
					if !includeSchema(schema) {
						goto event_handle_end
					}

					tableInfo, e := CachedTableInfo(c, schema, string(re.Table.Table))
					if e != nil {
						return e
					}

					dce := newDataChangeEvent(tableInfo, re, ev.Header.Timestamp)
					dce.Type = TypeInsert

					for _, row := range re.Rows {
						dce.Records = append(dce.Records, Record{After: row})
					}

					if e := callEventHandlers(c, dce); e != nil {
						return e
					}
				}
			case replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
				if re, ok := ev.Event.(*replication.RowsEvent); ok {
					schema := string(re.Table.Schema)
					if !includeSchema(schema) {
						goto event_handle_end
					}

					tableInfo, e := CachedTableInfo(c, schema, string(re.Table.Table))
					if e != nil {
						return e
					}
					dce := newDataChangeEvent(tableInfo, re, ev.Header.Timestamp)
					dce.Type = TypeDelete

					for _, row := range re.Rows {
						dce.Records = append(dce.Records, Record{Before: row})
					}

					if e := callEventHandlers(c, dce); e != nil {
						return e
					}
				}
			}

			// end of event handling, we are mainly handling log pos here
		event_handle_end:

			// in most cases, lostPos is on event header
			var logPos uint32

			// we don't always update pos on all events, even though some of them have position
			// if we update whenever we can, we may end up being stuck somewhere the next time we
			// startup the app again
			switch t := ev.Event.(type) {

			// for RotateEvent, LogPosition can be 0, have to use Position instead
			case *replication.RotateEvent:
				logPos = uint32(t.Position)
				logFileName = string(t.NextLogName)

			/*
				- QueryEvent if some DDL is executed
				- the go-mysql-elasticsearch also update it's pos on XIDEvent

				according to the doc: "An XID event is generated for a commit of a transaction that modifies one or more tables of an XA-capable storage engine"
				https://dev.mysql.com/doc/dev/mysql-server/latest/classXid__log__event.html

				it does seems like it's the 2PC thing for between the server and innodb engine in binlog
			*/
			case *replication.QueryEvent, *replication.XIDEvent:
				logPos = ev.Header.LogPos

			// this event shouldn't update our log pos
			default:
				continue
			}

			// update position
			updatePos(c, mysql.Position{Name: logFileName, Pos: logPos})

			if miso.IsShuttingDown() {
				c.Info("Server shutting down")
				return nil
			}

		}

	}
}

func updatePos(c miso.Rail, pos mysql.Position) {
	c.Infof("Next pos: %+v", pos)
	posMu.Lock()
	defer posMu.Unlock()
	nextPos = pos
}

func lastPos(c miso.Rail) (mysql.Position, error) {
	buf, err := io.ReadAll(posFile)
	if err != nil {
		return mysql.Position{}, fmt.Errorf("failed to read pos file, %w", err)
	}
	s := string(buf)
	if s == "" {
		return mysql.Position{}, nil
	}

	var pos mysql.Position
	e := json.Unmarshal([]byte(s), &pos)
	if e != nil {
		return mysql.Position{}, e
	}

	c.Infof("Last position: %v - %v", pos.Name, pos.Pos)
	return pos, nil
}

func NewStreamer(c miso.Rail, syncer *replication.BinlogSyncer) (*replication.BinlogStreamer, error) {
	pos, err := lastPos(c)
	if err != nil {
		return nil, err
	}
	return syncer.StartSync(pos)
}

func PrepareSync(rail miso.Rail) (*replication.BinlogSyncer, error) {
	cfg := replication.BinlogSyncerConfig{
		ServerID:             uint32(miso.GetPropInt(PropSyncServerId)),
		Flavor:               flavorMysql,
		Host:                 miso.GetPropStr(PropSyncHost),
		Port:                 uint16(miso.GetPropInt(PropSyncPort)),
		User:                 miso.GetPropStr(PropSyncUser),
		Password:             miso.GetPropStr(PropSyncPassword),
		MaxReconnectAttempts: miso.GetPropInt(PropSyncMaxReconnect),
		Logger:               rail,
	}

	p := miso.MySQLConnParam{
		User:     miso.GetPropStr(PropSyncUser),
		Password: miso.GetPropStr(PropSyncPassword),
		Host:     miso.GetPropStr(PropSyncHost),
		Port:     miso.GetPropInt(PropSyncPort),
	}
	client, err := miso.NewMySQLConn(rail, p)
	if err != nil {
		return nil, err
	}
	conn = client
	if !miso.IsProdMode() {
		conn = conn.Debug()
	}

	return replication.NewBinlogSyncer(cfg), nil
}

func includeSchema(schema string) bool {
	if _globalExclude != nil && _globalExclude.MatchString(schema) { // exclude specified and matched
		return false
	}
	if _globalInclude != nil && !_globalInclude.MatchString(schema) { // include specified, but doesn't match
		return false
	}
	return true
}

func SetGlobalInclude(r *regexp.Regexp) {
	_globalInclude = r
}

func SetGlobalExclude(r *regexp.Regexp) {
	_globalExclude = r
}

var alterTableRegex = regexp.MustCompile(`(?i)^\s*alter table ([\w_\d]+) .*$`)

func parseAlterTable(sql string) (string, bool) {
	matched := alterTableRegex.FindStringSubmatch(sql)
	if matched == nil {
		return "", false
	}

	return matched[1], true
}

func AttachPosFile(rail miso.Rail) error {
	pf := miso.GetPropStr(PropSyncPosFile)
	rail.Infof("Attaching to pos file: %v", pf)
	f, err := miso.OpenFile(pf, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return fmt.Errorf("failed to attach to pos file: %v, %w", pf, err)
	}
	posFile = f
	rail.Infof("Attached to pos file: %v", pf)

	// start ticker to periodically flush posFile
	updatePosFileTicker.Start()
	return nil
}

func DetachPosFile(rail miso.Rail) {
	if posFile == nil {
		return
	}
	FlushPosFile()
	posFile.Close()
	posFile = nil
}

func FlushPosFile() {
	posMu.Lock()
	defer posMu.Unlock()
	if currPos.Name == nextPos.Name && currPos.Pos == nextPos.Pos {
		return
	}

	s, e := json.Marshal(nextPos)
	if e != nil {
		miso.Errorf("failed to update posFile, unable to marshal pos %+v, %v", nextPos, e)
		return
	}
	posFile.Truncate(0)
	if _, err := posFile.WriteAt([]byte(s), 0); err != nil {
		miso.Errorf("failed to write posFile, content: %s, %v", s, e)
		return
	}
	if err := posFile.Sync(); err != nil {
		miso.Errorf("failed to fsync posFile, content: %s, %v", s, e)
		return
	}

	miso.Infof("pos moved from %+v to %+v", currPos, nextPos)
	currPos = nextPos
}
