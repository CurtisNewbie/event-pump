# event-pump

Simple app to parse and stream MySQL binlog event in real time. It's Powered by `github.com/go-mysql-org/go-mysql`.

- Tested on MySQL 8.0.23 / 5.7

## Requirements

- MySQL
- RabbitMQ

MySQL must enable binlog replication (it's enabled by default on MySQL 8.x).

```conf
# /etc/mysql/my.cnf

[mysqld]
  server_id=1
  log_bin=binlog
```

## Configuration

For more configuration, check [miso](https://github.com/CurtisNewbie/miso).

| Property                              | Description                                                                                                                                      | Default Value |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ------------- |
| sync.server-id                        | server-id used to mimic a replication server                                                                                                     | 100           |
| sync.user                             | username of the master MySQL instance                                                                                                            | root          |
| sync.password                         | password of the master MySQL instance                                                                                                            |               |
| sync.host                             | host of the master MySQL instance                                                                                                                | 127.0.0.1     |
| sync.port                             | port of the master MySQL instance                                                                                                                | 3306          |
| sync.pos.file                         | binlog position file **(be careful if you are upgrading event-pump)**                                                                            | binlog_pos    |
| sync.max-reconnect                    | max reconnect attempts (reconnect every second, 0 means infinite retry)                                                                          | 120           |
| filter.include                        | regexp for filtering schema names, if specified, only thoes thare are matched are included                                                       |               |
| filter.exclude                        | regexp for filtering schema names, if specified, thoes that thare are matched are excluded, `exclude` filter is executed before `include` filter |               |
| []pipeline                            | list of pipeline config                                                                                                                          |               |
| []pipeline.schema                     | regexp for matching schema name                                                                                                                  |               |
| []pipeline.table                      | regexp for matching table name                                                                                                                   |               |
| []pipeline.type                       | regexp for matching event type (optional)                                                                                                        |               |
| []pipeline.stream                     | event bus name (basically, the event is sent to a rabbitmq exchange identified by name `${pipeline.stream}` using routing key `'#'`)             |               |
| []pipeline.enabled                    | whether it's enabled                                                                                                                             |               |
| []pipeline.condition.[]column-changed | Filter events that contain changes to the specified columns                                                                                      |               |

## Configuration Example

```yaml
filter:
  include: "^(my_db|another_db)$"
  exclude: "^(system_db)$"

pipeline:
  - schema: ".*"
    table: ".*"
    type: "(INS|UPD)"
    stream: "data-change.echo"
    enabled: true
```

### Event Structure

The event message can be unmarshalled (from json) using following structs. Each event only contain changes to one single record, even though multiple records may be changed within the same transaction. It's more natural to use this structure when the receiver wants to react to the event and do some business logic.

```go
type StreamEvent struct {
	Timestamp uint32                       `json:"timestamp"` // epoch time second
	Schema    string                       `json:"schema"`
	Table     string                       `json:"table"`
	Type      string                       `json:"type"`    // INS-INSERT, UPD-UPDATE, DEL-DELETE
	Columns   map[string]StreamEventColumn `json:"columns"` // key is the column name
}

type StreamEventColumn struct {
	DataType string `json:"dataType"`
	Before   string `json:"before"`
	After    string `json:"after"`
}
```

E.g.,

```json
{
  "timestamp": 1688199982,
  "schema": "my_db",
  "table": "my_table",
  "type": "INS",
  "columns": {
    "id": {
      "dataType": "int",
      "before": "1",
      "after": "1"
    },
    "name": {
      "dataType": "varchar",
      "before": "banana",
      "after": "apple"
    }
  }
}
```

# Update

- Since v0.0.5, event-pump no longer depends on redis, binlog position is now recorded in a local file, using following format (previously, it's recorded on redis):

  ```
  {"Name":"binlog.000001","Pos":53318}
  ```

  If you are upgrading event-pump to >= v0.0.5, you should prepare this position file manually. You can retrieve previous binlog position using redis-cli:

  ```sh
  get "event-pump:pos:last"
  # "{\"Name\":\"binlog.000001\",\"Pos\":53318}"
  ```

  Then write the content to the position file.

# Maintenance

To recover from earliest binlog position:

Connect master instance to query the earilest binlog file name and position:

```sh
show binlog events limit 1;

# +------------------+-----+-------------+-----------+-------------+-----------------------------------+
# | Log_name         | Pos | Event_type  | Server_id | End_log_pos | Info                              |
# +------------------+-----+-------------+-----------+-------------+-----------------------------------+
# | mysql-bin.000292 |   4 | Format_desc |  ******** |         126 | Server ver: 8.0.36, Binlog ver: 4 |
# +------------------+-----+-------------+-----------+-------------+-----------------------------------+
```

Then update the binlog name and position back to the `binlog_pos` file, and then restart event-pump.


# Todo

- [ ] Supports HA using ZooKeeper.