package pump

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"

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

type DataChangEvent struct {
	Timestamp uint32
	Schema    string
	Table     string
	Type      ChangeType
	Columns   []string
	Before    []interface{}
	After     []interface{}
}

type EventHandler func(c common.ExecContext, dce DataChangEvent) error

func OnEventReceived(handler EventHandler) {
	handlers = append(handlers, handler)
}

func PumpEvents(c common.ExecContext, streamer *replication.BinlogStreamer) error {
	isProd := common.IsProdMode()

	for {
		ev, err := streamer.GetEvent(c.Ctx)
		if err != nil {
			continue
		}

		if server.IsShuttingDown() {
			return nil
		}

		// Must run `set global binlog_row_metadata=FULL;` to include all metadata like column names and so on
		// https://dev.mysql.com/doc/refman/8.0/en/replication-options-binary-log.html#sysvar_binlog_row_metadata

		eventType := ev.Header.EventType
		c.Log.Infof("EventType: %v, event: %v", eventType, reflect.TypeOf(ev.Event))

		switch eventType {

		case replication.UPDATE_ROWS_EVENTv0, replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
			if re, ok := ev.Event.(*replication.RowsEvent); ok {
				table := re.Table
				schemaName := string(table.Schema)
				tableName := string(table.Table)

				columns := table.ColumnNameString()
				if len(columns) < 1 {
					c.Log.Errorf("Binlog doesn't provide FULL metadata, unable to parse it, %+v", re)
				} else {

					dce := DataChangEvent{
						Timestamp: ev.Header.Timestamp,
						Schema:    schemaName,
						Table:     tableName,
						Type:      TYPE_UPDATE,
						Columns:   columns,
					}

					// N is before, N + 1 is after
					for i, row := range re.Rows {
						before := (i+1)%2 != 0

						r := append([]interface{}{}, row...)

						if before {
							dce.Before = r
						} else {
							dce.After = r

							// invoke handler
							for _, handle := range handlers {
								if e := handle(c, dce); e != nil {
									return e
								}
							}
						}
					}
				}
			}

		case replication.WRITE_ROWS_EVENTv0, replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
			if re, ok := ev.Event.(*replication.RowsEvent); ok {
				table := re.Table
				schemaName := string(table.Schema)
				tableName := string(table.Table)

				columns := table.ColumnNameString()
				if len(columns) < 1 {
					c.Log.Errorf("Binlog doesn't provide FULL metadata, unable to parse it, %+v", re)
				} else {

					dce := DataChangEvent{
						Timestamp: ev.Header.Timestamp,
						Schema:    schemaName,
						Table:     tableName,
						Type:      TYPE_INSERT,
						Columns:   columns,
						After:     append([]interface{}{}, re.Rows[0]...),
					}

					// invoke handler
					for _, handle := range handlers {
						if e := handle(c, dce); e != nil {
							return e
						}
					}
				}
			}
		case replication.DELETE_ROWS_EVENTv0, replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
			if re, ok := ev.Event.(*replication.RowsEvent); ok {
				table := re.Table
				schemaName := string(table.Schema)
				tableName := string(table.Table)

				columns := table.ColumnNameString()
				if len(columns) < 1 {
					c.Log.Errorf("Binlog doesn't provide FULL metadata, unable to parse it, %+v", re)
				} else {

					dce := DataChangEvent{
						Timestamp: ev.Header.Timestamp,
						Schema:    schemaName,
						Table:     tableName,
						Type:      TYPE_DELETE,
						Columns:   columns,
						Before:    append([]interface{}{}, re.Rows[0]...),
					}

					// invoke handler
					for _, handle := range handlers {
						if e := handle(c, dce); e != nil {
							return e
						}
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
		c.Log.Infof(">>> Curr Pos: %+v, eventType: %v", curr, eventType)
		if e := updatePos(curr); e != nil {
			return e
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
