package client

import (
	"testing"

	"github.com/curtisnewbie/miso/miso"
)

func TestCreatePipeline(t *testing.T) {
	miso.SetProp("client.addr.event-pump.host", "localhost")
	miso.SetProp("client.addr.event-pump.port", 8088)
	err := CreatePipeline(miso.EmptyRail(), Pipeline{
		Schema:     "vfm",
		Table:      "file_info",
		EventTypes: []EventType{EventTypeInsert},
		Stream:     "event.bus.vfm.file.saved",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRemovePipeline(t *testing.T) {
	miso.SetProp("client.addr.event-pump.host", "localhost")
	miso.SetProp("client.addr.event-pump.port", 8088)
	err := RemovePipeline(miso.EmptyRail(), Pipeline{
		Schema:     "vfm",
		Table:      "file_info",
		EventTypes: []EventType{EventTypeInsert},
		Stream:     "event.bus.vfm.file.saved",
	})
	if err != nil {
		t.Fatal(err)
	}
}
