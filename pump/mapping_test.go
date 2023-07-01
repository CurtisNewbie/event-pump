package pump

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMarshalStreamEventStruct(t *testing.T) {
	evt := StreamEvent{
		Timestamp: uint32(time.Now().UnixMilli() / 1000),
		Schema:    "my_db",
		Table:     "my_table",
		Type:      "INS",
		Columns: map[string]StreamEventColumn{
			"id": {
				DataType: "int",
				Before:   "1",
				After:    "1",
			},
			"name": {
				DataType: "varchar",
				Before:   "banana",
				After:    "apple",
			},
		},
	}
	b, err := json.MarshalIndent(&evt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\n%v", string(b))
}
