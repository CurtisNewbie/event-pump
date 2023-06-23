package pump

import (
	"github.com/curtisnewbie/gocommon/common"
)

type Pipeline struct {
	Schema  string
	Table   string
	Stream  string
	Enabled bool
}

type PipelineConfig struct {
	Pipelines []Pipeline `mapstructure:"pipeline"`
}

func LoadPipelines() PipelineConfig {
	var conf PipelineConfig
	common.UnmarshalFromProp(&conf)
	return conf
}
