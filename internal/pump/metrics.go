package pump

import "github.com/curtisnewbie/miso/miso"

var (
	binlogEventHisto = miso.NewPromHisto("event_pump_binlog_event")
)

func NewBinlogEventTimer() *miso.HistTimer {
	return miso.NewHistTimer(binlogEventHisto)
}
