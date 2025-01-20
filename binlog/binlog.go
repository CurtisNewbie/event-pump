package binlog

import (
	"github.com/curtisnewbie/event-pump/client"
	"github.com/curtisnewbie/miso/middleware/mysql"
	"github.com/curtisnewbie/miso/middleware/rabbit"
	"github.com/curtisnewbie/miso/miso"
	"github.com/curtisnewbie/miso/util"
)

type SubscribeBinlogOption struct {
	// binlog event pipeline
	Pipeline client.Pipeline

	// concurrency
	Concurrency int

	// event listener
	Listener func(rail miso.Rail, t client.StreamEvent) error

	// continue bootstrap even if pipeline creation was failed,
	// it's only necessary to create pipeline once.
	ContinueOnErr bool
}

type SubscribeBinlogOptionV3 struct {

	// merged binlog event pipeline
	MergedPipeline client.MergedPipeline

	// concurrency
	Concurrency int

	// event listener
	Listener func(rail miso.Rail, t client.StreamEvent) error

	// continue bootstrap even if pipeline creation was failed,
	// it's only necessary to create pipeline once.
	ContinueOnErr bool
}

// Subscribe binlog events on server bootstrap.
//
// This is only useful for applications written using miso.
//
// Make sure to run this method before miso.PostServerBootstrapped.
func SubscribeBinlogEventsOnBootstrap(p client.Pipeline, concurrency int,
	listener func(rail miso.Rail, t client.StreamEvent) error) {

	// create pipeline immediately such that the rabbitmq client can
	// recognize and register the queue/exchange/binding declration.
	rabbit.NewEventPipeline[client.StreamEvent](p.Stream).
		Listen(concurrency, listener)

	miso.PostServerBootstrapped(func(rail miso.Rail) error {
		return client.CreatePipeline(rail, p)
	})
}

// Subscribe binlog events on server bootstrap.
//
// This is only useful for applications written using miso.
//
// Make sure to run this method before miso.PostServerBootstrapped.
func SubscribeBinlogEventsOnBootstrapV2(opt SubscribeBinlogOption) {

	if opt.Pipeline.Schema == "" {
		opt.Pipeline.Schema = miso.GetPropStr(mysql.PropMySQLSchema)
	}

	// create pipeline immediately such that the rabbitmq client can
	// recognize and register the queue/exchange/binding declration.
	rabbit.NewEventPipeline[client.StreamEvent](opt.Pipeline.Stream).
		Listen(opt.Concurrency, opt.Listener)

	miso.PostServerBootstrapped(func(rail miso.Rail) error {
		err := client.CreatePipeline(rail, opt.Pipeline)
		if err != nil && opt.ContinueOnErr {
			rail.Errorf("failed to create pipeline, %#v, %v", opt.Pipeline, err)
			return nil
		}
		return err
	})
}

// Subscribe binlog events on server bootstrap.
//
// This is only useful for applications written using miso.
//
// Make sure to run this method before miso.PostServerBootstrapped.
func SubscribeBinlogEventsOnBootstrapV3(opt SubscribeBinlogOptionV3) {

	// create pipeline immediately such that the rabbitmq client can
	// recognize and register the queue/exchange/binding declration.
	rabbit.NewEventPipeline[client.StreamEvent](opt.MergedPipeline.Stream).
		Listen(opt.Concurrency, opt.Listener)

	appSchema := miso.GetPropStr(mysql.PropMySQLSchema)
	pipelines := opt.MergedPipeline.Pipelines
	for i := range pipelines {
		{
			v := pipelines[i]
			if v.Schema == "" {
				v.Schema = appSchema
				pipelines[i] = v
			}
		}
		pipe := opt.MergedPipeline.Pipelines[i]
		miso.PostServerBootstrapped(func(rail miso.Rail) error {
			err := client.CreatePipeline(rail, client.Pipeline{
				Schema:     pipe.Schema,
				Table:      pipe.Table,
				EventTypes: util.SliceCopy(pipe.EventTypes),
				Stream:     opt.MergedPipeline.Stream,
				Condition:  pipe.Condition,
			})
			if err != nil && opt.ContinueOnErr {
				rail.Errorf("failed to create pipeline, %#v, %v", pipe, err)
				return nil
			}
			return err
		})
	}
}
