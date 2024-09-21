package client

import (
	"github.com/curtisnewbie/miso/miso"
)

type EventType string

const (
	// row inserted
	EventTypeInsert = "INS"

	// row updated
	EventTypeUpdate = "UPD"

	// row deleted
	EventTypeDelete = "DEL"
)

type Pipeline struct {
	Schema     string      // schema name
	Table      string      // table name
	EventTypes []EventType // event types subscribed
	Stream     string      // miso event bus name
	Condition  Condition   // extra binlog filtering condition
}

type Condition struct {
	ColumnChanged []string // column names that are changed
}

func CreatePipeline(rail miso.Rail, req Pipeline) error {
	var res miso.GnResp[any]
	err := miso.NewDynTClient(rail, "/api/v1/create-pipeline", "event-pump").
		PostJson(req).
		Json(&res)
	if err != nil {
		rail.Errorf("Request failed, %v", err)
		return err
	}
	err = res.Err()
	if err != nil {
		rail.Errorf("Request failed, %v", err)
	}
	return err
}

func RemovePipeline(rail miso.Rail, req Pipeline) error {
	var res miso.GnResp[any]
	err := miso.NewDynTClient(rail, "/api/v1/remove-pipeline", "event-pump").
		PostJson(req).
		Json(&res)
	if err != nil {
		rail.Errorf("Request failed, %v", err)
		return err
	}
	err = res.Err()
	if err != nil {
		rail.Errorf("Request failed, %v", err)
	}
	return err
}
