package pump

import "github.com/curtisnewbie/miso/core"

type Filter interface {
	Include(rail core.Rail, evt any) bool
}

type noOpFilter struct {
}

func (f noOpFilter) Include(rail core.Rail, evt any) bool {
	return true
}

type columnFilter struct {
	ColumnsChanged []string
}

func (f columnFilter) Include(rail core.Rail, evt any) bool {
	switch ev := evt.(type) {
	case StreamEvent:
		if ev.Type != TYPE_UPDATE {
			return true
		}

		for _, cc := range f.ColumnsChanged {
			sec, ok := ev.Columns[cc]
			if ok && sec.Before != sec.After {
				return true
			}
		}

		rail.Debugf("Event filtered out, doesn't contain change to any of the specified columns: %v", f.ColumnsChanged)
		return false // the event doesn't include any change to these specified columns

	case DataChangeEvent:
		return true // doesn't support at all
	}

	return true
}

func NewFilters(p Pipeline) []Filter {
	if len(p.Condition.ColumnChanged) < 1 {
		return []Filter{noOpFilter{}}
	}

	return []Filter{columnFilter{core.Distinct(p.Condition.ColumnChanged)}}
}
