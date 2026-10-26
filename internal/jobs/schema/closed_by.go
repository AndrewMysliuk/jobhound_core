package schema

import "fmt"

// ClosedBy records which path closed a vacancy.
type ClosedBy string

const (
	ClosedByATS        ClosedBy = "ATS"
	ClosedByAggregator ClosedBy = "AGGREGATOR"
)

func (c ClosedBy) String() string { return string(c) }

func (c ClosedBy) Equals(str string) bool { return string(c) == str }

func (c ClosedBy) Pointer() *ClosedBy { return &c }

func (c ClosedBy) FromValue(str string) (ClosedBy, error) {
	switch ClosedBy(str) {
	case ClosedByATS, ClosedByAggregator:
		return ClosedBy(str), nil
	default:
		return "", fmt.Errorf("unknown ClosedBy %q: valid values are %v", str, ValuesClosedBy())
	}
}

func ValuesClosedBy() []ClosedBy {
	return []ClosedBy{ClosedByATS, ClosedByAggregator}
}

func FromStringClosedBy(str string) (ClosedBy, error) {
	var z ClosedBy
	return z.FromValue(str)
}
