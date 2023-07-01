package pump

import "fmt"

const (
	STRUCT_RAW    = "raw"
	STRUCT_STREAM = "stream"
)

type StreamEvent struct {
	Timestamp uint32                       `json:"timestamp"`
	Schema    string                       `json:"schema"`
	Table     string                       `json:"table"`
	Type      string                       `json:"type"`    // INS-INSERT, UPD-UPDATE, DEL-DELETE
	Columns   map[string]StreamEventColumn `json:"columns"` // key is the column name
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

type rawEventMapper struct {
}

func (m rawEventMapper) MapEvent(dce DataChangeEvent) ([]any, error) {
	return []any{dce}, nil
}

func NewMapper(structure string) Mapper {
	switch structure {
	case STRUCT_RAW:
		return rawEventMapper{}
	case STRUCT_STREAM:
		return streamEventMapper{}
	default:
		return streamEventMapper{}
	}
}
