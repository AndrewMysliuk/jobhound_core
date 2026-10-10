package schema

import "fmt"

// SignalCode identifies one scoring hit or one hard cutoff.
type SignalCode string

const (
	SignalQuery           SignalCode = "QUERY"
	SignalPenalty         SignalCode = "PENALTY"
	SignalCutExcludeTitle SignalCode = "CUT_EXCLUDE_TITLE"
	SignalCutExcludeText  SignalCode = "CUT_EXCLUDE_TEXT"
)

func (c SignalCode) String() string { return string(c) }

func (c SignalCode) Equals(s string) bool { return string(c) == s }

func (c SignalCode) Pointer() *SignalCode { return &c }

func (c SignalCode) FromValue(s string) (SignalCode, error) {
	switch SignalCode(s) {
	case SignalQuery, SignalPenalty, SignalCutExcludeTitle, SignalCutExcludeText:
		return SignalCode(s), nil
	default:
		return "", fmt.Errorf("unknown SignalCode %q: valid values are %v", s, ValuesSignalCode())
	}
}

func ValuesSignalCode() []SignalCode {
	return []SignalCode{SignalQuery, SignalPenalty, SignalCutExcludeTitle, SignalCutExcludeText}
}

func FromStringSignalCode(s string) (SignalCode, error) {
	var z SignalCode
	return z.FromValue(s)
}

// Signal is one fired phrase with its contribution.
type Signal struct {
	Code   SignalCode `json:"code"`
	Points int        `json:"points"`
	Terms  []string   `json:"terms,omitempty"`
}
