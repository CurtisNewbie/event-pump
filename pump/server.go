package pump

import (
	"errors"
	"regexp"

	"github.com/curtisnewbie/miso/bus"
	"github.com/curtisnewbie/miso/core"
	"github.com/curtisnewbie/miso/server"
	"github.com/go-mysql-org/go-mysql/replication"
)

var (
	defaultLogHandler = func(rail core.Rail, dce DataChangeEvent) error {
		rail.Infof("Received event: '%v'", dce)
		return nil
	}
)

func PreServerBootstrap(rail core.Rail) error {

	config := LoadConfig()
	rail.Debugf("Config: %+v", config)

	if config.Filter.Include != "" {
		SetGlobalInclude(regexp.MustCompile(config.Filter.Include))
	}

	if config.Filter.Exclude != "" {
		SetGlobalExclude(regexp.MustCompile(config.Filter.Exclude))
	}

	for _, p := range config.Pipelines {
		pipeline := p
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

		// filter rules for complex configuration, e.g., only the events that include changes to certain columns
		filters := NewFilters(pipeline)

		// mapper for converting the structure of the event
		mapper := NewMapper()

		// Declare Stream
		bus.DeclareEventBus(pipeline.Stream)

		OnEventReceived(func(c core.Rail, dce DataChangeEvent) error {
			if !schemaPattern.MatchString(dce.Schema) {
				c.Debugf("schema pattern not matched, event ignored, %v", dce.Schema)
				return nil
			}
			if !tablePattern.MatchString(dce.Table) {
				c.Debugf("table pattern not matched, event ignored, %v", dce.Table)
				return nil
			}
			if typePattern != nil && !typePattern.MatchString(dce.Type) {
				c.Debugf("type pattern not matched, event ignored, %v", dce.Type)
				return nil
			}

			// based on configuration, we may convert the dce to some sort of structure meaningful to the receiver
			// one change event may be manified to multple events, e.g., an update to multiple rows
			events, err := mapper.MapEvent(dce)
			if err != nil {
				return err
			}

			c.Debugf("DCE: %s", dce)

			for _, evt := range events {
				for _, filter := range filters {
					if !filter.Include(c, evt) {
						continue
					}
				}

				if err := bus.SendToEventBus(c, evt, pipeline.Stream); err != nil {
					return err
				}

			}
			return nil
		})
		rail.Infof("Subscribed binlog events, schema: '%v', table: '%v', type: '%v', event-bus: %s, conditions: %+v",
			pipeline.Schema, pipeline.Table, pipeline.Type, pipeline.Stream, pipeline.Condition)
	}

	return nil
}

func PostServerBootstrap(rail core.Rail) error {
	syncer, err := PrepareSync(rail)
	if err != nil {
		return err
	}

	streamer, err := NewStreamer(rail, syncer)
	if err != nil {
		return err
	}

	if !HasAnyEventHandler() {
		OnEventReceived(defaultLogHandler)
	}

	go func(rail core.Rail, streamer *replication.BinlogStreamer) {
		defer syncer.Close()
		if e := PumpEvents(rail, syncer, streamer); e != nil {
			rail.Errorf("PumpEvents encountered error: %v, exiting", e)
			server.Shutdown()
		}
	}(rail.NextSpan(), streamer)
	return nil
}
