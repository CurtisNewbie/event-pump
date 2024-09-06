package pump

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/curtisnewbie/miso/middleware/rabbit"
	"github.com/curtisnewbie/miso/miso"
	"github.com/curtisnewbie/miso/util"
	"github.com/go-mysql-org/go-mysql/replication"
)

const (
	PropHAEnabled = "ha.enabled"
)

var (
	defaultLogHandler = func(rail miso.Rail, dce DataChangeEvent) error {
		rail.Infof("Received event: '%v'", dce)
		return nil
	}
	pumpEventWg sync.WaitGroup
	syncRunOnce sync.Once
)

var (
	pipelineMap = map[string][]Pipeline{}
	pmu         sync.Mutex
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
		if err := AddPipeline(rail, p); err != nil {
			return err
		}
	}

	return nil
}

func samePipeline(a Pipeline, b Pipeline) bool {
	return a.Schema == b.Schema &&
		a.Table == b.Table &&
		a.Type == b.Type &&
		a.Stream == b.Stream &&
		sameCondition(a.Condition, b.Condition)
}

func sameCondition(a Condition, b Condition) bool {
	if len(a.ColumnChanged) != len(b.ColumnChanged) {
		return false
	}

	slices.Sort(a.ColumnChanged)
	slices.Sort(b.ColumnChanged)
	for i := 0; i < len(a.ColumnChanged); i++ {
		if a.ColumnChanged[i] != b.ColumnChanged[i] {
			return false
		}
	}
	return true
}

// misoapi-http: POST /api/v1/create-pipeline
// misoapi-desc: Create new pipeline. Duplicate pipeline is ignored, HA is not supported.
func ApiCreatePipeline(rail miso.Rail, pipeline Pipeline) error {
	if miso.GetPropBool(PropHAEnabled) {
		return miso.NewErrf("Not supported for HA mode")
	}
	return AddPipeline(rail, pipeline)
}

// misoapi-http: POST /api/v1/remove-pipeline
// misoapi-desc: Remove existing pipeline. HA is not supported.
func ApiRemovePipeline(rail miso.Rail, pipeline Pipeline) error {
	if miso.GetPropBool(PropHAEnabled) {
		return miso.NewErrf("Not supported for HA mode")
	}
	RemovePipeline(rail, pipeline)
	return nil
}

func RemovePipeline(rail miso.Rail, pipeline Pipeline) {
	pmu.Lock()
	defer pmu.Unlock()

	pk := pipeline.Schema + "." + pipeline.Table
	if prev, ok := pipelineMap[pk]; ok {
		for i, p := range prev {
			if samePipeline(p, pipeline) {
				pipelineMap[pk] = util.SliceRemove(pipelineMap[pk], i)
				RemoveEventHandler(p.HandlerId)
				rail.Infof("Removed pipeline: %#v", p)
				return
			}
		}
	}
}

func AddPipeline(rail miso.Rail, pipeline Pipeline) error {
	if !pipeline.Enabled {
		return nil
	}
	if pipeline.Stream == "" {
		return errors.New("pipeline.stream is emtpy")
	}
	pipeline.Schema = strings.TrimSpace(pipeline.Schema)
	pipeline.Table = strings.TrimSpace(pipeline.Table)
	pipeline.Type = strings.TrimSpace(pipeline.Type)
	pipeline.Stream = strings.TrimSpace(pipeline.Stream)
	for i, c := range pipeline.Condition.ColumnChanged {
		pipeline.Condition.ColumnChanged[i] = strings.TrimSpace(c)
	}

	pmu.Lock()
	defer pmu.Unlock()

	pk := pipeline.Schema + "." + pipeline.Table
	if prev, ok := pipelineMap[pk]; ok {
		for _, p := range prev {
			if samePipeline(p, pipeline) {
				rail.Infof("Duplicate pipeline found, skipped, %#v", pipeline)
				return nil
			}
		}
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
	rabbit.NewEventBus(pipeline.Stream)

	handlerId := OnEventReceived(func(c miso.Rail, dce DataChangeEvent) error {
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
			anyMatch := false
			for _, filter := range filters {
				if filter.Include(c, evt) {
					anyMatch = true
					break
				}
			}
			if anyMatch {
				if err := rabbit.PubEventBus(c, evt, pipeline.Stream); err != nil {
					return err
				}
				if !miso.IsProdMode() {
					c.Infof("Event Pipeline triggered, schema: '%v', table: '%v', type: '%v', event-bus: %s, conditions: %+v",
						pipeline.Schema, pipeline.Table, pipeline.Type, pipeline.Stream, pipeline.Condition)
				}
			}
		}
		return nil
	})

	pipeline.HandlerId = handlerId
	pipelineMap[pk] = append(pipelineMap[pk], pipeline)

	rail.Infof("Subscribed binlog events, schema: '%v', table: '%v', type: '%v', event-bus: %s, conditions: %+v",
		pipeline.Schema, pipeline.Table, pipeline.Type, pipeline.Stream, pipeline.Condition)
	return nil
}

func PostServerBootstrap(rail miso.Rail) error {

	haEnabled := miso.GetPropBool(PropHAEnabled)
	SetupPosFileStorage(haEnabled)

	startSync := func() {
		if err := AttachPos(rail); err != nil {
			panic(fmt.Errorf("failed to attach pos file, %v", err))
		}

		syncer, err := PrepareSync(rail)
		if err != nil {
			DetachPos(rail)
			panic(fmt.Errorf("failed to create syncer, %v", err))
		}

		streamer, err := NewStreamer(rail, syncer)
		if err != nil {
			DetachPos(rail)
			panic(fmt.Errorf("failed to create streamer, %v", err))
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
				DetachPos(rail)
				pumpEventWg.Done()
			}()

			if e := PumpEvents(rail, syncer, streamer); e != nil {
				rail.Errorf("PumpEvents encountered error: %v, exiting", e)
				miso.Shutdown()
				return
			}
		}(nrail, streamer)
	}

	if haEnabled {
		return ZkElectLeader(rail, func() {
			syncRunOnce.Do(startSync)
		})
	} else {
		startSync()
	}

	return nil
}

func BootstrapServer(args []string) {
	miso.PreServerBootstrap(PreServerBootstrap)
	miso.PostServerBootstrapped(PostServerBootstrap)
	miso.BootstrapServer(args)
}
