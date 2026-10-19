package schema

import "fmt"

// EmploymentLoad is a full-time / part-time answer. "unknown" is not a member.
type EmploymentLoad string

const (
	EmploymentLoadFullTime EmploymentLoad = "full_time"
	EmploymentLoadPartTime EmploymentLoad = "part_time"
)

func (e EmploymentLoad) String() string { return string(e) }

func (e EmploymentLoad) Equals(str string) bool { return string(e) == str }

func (e EmploymentLoad) Pointer() *EmploymentLoad { return &e }

func (e EmploymentLoad) FromValue(str string) (EmploymentLoad, error) {
	switch EmploymentLoad(str) {
	case EmploymentLoadFullTime, EmploymentLoadPartTime:
		return EmploymentLoad(str), nil
	default:
		return "", fmt.Errorf("unknown EmploymentLoad %q: valid values are %v", str, ValuesEmploymentLoad())
	}
}

func ValuesEmploymentLoad() []EmploymentLoad {
	return []EmploymentLoad{EmploymentLoadFullTime, EmploymentLoadPartTime}
}

func FromStringEmploymentLoad(str string) (EmploymentLoad, error) {
	var z EmploymentLoad
	return z.FromValue(str)
}
