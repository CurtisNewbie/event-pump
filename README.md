# event-pump

Simple app to parse and stream MySQL binlog event in real time. It's Powered by `github.com/go-mysql-org/go-mysql`.

- Tested on MySQL 8.0.23 / 5.7

## Requirements

- MySQL
- RabbitMQ
- ZooKeeper (if HA mode is enabled)

MySQL must enable binlog replication (it's enabled by default on MySQL 8.x).

```conf
# /etc/mysql/my.cnf

[mysqld]
  server_id=1
  log_bin=binlog
```

## Configuration

For more configuration, check [miso](https://github.com/CurtisNewbie/miso).

| Property                              | Description                                                                                                                                      | Default Value  |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | -------------- |
| sync.server-id                        | server-id used to mimic a replication server                                                                                                     | 100            |
| sync.user                             | username of the master MySQL instance                                                                                                            | root           |
| sync.password                         | password of the master MySQL instance                                                                                                            |                |
| sync.host                             | host of the master MySQL instance                                                                                                                | 127.0.0.1      |
| sync.port                             | port of the master MySQL instance                                                                                                                | 3306           |
| sync.pos.file                         | binlog position file **(be careful if you are upgrading event-pump)**                                                                            | binlog_pos     |
| sync.max-reconnect                    | max reconnect attempts (reconnect every second, 0 means infinite retry)                                                                          | 120            |
| filter.include                        | regexp for filtering schema names, if specified, only thoes thare are matched are included                                                       |                |
| filter.exclude                        | regexp for filtering schema names, if specified, thoes that thare are matched are excluded, `exclude` filter is executed before `include` filter |                |
| local.pipelines.file                  | locally cached pipeline configurations                                                                                                           | pipelines.json |
| []pipeline.schema                     | regexp for matching schema name                                                                                                                  |                |
| []pipeline.table                      | regexp for matching table name                                                                                                                   |                |
| []pipeline.type                       | regexp for matching event type (optional)                                                                                                        |                |
| []pipeline.stream                     | event bus name (basically, the event is sent to a rabbitmq exchange identified by name `${pipeline.stream}` using routing key `'#'`)             |                |
| []pipeline.enabled                    | whether it's enabled                                                                                                                             |                |
| []pipeline.condition.[]column-changed | Filter events that contain changes to the specified columns                                                                                      |                |
| ha.enabled                            | Enable HA Mode                                                                                                                                   | false          |
| ha.zookeeper.[]host                   | ZooKeeper Hosts                                                                                                                                  |                |

### Configuration Example

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

## Update

- Since v0.0.5, (**standalone**) event-pump no longer depends on redis, binlog position is now recorded in a local file, using following format (previously, it's recorded on redis):

  ```
  {"Name":"binlog.000001","Pos":53318}
  ```

  If you are upgrading event-pump to >= v0.0.5, you should prepare this position file manually. You can retrieve previous binlog position using redis-cli:

  ```sh
  get "event-pump:pos:last"
  # "{\"Name\":\"binlog.000001\",\"Pos\":53318}"
  ```

  Then write the content to the position file.

- Since v0.0.10, event-pump introduces HA mode. In HA Mode, multiple event-pump instances undertake leader election using ZooKeeper; only the leader node is responsible for binlog fetching and parsing, and the remaining nodes are backup. Since there are more than one node running, the binlog position is stored in ZooKeeper as well (see `High-Availability Mode` section).

## Maintenance

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

## High-Availability Mode

event-pump also supports HA using ZooKeeper. Enable HA mode as follows:

```yaml
ha:
  enabled: true
  zookeeper:
    host:
      - "127.0.0.1"
```

All event-pump instances attempt to create Ephemeral Node `/eventpump/leader` on startup. Only one instance can succeed, and the one that created the node is elected to be leader. Other nodes will standby and subscribe to changes to the node. If leader node is somehow down, standby instances will be noticed and attempt the leader election again.

If the HA mode is enabled, binlog position is nolonger stored in a local file. Instead, the binlog position is set to Persistent Node `/eventpump/pos` using the same json format. When leader node bootstraps, and it notices that the node `/eventpump/pos` doesn't exist, it will attempt to read local binlog pos file, and save the value to ZooKeeper.

E.g., Using `zkCli`:

```sh
[zk: localhost:2181(CONNECTED) 3] get /eventpump/leader
# 10.200.1.38

[zk: localhost:2181(CONNECTED) 22] get /eventpump/pos
# {"Name":"mysql-bin.000004","Pos":2842305}
```

## More Documentation

- [API Endpoints](./doc/api.md)

## Creating / Removing Pipelines Through API

event-pump now provides API endpoints to create or remove pipelines in non-HA mode. The pipelines created are by default saved locally in file named `pipelines.json` (see configuration for `'local.pipelines.file'`).

It's recommended to manage the pipelines through configuration file, but the API should give you enough space to manipulate the pipline configuration without restarting the server.

e.g.,

```json
[
  {
    "schema": "mini_fstore",
    "table": "file",
    "stream": "mystream",
    "type": "",
    "condition": { "columnChanged": null }
  }
]
```
