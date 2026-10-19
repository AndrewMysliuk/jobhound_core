package schema

import "fmt"

// Authorization is a work-authorization answer. "unknown" is not a member.
type Authorization string

const (
	AuthorizationNone     Authorization = "none"
	AuthorizationRequired Authorization = "required"
)

func (a Authorization) String() string { return string(a) }

func (a Authorization) Equals(str string) bool { return string(a) == str }

func (a Authorization) Pointer() *Authorization { return &a }

func (a Authorization) FromValue(str string) (Authorization, error) {
	switch Authorization(str) {
	case AuthorizationNone, AuthorizationRequired:
		return Authorization(str), nil
	default:
		return "", fmt.Errorf("unknown Authorization %q: valid values are %v", str, ValuesAuthorization())
	}
}

func ValuesAuthorization() []Authorization {
	return []Authorization{AuthorizationNone, AuthorizationRequired}
}

func FromStringAuthorization(str string) (Authorization, error) {
	var z Authorization
	return z.FromValue(str)
}
