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

	// For testing
	// bus.SubscribeEventBus("data-change.echo", 1, func(dce StreamEvent) error {
	// 	c.Log.Infof("Receieved: %+v", dce)
	// 	return nil
	// })

	config := LoadConfig()
	c.Log.Debugf("Config: %+v", config)

	if config.Filter.Include != "" {
		SetGlobalInclude(regexp.MustCompile(config.Filter.Include))
	}

	if config.Filter.Exclude != "" {
		SetGlobalExclude(regexp.MustCompile(config.Filter.Exclude))
	}

	for _, pipeline := range config.Pipelines {
		if !pipeline.Enabled {
			continue
		}

		if pipeline.Stream == "" {
			return errors.New("pipeline.stream is emtpy")
		}

		schemaPattern := regexp.MustCompile(pipeline.Schema)
		tablePattern := regexp.MustCompile(pipeline.Table)
		var typePattern *regexp.Regexp
		if pipeline.Type != "" {
			typePattern = regexp.MustCompile(pipeline.Type)
		}

		// mapper for converting the event
		mapper := NewMapper(pipeline.Type)

		// Declare Stream
		bus.DeclareEventBus(pipeline.Stream)

		OnEventReceived(func(c common.ExecContext, dce DataChangeEvent) error {
			if !schemaPattern.MatchString(dce.Schema) {
				c.Log.Debugf("schema pattern not matched, event ignored, %v", dce.Schema)
				return nil
			}
			if !tablePattern.MatchString(dce.Table) {
				c.Log.Debugf("table pattern not matched, event ignored, %v", dce.Table)
				return nil
			}
			if typePattern != nil && !typePattern.MatchString(dce.Type) {
				c.Log.Debugf("type pattern not matched, event ignored, %v", dce.Type)
				return nil
			}

			// based on configuration, we may convert the dce to some sort of structure meaningful to the receiver
			events, err := mapper.MapEvent(dce)
			if err != nil {
				return err
			}

			c.Log.Debugf("DCE: %s", dce)

			// dispatch event, one change event may be manified to multple events
			// e.g., an update to multiple rows
			for _, evt := range events {
				if err := bus.SendToEventBus(evt, pipeline.Stream); err != nil {
					return err
				}
			}
			return nil
		})
		c.Log.Infof("Subscribed DataChangeEvent with schema pattern: '%v', table pattern: '%v', type pattern: '%v'",
			pipeline.Schema, pipeline.Table, pipeline.Type)
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
