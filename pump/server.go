package pump

import (
	"github.com/curtisnewbie/gocommon/common"
	"github.com/go-mysql-org/go-mysql/replication"
)

func PreServerBootstrap(c common.ExecContext) error {
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

	OnEventReceived(func(c common.ExecContext, dce DataChangeEvent) error {
		// TODO: Add some streaming stuff, e.g., configuration
		c.Log.Infof(">>>>>> Event: %+v", dce)
		return nil
	})

	go func(streamer *replication.BinlogStreamer) {
		defer syncer.Close()
		if e := PumpEvents(c, syncer, streamer); e != nil {
			c.Log.Fatal(e)
		}
	}(streamer)
	return nil
}
