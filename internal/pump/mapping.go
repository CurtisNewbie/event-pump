package pump

import "fmt"

type StreamEvent struct {
	Timestamp uint32                       `json:"timestamp"` // Epoch time second
	Schema    string                       `json:"schema"`    // Schema name
	Table     string                       `json:"table"`     // Table name
	Type      string                       `json:"type"`      // Event Type: INS-INSERT, UPD-UPDATE, DEL-DELETE
	Columns   map[string]StreamEventColumn `json:"columns"`   // Map of column name and value changes
}

type StreamEventColumn struct {
	DataType string `json:"dataType"`
	Before   string `json:"before"`
	After    string `json:"after"`
}

type Mapper interface {
	MapEvent(DataChangeEvent) ([]any, error)
}

type streamEventMapper struct {
}

func (m streamEventMapper) MapEvent(dce DataChangeEvent) ([]any, error) {
	mapped := []any{}
	for _, rec := range dce.Records {
		columns := map[string]StreamEventColumn{}
		for j, col := range dce.Columns {
			var before string
			var after string

			if j < len(rec.Before) {
				before = fmt.Sprintf("%v", rec.Before[j])
			}
			if j < len(rec.After) {
				after = fmt.Sprintf("%v", rec.After[j])
			}
			columns[col.Name] = StreamEventColumn{
				DataType: col.DataType,
				Before:   before,
				After:    after,
			}
		}

		mapped = append(mapped, StreamEvent{
			Timestamp: dce.Timestamp,
			Schema:    dce.Schema,
			Table:     dce.Table,
			Type:      dce.Type,
			Columns:   columns,
		})
	}

	return mapped, nil
}

func NewMapper() Mapper {
	return streamEventMapper{}
}
