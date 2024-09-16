package binlog

import (
	"github.com/curtisnewbie/event-pump/client"
	"github.com/curtisnewbie/miso/middleware/rabbit"
	"github.com/curtisnewbie/miso/miso"
)

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
