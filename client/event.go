package client

type StreamEvent struct {
	Timestamp uint32                 `json:"timestamp"` // epoch time second
	Schema    string                 `json:"schema"`
	Table     string                 `json:"table"`
	Type      string                 `json:"type"`    // INS-INSERT, UPD-UPDATE, DEL-DELETE
	Columns   map[string]EventColumn `json:"columns"` // key is the column name
}

type EventColumn struct {
	DataType string `json:"dataType"`
	Before   string `json:"before"`
	After    string `json:"after"`
}

// Get Column's After value.
func (b *StreamEvent) ColumnAfter(name string) (string, bool) {
	v, ok := b.Columns[name]
	if !ok {
		return "", false
	}
	return v.After, true
}
