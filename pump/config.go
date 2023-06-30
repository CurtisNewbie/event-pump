package pump

import (
	"github.com/curtisnewbie/gocommon/common"
)

type Pipeline struct {
	Schema  string
	Table   string
	Type    string
	Stream  string
	Enabled bool
}

type GlobalFilter struct {
	Include string
	Exclude string
}

type EventPumpConfig struct {
	Filter    GlobalFilter `mapstructure:"filter"`
	Pipelines []Pipeline   `mapstructure:"pipeline"`
}

func LoadConfig() EventPumpConfig {
	var conf EventPumpConfig
	common.UnmarshalFromProp(&conf)
	return conf
}
