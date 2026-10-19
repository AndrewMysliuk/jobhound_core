package schema

import "fmt"

// PayPeriod is a compensation period. "unknown" is not a member.
type PayPeriod string

const (
	PayPeriodYear  PayPeriod = "year"
	PayPeriodMonth PayPeriod = "month"
	PayPeriodWeek  PayPeriod = "week"
	PayPeriodDay   PayPeriod = "day"
	PayPeriodHour  PayPeriod = "hour"
)

func (p PayPeriod) String() string { return string(p) }

func (p PayPeriod) Equals(str string) bool { return string(p) == str }

func (p PayPeriod) Pointer() *PayPeriod { return &p }

func (p PayPeriod) FromValue(str string) (PayPeriod, error) {
	switch PayPeriod(str) {
	case PayPeriodYear, PayPeriodMonth, PayPeriodWeek, PayPeriodDay, PayPeriodHour:
		return PayPeriod(str), nil
	default:
		return "", fmt.Errorf("unknown PayPeriod %q: valid values are %v", str, ValuesPayPeriod())
	}
}

func ValuesPayPeriod() []PayPeriod {
	return []PayPeriod{PayPeriodYear, PayPeriodMonth, PayPeriodWeek, PayPeriodDay, PayPeriodHour}
}

func FromStringPayPeriod(str string) (PayPeriod, error) {
	var z PayPeriod
	return z.FromValue(str)
}
