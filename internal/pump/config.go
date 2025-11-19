package pump

import "github.com/curtisnewbie/miso/miso"

type Condition struct {
	// name of columns that change in an binlog event.
	ColumnChanged []string `mapstructure:"column-changed" json:"columnChanged"`
}

type Pipeline struct {
	// event handler id.
	HandlerId string `json:"-"`

	// schema name.
	Schema string `json:"schema"`

	// table name.
	Table string `json:"table"`

	// event bus name.
	Stream string `json:"stream"`

	// event type regexp.
	Type string `json:"type"`

	// event types: INS, UPD, DEL.
	Types []string `json:"-"`

	// Whether pipeline is enabled.
	//
	// pipeline created using API is always enabled.
	Enabled bool `json:"-"`

	// extra filtering conditions
	Condition Condition `mapstructure:"condition" json:"condition"`
}

type GlobalFilter struct {
	Include string
	Exclude string
}

type EventMapping struct {
	From string
	To   string
	Type string
}

type EventPumpConfig struct {
	Filter    GlobalFilter `mapstructure:"filter"`
	Pipelines []Pipeline   `mapstructure:"pipeline"`
}

func LoadConfig() EventPumpConfig {
	var conf EventPumpConfig
	miso.UnmarshalFromProp(&conf)
	return conf
}
