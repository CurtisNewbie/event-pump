package pump

import (
	"errors"
	"regexp"
	"sync"

	"github.com/curtisnewbie/miso/miso"
	"github.com/go-mysql-org/go-mysql/replication"
)

var (
	defaultLogHandler = func(rail miso.Rail, dce DataChangeEvent) error {
		rail.Infof("Received event: '%v'", dce)
		return nil
	}
	pumpEventWg sync.WaitGroup
)

func PreServerBootstrap(rail miso.Rail) error {

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
		miso.NewEventBus(pipeline.Stream)

		OnEventReceived(func(c miso.Rail, dce DataChangeEvent) error {
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

					if err := miso.PubEventBus(c, evt, pipeline.Stream); err != nil {
						return err
					}
				}
			}
			return nil
		})
		rail.Infof("Subscribed binlog events, schema: '%v', table: '%v', type: '%v', event-bus: %s, conditions: %+v",
			pipeline.Schema, pipeline.Table, pipeline.Type, pipeline.Stream, pipeline.Condition)
	}

	return nil
}

func PostServerBootstrap(rail miso.Rail) error {
	if err := AttachPosFile(rail); err != nil {
		return err
	}

	syncer, err := PrepareSync(rail)
	if err != nil {
		DetachPosFile(rail)
		return err
	}

	streamer, err := NewStreamer(rail, syncer)
	if err != nil {
		DetachPosFile(rail)
		return err
	}

	if !HasAnyEventHandler() {
		OnEventReceived(defaultLogHandler)
	}

	// make sure the goroutine exit before the server stops
	nrail, cancel := rail.NextSpan().WithCancel()
	miso.AddShutdownHook(func() {
		cancel()
		pumpEventWg.Wait()
	})

	pumpEventWg.Add(1)
	go func(rail miso.Rail, streamer *replication.BinlogStreamer) {
		defer func() {
			syncer.Close()
			DetachPosFile(rail)
			pumpEventWg.Done()
		}()

		if e := PumpEvents(rail, syncer, streamer); e != nil {
			rail.Errorf("PumpEvents encountered error: %v, exiting", e)
			miso.Shutdown()
			return
		}
	}(nrail, streamer)
	return nil
}

func BootstrapServer(args []string) {
	miso.PreServerBootstrap(PreServerBootstrap)
	miso.PostServerBootstrapped(PostServerBootstrap)
	miso.BootstrapServer(args)
}
