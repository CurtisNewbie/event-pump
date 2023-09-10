package pump

import "github.com/curtisnewbie/miso/miso"

type Condition struct {
	ColumnChanged []string `mapstructure:"column-changed"`
}

type Pipeline struct {
	Schema    string
	Table     string
	Type      string
	Stream    string
	Enabled   bool
	Condition Condition `mapstructure:"condition"`
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
