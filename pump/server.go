package pump

import (
	"github.com/curtisnewbie/gocommon/common"
)

func PreServerBootstrap(c common.ExecContext) error {
	return nil
}

func PostServerBootstrap(c common.ExecContext) error {
	syncer, err := NewSyncer(c)
	if err != nil {
		return err
	}

	streamer, err := NewStreamer(c, syncer)
	if err != nil {
		return err
	}

	OnEventReceived(func(c common.ExecContext, dce DataChangeEvent) error {
		// TODO: Add some streaming stuff, e.g., configuration
		c.Log.Infof("Event: %+v", dce)
		return nil
	})

	return PumpEvents(c, streamer)
}
