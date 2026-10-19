package schema

import "fmt"

// SignalCode identifies one scoring hit or one hard cutoff.
type SignalCode string

const (
	SignalGateRole    SignalCode = "GATE_ROLE"
	SignalGateHard    SignalCode = "GATE_HARD"
	SignalCutLocation SignalCode = "CUT_LOCATION"
	SignalCutTimezone SignalCode = "CUT_TIMEZONE"
	SignalCutPostedAt SignalCode = "CUT_POSTED_AT"
	SignalPlus        SignalCode = "PLUS"
	SignalPenalty     SignalCode = "PENALTY"
	SignalLLM         SignalCode = "LLM"
	SignalConflict    SignalCode = "CONFLICT"
)

func (c SignalCode) String() string { return string(c) }

func (c SignalCode) Equals(s string) bool { return string(c) == s }

func (c SignalCode) Pointer() *SignalCode { return &c }

func (c SignalCode) FromValue(s string) (SignalCode, error) {
	switch SignalCode(s) {
	case SignalGateRole, SignalGateHard, SignalCutLocation, SignalCutTimezone, SignalCutPostedAt, SignalPlus, SignalPenalty, SignalLLM, SignalConflict:
		return SignalCode(s), nil
	default:
		return "", fmt.Errorf("unknown SignalCode %q: valid values are %v", s, ValuesSignalCode())
	}
}

func ValuesSignalCode() []SignalCode {
	return []SignalCode{SignalGateRole, SignalGateHard, SignalCutLocation, SignalCutTimezone, SignalCutPostedAt, SignalPlus, SignalPenalty, SignalLLM, SignalConflict}
}

func FromStringSignalCode(s string) (SignalCode, error) {
	var z SignalCode
	return z.FromValue(s)
}

// Signal is one fired rule with its contribution. Delta is 0 for gates.
type Signal struct {
	Code        SignalCode `json:"code"`
	Rule        string     `json:"rule"`
	Phrase      string     `json:"phrase,omitempty"`
	Delta       int        `json:"delta"`
	Occurrences int        `json:"occurrences,omitempty"`
	Value       string     `json:"value,omitempty"`
}
