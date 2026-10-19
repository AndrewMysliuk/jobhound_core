package schema

import "fmt"

// Scope is a hiring-scope or timezone constraint value. "unknown" is not a member.
type Scope string

const (
	ScopeWorldwide  Scope = "worldwide"
	ScopeRestricted Scope = "restricted"
)

func (s Scope) String() string { return string(s) }

func (s Scope) Equals(str string) bool { return string(s) == str }

func (s Scope) Pointer() *Scope { return &s }

func (s Scope) FromValue(str string) (Scope, error) {
	switch Scope(str) {
	case ScopeWorldwide, ScopeRestricted:
		return Scope(str), nil
	default:
		return "", fmt.Errorf("unknown Scope %q: valid values are %v", str, ValuesScope())
	}
}

func ValuesScope() []Scope {
	return []Scope{ScopeWorldwide, ScopeRestricted}
}

func FromStringScope(str string) (Scope, error) {
	var z Scope
	return z.FromValue(str)
}
