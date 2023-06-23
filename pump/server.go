package pump

import (
	"errors"
	"regexp"

	"github.com/curtisnewbie/gocommon/bus"
	"github.com/curtisnewbie/gocommon/common"
	"github.com/go-mysql-org/go-mysql/replication"
)

var (
	defaultLogHandler = func(c common.ExecContext, dce DataChangeEvent) error {
		c.Log.Infof("Received event: '%v'", dce)
		return nil
	}
)

func PreServerBootstrap(c common.ExecContext) error {

	/*
		// For testing
		bus.SubscribeEventBus("data-change.echo", 1, func(dce DataChangeEvent) error {
			c.Log.Infof("Receieved: %+v", dce)
			return nil
		})
	*/

	pipelineConfig := LoadPipelines()
	c.Log.Infof("pipeline config: %+v", pipelineConfig)

	for _, pipeline := range pipelineConfig.Pipelines {
		if !pipeline.Enabled {
			continue
		}

		if pipeline.Stream == "" {
			return errors.New("pipeline.stream is emtpy")
		}

		schemaPattern := regexp.MustCompile(pipeline.Schema)
		tablePattern := regexp.MustCompile(pipeline.Table)

		// Declare Stream
		bus.DeclareEventBus(pipeline.Stream)

		OnEventReceived(func(c common.ExecContext, dce DataChangeEvent) error {
			if !schemaPattern.MatchString(dce.Schema) {
				return nil
			}
			if !tablePattern.MatchString(dce.Table) {
				return nil
			}
			return bus.SendToEventBus(dce, pipeline.Stream)
		})
		c.Log.Infof("Subscribed DataChangeEvent with schema pattern: '%v', table pattern: '%v'", pipeline.Schema, pipeline.Table)
	}

	return nil
}

func PostServerBootstrap(c common.ExecContext) error {
	syncer, err := PrepareSync(c)
	if err != nil {
		return err
	}

	streamer, err := NewStreamer(c, syncer)
	if err != nil {
		return err
	}

	if !HasAnyEventHandler() {
		OnEventReceived(defaultLogHandler)
	}

	go func(ec common.ExecContext, streamer *replication.BinlogStreamer) {
		defer syncer.Close()
		if e := PumpEvents(ec, syncer, streamer); e != nil {
			ec.Log.Fatal(e)
		}
	}(c.NextSpan(), streamer)
	return nil
}
