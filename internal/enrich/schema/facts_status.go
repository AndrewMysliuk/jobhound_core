package schema

import "fmt"

// FactsStatus is the page-facing enrich state for one job.
type FactsStatus string

const (
	FactsStatusOff     FactsStatus = "off"
	FactsStatusPending FactsStatus = "pending"
	FactsStatusReady   FactsStatus = "ready"
)

func (s FactsStatus) String() string { return string(s) }

func (s FactsStatus) Equals(str string) bool { return string(s) == str }

func (s FactsStatus) Pointer() *FactsStatus { return &s }

func (s FactsStatus) FromValue(str string) (FactsStatus, error) {
	switch FactsStatus(str) {
	case FactsStatusOff, FactsStatusPending, FactsStatusReady:
		return FactsStatus(str), nil
	default:
		return "", fmt.Errorf("unknown FactsStatus %q: valid values are %v", str, ValuesFactsStatus())
	}
}

func ValuesFactsStatus() []FactsStatus {
	return []FactsStatus{FactsStatusOff, FactsStatusPending, FactsStatusReady}
}

func FromStringFactsStatus(str string) (FactsStatus, error) {
	var z FactsStatus
	return z.FromValue(str)
}
