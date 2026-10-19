package schema

import "fmt"

// WorkMode is a work-mode answer. "unknown" is not a member.
type WorkMode string

const (
	WorkModeRemote WorkMode = "remote"
	WorkModeHybrid WorkMode = "hybrid"
	WorkModeOffice WorkMode = "office"
)

func (m WorkMode) String() string { return string(m) }

func (m WorkMode) Equals(str string) bool { return string(m) == str }

func (m WorkMode) Pointer() *WorkMode { return &m }

func (m WorkMode) FromValue(str string) (WorkMode, error) {
	switch WorkMode(str) {
	case WorkModeRemote, WorkModeHybrid, WorkModeOffice:
		return WorkMode(str), nil
	default:
		return "", fmt.Errorf("unknown WorkMode %q: valid values are %v", str, ValuesWorkMode())
	}
}

func ValuesWorkMode() []WorkMode {
	return []WorkMode{WorkModeRemote, WorkModeHybrid, WorkModeOffice}
}

func FromStringWorkMode(str string) (WorkMode, error) {
	var z WorkMode
	return z.FromValue(str)
}
