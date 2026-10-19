package schema

import "fmt"

// Engagement is an employment / contract answer. "unknown" is not a member.
type Engagement string

const (
	EngagementEmployment Engagement = "employment"
	EngagementContract   Engagement = "contract"
	EngagementOther      Engagement = "other"
)

func (e Engagement) String() string { return string(e) }

func (e Engagement) Equals(str string) bool { return string(e) == str }

func (e Engagement) Pointer() *Engagement { return &e }

func (e Engagement) FromValue(str string) (Engagement, error) {
	switch Engagement(str) {
	case EngagementEmployment, EngagementContract, EngagementOther:
		return Engagement(str), nil
	default:
		return "", fmt.Errorf("unknown Engagement %q: valid values are %v", str, ValuesEngagement())
	}
}

func ValuesEngagement() []Engagement {
	return []Engagement{EngagementEmployment, EngagementContract, EngagementOther}
}

func FromStringEngagement(str string) (Engagement, error) {
	var z Engagement
	return z.FromValue(str)
}
