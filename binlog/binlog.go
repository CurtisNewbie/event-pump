package binlog

import (
	"github.com/curtisnewbie/event-pump/client"
	"github.com/curtisnewbie/miso/middleware/rabbit"
	"github.com/curtisnewbie/miso/miso"
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
