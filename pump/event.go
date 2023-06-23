package pump

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/curtisnewbie/gocommon/common"
	mys "github.com/curtisnewbie/gocommon/mysql"
	red "github.com/curtisnewbie/gocommon/redis"
	"github.com/curtisnewbie/gocommon/server"
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
)

func init() {
	common.SetDefProp(PROP_SYNC_SERVER_ID, 100)
	common.SetDefProp(PROP_SYNC_HOST, "127.0.0.1")
	common.SetDefProp(PROP_SYNC_PORT, 3306)
	common.SetDefProp(PROP_SYNC_USER, "root")
	common.SetDefProp(PROP_SYNC_PASSWORD, "")
}

type Record struct {
	Before []interface{} `json:"before"`
	After  []interface{} `json:"after"`
}

type DataChangeEvent struct {
	Timestamp uint32         `json:"timestamp"`
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

type EventHandler func(c common.ExecContext, dce DataChangeEvent) error

func HasAnyEventHandler() bool {
	return len(handlers) > 0
}

func OnEventReceived(handler EventHandler) {
	handlers = append(handlers, handler)
}

func callEventHandlers(c common.ExecContext, dce DataChangeEvent) error {
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

func FetchTableInfo(c common.ExecContext, schema string, table string) (TableInfo, error) {
	var columns []ColumnInfo
	e := conn.
		Table("information_schema.columns").
		Select("column_name, ordinal_position, data_type").
		Where("table_schema = ? AND table_name = ?", schema, table).
		Order("ordinal_position asc").
		Scan(&columns).Error
	return TableInfo{Table: table, Schema: schema, Columns: columns}, e
}

func ResetTableInfoCache(c common.ExecContext, schema string, table string) {
	k := schema + "." + table
	delete(tableInfoMap, k)
	c.Log.Infof("Reset TableInfo cache, %v.%v", schema, table)
}

func CachedTableInfo(c common.ExecContext, schema string, table string) (TableInfo, error) {
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

func PumpEvents(c common.ExecContext, syncer *replication.BinlogSyncer, streamer *replication.BinlogStreamer) error {
	isProd := common.IsProdMode()

	for {
		ev, err := streamer.GetEvent(c.Ctx)
		if err != nil {
			c.Log.Errorf("GetEvent returned error, %v", err)
			continue // retry GetEvent
		}
		if !isProd {
			ev.Dump(os.Stdout)
		}

		/*
			We are using Table.ColumnNameString() to resolve the actual column names, the column names are actually
			fetched from the master instance using simple queries.

			e.g.,

				ev.Event.(*replication.RowsEvent).Table.ColumnNameString()

			It's not very useful, it requires `binlog_row_metadata=FULL` and MySQL >= 8.0

			https://dev.mysql.com/doc/refman/8.0/en/replication-options-binary-log.html#sysvar_binlog_row_metadata
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
				tableInfo, e := CachedTableInfo(c, string(re.Table.Schema), string(re.Table.Table))
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
				tableInfo, e := CachedTableInfo(c, string(re.Table.Schema), string(re.Table.Table))
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
				tableInfo, e := CachedTableInfo(c, string(re.Table.Schema), string(re.Table.Table))
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

		if _, ok := ev.Event.(*replication.FormatDescriptionEvent); ok {
			continue // it doesn't have position at all, LogPos is always 0
		}

		// for RotateEvent, LogPosition can be 0, have to use Position instead
		logPos := ev.Header.LogPos
		if re, ok := ev.Event.(*replication.RotateEvent); ok {
			logPos = uint32(re.Position)
			logFileName = string(re.NextLogName)
		}

		// update position on redis
		if e := updatePos(c, mysql.Position{Name: logFileName, Pos: logPos}); e != nil {
			return e
		}

		if server.IsShuttingDown() {
			c.Log.Info("Server shutting down")
			return nil
		}
	}
}

func updatePos(c common.ExecContext, pos mysql.Position) error {
	c.Log.Infof("Curr Pos: %+v", pos)
	s, e := json.Marshal(&pos)
	if e != nil {
		return e
	}

	sc := red.GetRedis().Set(lastPosKey, []byte(s), 0)
	return sc.Err()
}

func lastPos(c common.ExecContext) (mysql.Position, error) {

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

	c.Log.Infof("Last position: %v - %v", pos.Name, pos.Pos)
	return pos, nil
}

func NewStreamer(c common.ExecContext, syncer *replication.BinlogSyncer) (*replication.BinlogStreamer, error) {
	pos, err := lastPos(c)
	if err != nil {
		return nil, err
	}
	return syncer.StartSync(pos)
}

func PrepareSync(c common.ExecContext) (*replication.BinlogSyncer, error) {
	cfg := replication.BinlogSyncerConfig{
		ServerID: uint32(common.GetPropInt(PROP_SYNC_SERVER_ID)),
		Flavor:   flavorMysql,
		Host:     common.GetPropStr(PROP_SYNC_HOST),
		Port:     uint16(common.GetPropInt(PROP_SYNC_PORT)),
		User:     common.GetPropStr(PROP_SYNC_USER),
		Password: common.GetPropStr(PROP_SYNC_PASSWORD),
		Logger:   c.Log,
	}

	client, err := mys.NewConn(
		common.GetPropStr(PROP_SYNC_USER),
		common.GetPropStr(PROP_SYNC_PASSWORD),
		"",
		common.GetPropStr(PROP_SYNC_HOST),
		common.GetPropStr(PROP_SYNC_PORT),
		"",
	)
	if err != nil {
		return nil, err
	}
	conn = client
	if !common.IsProdMode() {
		conn = conn.Debug()
	}

	return replication.NewBinlogSyncer(cfg), nil
}
