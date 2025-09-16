package pump

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/curtisnewbie/miso/encoding/json"
	"github.com/curtisnewbie/miso/middleware/rabbit"
	"github.com/curtisnewbie/miso/middleware/user-vault/auth"
	"github.com/curtisnewbie/miso/miso"
	"github.com/curtisnewbie/miso/util"
	"github.com/curtisnewbie/miso/util/slutil"
	"github.com/go-mysql-org/go-mysql/replication"
)

const (
	PropHAEnabled          = "ha.enabled"
	PropPipelineConfigFile = "local.pipelines.file"

	DashboardResourceCode = "event-pump-dashboard"
)

var (
	defaultLogHandler = func(rail miso.Rail, dce DataChangeEvent, ctx *EventHandleContext) error {
		rail.Infof("Received event: '%v'", dce)
		return nil
	}
	pumpEventWg sync.WaitGroup
	syncRunOnce sync.Once
)

var (
	pipelineMap = map[string][]Pipeline{}
	pipMu       sync.RWMutex
)

var (
	// save pipline config to local file every 30s
	pipelineConfigSyncTick = miso.NewTickRuner(30*time.Second, saveLocalConfigs)
	saveLocalConfigMutex   sync.Mutex
)

func init() {
	miso.SetDefProp(PropPipelineConfigFile, "pipelines.json")
}

func PreServerBootstrap(rail miso.Rail) error {

	// for moon-monorepo project, if event-pump was deployed as a standalone project, then this has no effect
	prepareResourceForMoon()

	// prepare health indicator
	prepareHealthIndicator()

	config := LoadConfig()
	rail.Debugf("Config: %+v", config)

	for i, p := range config.Pipelines {
		if len(p.Types) > 0 {
			p.Type = pipelineTypeRegex(p.Types)
		}
		config.Pipelines[i] = p
	}

	if config.Filter.Include != "" {
		SetGlobalInclude(regexp.MustCompile(config.Filter.Include))
	}

	if config.Filter.Exclude != "" {
		SetGlobalExclude(regexp.MustCompile(config.Filter.Exclude))
	}

	config.Pipelines = append(config.Pipelines, loadLocalConfigs(rail)...)

	for _, p := range config.Pipelines {
		if err := AddPipeline(rail, p); err != nil {
			return err
		}
	}

	if err := PrepareStatic(rail); err != nil {
		return err
	}

	return nil
}

func isHaMode() bool {
	return miso.GetPropBool(PropHAEnabled)
}

func loadLocalConfigs(rail miso.Rail) []Pipeline {
	fn := miso.GetPropStr(PropPipelineConfigFile)
	if fn == "" {
		return []Pipeline{}
	}
	pl := []Pipeline{}
	c, err := util.ReadFileAll(fn)
	if err != nil {
		rail.Infof("Read local config file failed, %v", err)
		return pl
	}

	err = json.ParseJson(c, &pl)
	if err != nil {
		rail.Infof("Parse local Pipeline failed, %v", err)
		return pl
	}

	typeRegex := regexp.MustCompile(`^\^\(([^\)]*)\)\$$`)
	for i, p := range pl {
		p.Enabled = true
		if p.Type != "" {
			sub := typeRegex.FindStringSubmatch(p.Type)
			rail.Debugf("sub: %v", sub)
			if len(sub) > 1 {
				p.Types = strings.Split(sub[1], "|")
				p.Type = pipelineTypeRegex(p.Types)
			}
		}
		pl[i] = p
	}
	rail.Infof("Loaded local Pipeline configs, count: %v", len(pl))
	return pl
}

