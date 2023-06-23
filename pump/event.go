package pump

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/curtisnewbie/gocommon/common"
	red "github.com/curtisnewbie/gocommon/redis"
	"github.com/curtisnewbie/gocommon/server"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/go-redis/redis"
)

const (
	PROP_SYNC_SERVER_ID = "prop.sync.server-id"
	PROP_SYNC_HOST      = "prop.sync.host"
	PROP_SYNC_PORT      = "prop.sync.port"
	PROP_SYNC_USER      = "prop.sync.user"
	PROP_SYNC_PASSWORD  = "prop.sync.password"

	flavorMysql = "mysql"

	lastPosKey = "event-pump:pos:last"

	TYPE_INSERT ChangeType = "INS"
	TYPE_UPDATE ChangeType = "UPD"
	TYPE_DELETE ChangeType = "DEL"
)

var (
	logFileName = ""
	handlers    = []EventHandler{}
)

func init() {
	common.SetDefProp(PROP_SYNC_SERVER_ID, 100)
	common.SetDefProp(PROP_SYNC_HOST, "127.0.0.1")
	common.SetDefProp(PROP_SYNC_PORT, 3306)
	common.SetDefProp(PROP_SYNC_USER, "root")
	common.SetDefProp(PROP_SYNC_PASSWORD, "")
}

type ChangeType string

type Record struct {
	Before []interface{}
	After  []interface{}
}

type DataChangeEvent struct {
	Time      time.Time
	Timestamp uint32
	Schema    string
	Table     string
	Type      ChangeType
	Records   []Record
	Columns   []string
}

func (d DataChangeEvent) String() string {
	rs := []string{}
	for _, r := range d.Records {
		rs = append(rs, d.PrintRecord(r))
	}
	joinedRecords := strings.Join(rs, ", ")
	return fmt.Sprintf("DataChangeEvent{ Timestamp: %v (%s), Schema: %v, Table: %v, Type: %v, Records: [ %v ] }",
		d.Timestamp, d.Time, d.Schema, d.Table, d.Type, joinedRecords)
}

func (d DataChangeEvent) PrintRecord(r Record) string {
	bef := d.rowToStr(r.Before)
	aft := d.rowToStr(r.After)
	return fmt.Sprintf("{ before: %v, after: %v }", bef, aft)
}

func (d DataChangeEvent) getColName(j int) string {
	if j < len(d.Columns) {
		return d.Columns[j]
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

func newDataChangeEvent(ev *replication.BinlogEvent, re *replication.RowsEvent) DataChangeEvent {
	table := re.Table
	schemaName := string(table.Schema)
	tableName := string(table.Table)
	return DataChangeEvent{
		Timestamp: ev.Header.Timestamp,
		Schema:    schemaName,
		Table:     tableName,
		Time:      time.Unix(int64(ev.Header.Timestamp), 0),
		Columns:   re.Table.ColumnNameString(),
		Records:   []Record{},
	}
}

func PumpEvents(c common.ExecContext, streamer *replication.BinlogStreamer) error {
	isProd := common.IsProdMode()

	for {
		ev, err := streamer.GetEvent(c.Ctx)
		if err != nil {
			continue // retry GetEvent
		}

		// Must run `set global binlog_row_metadata=FULL;` or configure it in option file to include all metadata like column names and so on
		// https://dev.mysql.com/doc/refman/8.0/en/replication-options-binary-log.html#sysvar_binlog_row_metadata

		eventType := ev.Header.EventType
		c.Log.Debugf("EventType: %v, event: %v", eventType, reflect.TypeOf(ev.Event))

		switch eventType {

		case replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
			if re, ok := ev.Event.(*replication.RowsEvent); ok {
				columns := re.Table.ColumnNameString()
				if len(columns) < 1 {
					return fmt.Errorf("binlog doesn't provide FULL metadata, unable to parse it, %+v", re)
				} else {

					dce := newDataChangeEvent(ev, re)
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
			}

		case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
			if re, ok := ev.Event.(*replication.RowsEvent); ok {
				columns := re.Table.ColumnNameString()
				if len(columns) < 1 {
					return fmt.Errorf("binlog doesn't provide FULL metadata, unable to parse it, %+v", re)
				} else {

					dce := newDataChangeEvent(ev, re)
					dce.Type = TYPE_INSERT

					for _, row := range re.Rows {
						dce.Records = append(dce.Records, Record{
							After: row,
						})
					}

					if e := callEventHandlers(c, dce); e != nil {
						return e
					}
				}
			}
		case replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
			if re, ok := ev.Event.(*replication.RowsEvent); ok {
				columns := re.Table.ColumnNameString()
				if len(columns) < 1 {
					return fmt.Errorf("binlog doesn't provide FULL metadata, unable to parse it, %+v", re)
				} else {

					dce := newDataChangeEvent(ev, re)
					dce.Type = TYPE_DELETE

					for _, row := range re.Rows {
						dce.Records = append(dce.Records, Record{
							Before: row,
						})
					}

					if e := callEventHandlers(c, dce); e != nil {
						return e
					}
				}
			}
		}

		if !isProd {
			ev.Dump(os.Stdout)
		}

		if _, ok := ev.Event.(*replication.FormatDescriptionEvent); ok {
			continue // it doesn't have position at all, LogPos is always 0
		}

		logPos := ev.Header.LogPos

		// for RotateEvent, LogPosition can be 0, have to use Position instead
		if re, ok := ev.Event.(*replication.RotateEvent); ok {
			logPos = uint32(re.Position)
			logFileName = string(re.NextLogName)
		}

		curr := mysql.Position{Name: logFileName, Pos: logPos}
		c.Log.Infof("Curr Pos: %+v, eventType: %v", curr, eventType)
		if e := updatePos(curr); e != nil {
			return e
		}

		if server.IsShuttingDown() {
			return nil
		}
	}
}

func updatePos(pos mysql.Position) error {
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

func NewSyncer(c common.ExecContext) (*replication.BinlogSyncer, error) {
	cfg := replication.BinlogSyncerConfig{
		ServerID: uint32(common.GetPropInt(PROP_SYNC_SERVER_ID)),
		Flavor:   flavorMysql,
		Host:     common.GetPropStr(PROP_SYNC_HOST),
		Port:     uint16(common.GetPropInt(PROP_SYNC_PORT)),
		User:     common.GetPropStr(PROP_SYNC_USER),
		Password: common.GetPropStr(PROP_SYNC_PASSWORD),
		Logger:   c.Log,
	}
	return replication.NewBinlogSyncer(cfg), nil
}
