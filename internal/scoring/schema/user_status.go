package schema

import "fmt"

// UserStatus is the viewer's mark on a match. Rescoring does not overwrite it.
type UserStatus string

const (
	UserStatusNew    UserStatus = "NEW"
	UserStatusHidden UserStatus = "HIDDEN"
)

func (u UserStatus) String() string { return string(u) }

func (u UserStatus) Equals(s string) bool { return string(u) == s }

func (u UserStatus) Pointer() *UserStatus { return &u }

func (u UserStatus) FromValue(s string) (UserStatus, error) {
	switch UserStatus(s) {
	case UserStatusNew, UserStatusHidden:
		return UserStatus(s), nil
	default:
		return "", fmt.Errorf("unknown UserStatus %q: valid values are %v", s, ValuesUserStatus())
	}
}

func ValuesUserStatus() []UserStatus {
	return []UserStatus{UserStatusNew, UserStatusHidden}
}

func FromStringUserStatus(s string) (UserStatus, error) {
	var z UserStatus
	return z.FromValue(s)
}