func saveLocalConfigs() {
	saveLocalConfigMutex.Lock()
	defer saveLocalConfigMutex.Unlock()

	fn := miso.GetPropStr(PropPipelineConfigFile)
	if fn == "" {
		return
	}

	// temp file, the configs are first written and flushed to temp file
	// then the temp file is renamed to target file
	wbuf := fn + "_buffer"

	var pl []Pipeline = copyPipelines()
	rail := miso.EmptyRail()

	f, err := util.ReadWriteFile(wbuf)
	if err != nil {
		rail.Errorf("Failed to save local config file, '%v', %v", wbuf, err)
		return
	}
	defer f.Close()

	_ = f.Truncate(0)

	s, err := json.SWriteJson(pl)
	if err != nil {
		rail.Errorf("Failed to save local config file, '%v', %v", wbuf, err)
		return
	}

	_, err = f.WriteString(s)
	if err != nil {
		rail.Errorf("Failed to save local config file, '%v', %v", wbuf, err)
		return
	}
	if err := f.Sync(); err != nil {
		rail.Errorf("Failed to sync to local config file, '%v', %v", wbuf, err)
		return
	}

	if err := os.Rename(wbuf, fn); err != nil {
		rail.Errorf("Failed to overwrite local config file, '%v', %v", fn, err)
		return
	}

	rail.Debugf("Persistent pipelines saved to local file: %v", fn)
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

type ApiPipeline struct {
	Schema     string    `desc:"schema name"`
	Table      string    `desc:"table name"`
	EventTypes []string  `desc:"event types; INS - Insert, UPD - Update, DEL - Delete"`
	Stream     string    `desc:"event bus name"`
	Condition  Condition `desc:"extra filtering conditions"`
}

func (p ApiPipeline) Pipeline() Pipeline {
	pl := Pipeline{}
	pl.Schema = p.Schema
	pl.Table = p.Table
	pl.Type = pipelineTypeRegex(p.EventTypes)
	pl.Stream = p.Stream
	pl.Condition = p.Condition
	pl.Enabled = true
	return pl
}

func pipelineTypeRegex(typs []string) string {
	if len(typs) < 1 {
		return ""
	}
	typs = slutil.Distinct(typs)
	sort.Strings(typs)
	return "^(" + strings.Join(typs, "|") + ")$"
}

func copyPipelines() []Pipeline {
	pipMu.RLock()
	defer pipMu.RUnlock()

	cp := make([]Pipeline, 0, len(pipelineMap))
	for _, v := range pipelineMap {
		cp = append(cp, v...)
	}
	return cp
}

func copyApiPipelines() []ApiPipeline {
	pipMu.RLock()
	defer pipMu.RUnlock()

	cp := make([]ApiPipeline, 0, len(pipelineMap))
	for _, v := range pipelineMap {
		cvt := slutil.MapTo(v, func(p Pipeline) ApiPipeline {
			return ApiPipeline{
				Schema:     p.Schema,
				Table:      p.Table,
				EventTypes: p.Types,
				Stream:     p.Stream,
				Condition:  p.Condition,
			}
		})
		cp = append(cp, cvt...)
	}
	sort.Slice(cp, func(i, j int) bool {
		pi := cp[i]
		pj := cp[j]
		if pi.Schema != pj.Schema {
			return pi.Schema < pj.Schema
		}
		if pi.Table != pj.Table {
			return pi.Table < pj.Table
		}
		if pi.Schema != pj.Schema {
			return pi.Schema < pj.Schema
		}
		return false
	})
	return cp
}

func RemovePipeline(rail miso.Rail, pipeline Pipeline) {
	pipMu.Lock()
	defer pipMu.Unlock()

	pk := pipeline.Schema + "." + pipeline.Table
	if prev, ok := pipelineMap[pk]; ok {
		for i, p := range prev {
			if samePipeline(p, pipeline) {
				pipelineMap[pk] = slutil.SliceRemove(pipelineMap[pk], i)
				RemoveEventHandler(p.HandlerId)
				rail.Infof("Removed pipeline: %#v", p)
				return
			}
		}
	}
	rail.Infof("Pipeline not found, nothing to remove: %#v", pipeline)
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

	pipMu.Lock()
	defer pipMu.Unlock()

	pk := pipeline.Schema + "." + pipeline.Table
	if prev, ok := pipelineMap[pk]; ok {
		for _, p := range prev {
			if samePipeline(p, pipeline) {
				rail.Debugf("Duplicate pipeline found, skipped, %#v", pipeline)
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

	handlerId := OnEventReceived(func(c miso.Rail, dce DataChangeEvent, ctx *EventHandleContext) error {
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
		isProd := miso.IsProdMode()

		if ctx.StreamDispatched.Has(pipeline.Stream) {
			// already dispatched to streaming pipeline, we should just ignore it
			// if there are multiple pipelines with diffrent event_type/column filtering conditions pointing toward the same stream,
			// then they are simply duplicates
			if !isProd {
				c.Infof("Event Pipeline Stream has already been triggered '%s', skipped", pipeline.Stream)
			}
			return nil
		}

		for _, evt := range events {
			anyMatch := false
			for _, filter := range filters {
				if filter.Include(c, evt) {
					anyMatch = true
					break
				}
			}

			if anyMatch {
				ctx.StreamDispatched.Add(pipeline.Stream)
				if err := rabbit.PubEventBus(c, evt, pipeline.Stream); err != nil {
					return err
				}
				if !isProd {
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

	haMode := isHaMode()
	SetupPosFileStorage(haMode)

	if !haMode {
		pipelineConfigSyncTick.Start()
	}

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
			if !haMode {
				saveLocalConfigs()
			}
			cancel()
			pumpEventWg.Wait()
		})

		pumpEventWg.Add(1)
		go func(rail miso.Rail, streamer *replication.BinlogStreamer) {
			defer func() {
				syncer.Close()
				DetachPos(rail)
				pumpEventWg.Done()
				miso.Shutdown()
			}()

			if e := PumpEvents(rail, syncer, streamer); e != nil {
				rail.Errorf("PumpEvents encountered error: %v, exiting", e)
				return
			}
		}(nrail, streamer)
	}

	if haMode {
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
	miso.PostServerBootstrap(PostServerBootstrap)
	miso.BootstrapServer(args)
}

func prepareResourceForMoon() {
	auth.ExposeResourceInfo(
		[]auth.Resource{
			{Name: "event-pump dashboard", Code: DashboardResourceCode},
		},
		auth.Endpoint{
			Type:    auth.ScopeProtected,
			Url:     "/event-pump/**",
			Group:   "event-pump",
			Desc:    "event-pump dashboard and api",
			ResCode: DashboardResourceCode,
			Method:  "*",
		})
}

func prepareHealthIndicator() {
	miso.AddHealthIndicator(miso.HealthIndicator{
		Name: "Binlog Health Indicator",
		CheckHealth: func(rail miso.Rail) bool {
			healthy := checkBinlogHealth()
			rail.Debugf("Binlog polling is healthy: %v", healthy)
			return healthy
		},
	})
}
