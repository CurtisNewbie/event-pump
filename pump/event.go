package pump

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/curtisnewbie/miso/core"
	mys "github.com/curtisnewbie/miso/mysql"
	red "github.com/curtisnewbie/miso/redis"
	"github.com/curtisnewbie/miso/server"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/go-redis/redis"
	"gorm.io/gorm"
)

const (
	PROP_SYNC_SERVER_ID = "sync.server-id"
	PROP_SYNC_HOST      = "sync.host"
	PROP_SYNC_PORT      = "sync.port"
	PROP_SYNC_USER      = "sync.user"
	PROP_SYNC_PASSWORD  = "sync.password"

	flavorMysql = "mysql"

	lastPosKey = "event-pump:pos:last"

	TYPE_INSERT = "INS"
	TYPE_UPDATE = "UPD"
	TYPE_DELETE = "DEL"
)

var (
	logFileName           = ""
	handlers              = []EventHandler{}
	tableInfoMap          = make(map[string]TableInfo)
	conn         *gorm.DB = nil

	_globalInclude *regexp.Regexp = nil
	_globalExclude *regexp.Regexp = nil
)

func init() {
	core.SetDefProp(PROP_SYNC_SERVER_ID, 100)
	core.SetDefProp(PROP_SYNC_HOST, "127.0.0.1")
	core.SetDefProp(PROP_SYNC_PORT, 3306)
	core.SetDefProp(PROP_SYNC_USER, "root")
	core.SetDefProp(PROP_SYNC_PASSWORD, "")
}

type Record struct {
	Before []interface{} `json:"before"`
	After  []interface{} `json:"after"`
}

type DataChangeEvent struct {
	Timestamp uint32         `json:"timestamp"` // epoc time second
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

type EventHandler func(c core.Rail, dce DataChangeEvent) error

func HasAnyEventHandler() bool {
	return len(handlers) > 0
}

func OnEventReceived(handler EventHandler) {
	handlers = append(handlers, handler)
}

func callEventHandlers(c core.Rail, dce DataChangeEvent) error {
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

func FetchTableInfo(c core.Rail, schema string, table string) (TableInfo, error) {
	var columns []ColumnInfo
	e := conn.
		Table("information_schema.columns").
		Select("column_name COLUMN_NAME, ordinal_position ORDINAL_POSITION, data_type DATA_TYPE").
		Where("table_schema = ? AND table_name = ?", schema, table).
		Order("ordinal_position asc").
		Scan(&columns).Error
	return TableInfo{Table: table, Schema: schema, Columns: columns}, e
}

func ResetTableInfoCache(c core.Rail, schema string, table string) {
	k := schema + "." + table
	delete(tableInfoMap, k)
	c.Infof("Reset TableInfo cache, %v.%v", schema, table)
}

func CachedTableInfo(c core.Rail, schema string, table string) (TableInfo, error) {
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

func PumpEvents(c core.Rail, syncer *replication.BinlogSyncer, streamer *replication.BinlogStreamer) error {
	isProd := core.IsProdMode()

	for {
		ev, err := streamer.GetEvent(c.Ctx)
		if err != nil {
			c.Errorf("GetEvent returned error, %v", err)
			continue // retry GetEvent
		}
		if !isProd {
			ev.Dump(os.Stdout)
		}

		/*
			We are not using Table.ColumnNameString() to resolve the actual column names, the column names are actually
			fetched from the master instance using simple queries.

			e.g.,

				ev.Event.(*replication.RowsEvent).Table.ColumnNameString()

			It's not very useful, it requires `binlog_row_metadata=FULL` and MySQL >= 8.0

			https://dev.mysql.com/doc/refman/8.0/en/replication-options-binary-log.html#sysvar_binlog_row_metadata

			TODO: The code is quite redundant, refactor it
		*/

		switch ev.Header.EventType {

		case replication.TABLE_MAP_EVENT:

			// the table may be changed, reset the cache
			if tme, ok := ev.Event.(*replication.TableMapEvent); ok {
				ResetTableInfoCache(c, string(tme.Schema), string(tme.Table))
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
				dce.Type = TYPE_UPDATE
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
				dce.Type = TYPE_INSERT

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
				dce.Type = TYPE_DELETE

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

		// update position on redis
		if e := updatePos(c, mysql.Position{Name: logFileName, Pos: logPos}); e != nil {
			return e
		}

		if server.IsShuttingDown() {
			c.Info("Server shutting down")
			return nil
		}
	}
}

func updatePos(c core.Rail, pos mysql.Position) error {
	c.Infof("Curr Pos: %+v", pos)
	s, e := json.Marshal(&pos)
	if e != nil {
		return e
	}

	sc := red.GetRedis().Set(lastPosKey, []byte(s), 0)
	return sc.Err()
}

func lastPos(c core.Rail) (mysql.Position, error) {

	sc := red.GetRedis().Get(lastPosKey)
	if sc.Err() != nil {
		if errors.Is(sc.Err(), redis.Nil) {
			return mysql.Position{}, nil
		}
		return mysql.Position{}, sc.Err()
	}

	s := sc.Val()
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

func NewStreamer(c core.Rail, syncer *replication.BinlogSyncer) (*replication.BinlogStreamer, error) {
	pos, err := lastPos(c)
	if err != nil {
		return nil, err
	}
	return syncer.StartSync(pos)
}

func PrepareSync(rail core.Rail) (*replication.BinlogSyncer, error) {
	cfg := replication.BinlogSyncerConfig{
		ServerID: uint32(core.GetPropInt(PROP_SYNC_SERVER_ID)),
		Flavor:   flavorMysql,
		Host:     core.GetPropStr(PROP_SYNC_HOST),
		Port:     uint16(core.GetPropInt(PROP_SYNC_PORT)),
		User:     core.GetPropStr(PROP_SYNC_USER),
		Password: core.GetPropStr(PROP_SYNC_PASSWORD),
		Logger:   rail.Logger(),
	}

	client, err := mys.NewConn(
		core.GetPropStr(PROP_SYNC_USER),
		core.GetPropStr(PROP_SYNC_PASSWORD),
		"",
		core.GetPropStr(PROP_SYNC_HOST),
		core.GetPropStr(PROP_SYNC_PORT),
		"",
	)
	if err != nil {
		return nil, err
	}
	conn = client
	if !core.IsProdMode() {
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
