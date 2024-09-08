package client

import (
	"github.com/curtisnewbie/miso/miso"
)

type EventType string

const (
	EventTypeInsert = "INS"
	EventTypeUpdate = "UPD"
	EventTypeDelete = "DEL"
)

type Pipeline struct {
	Schema     string
	Table      string
	EventTypes []EventType
	Stream     string
	Condition  Condition
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
