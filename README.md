# event-pump

Simple app to parse and stream MySQL binlog event in real time. It's Powered by `github.com/go-mysql-org/go-mysql`.

- Tested on MySQL 8.0.23

## Requirements

- MySQL
- Redis
- Consul
- RabbitMQ
- [Goauth](https://github.com/CurtisNewbie/goauth)

## Configuration

| Property           | Description                                                                                                                                         | Default Value |
|--------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| sync.server-id     | server-id used to mimic a replication server                                                                                                        | 100           |
| sync.user          | username of the master MySQL instance                                                                                                               | root          |
| sync.password      | password of the master MySQL instance                                                                                                               |               |
| sync.host          | host of the master MySQL instance                                                                                                                   | 127.0.0.1     |
| sync.port          | port of the master MySQL instance                                                                                                                   | 3306          |
| []pipeline         | list of pipeline config                                                                                                                             |               |
| []pipeline.schema  | regexp for matching schema name                                                                                                                     |               |
| []pipeline.table   | regexp for matching table name                                                                                                                      |               |
| []pipeline.stream  | event bus name (basically, the event is sent to a rabbitmq exchange identified by name `"event.bus." + ${pipeline.stream}` using routing key `'#'`) |               |
| []pipeline.enabled | whether it's enabled                                                                                                                                |               |


## Example

```yaml
pipeline:
  - schema: '.*'
    table: '.*'
    stream: 'data-change.echo'
    enabled: true
```

## Event Struct

The event can be unmarshalled using following structs:

```go
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
```