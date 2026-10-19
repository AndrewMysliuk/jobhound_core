package schema

import "fmt"

// FieldName is one of the eight facts answers.
type FieldName string

const (
	FieldNameOffice            FieldName = "office"
	FieldNameHiringScope       FieldName = "hiring_scope"
	FieldNameWorkAuthorization FieldName = "work_authorization"
	FieldNameTimezone          FieldName = "timezone"
	FieldNameCompensation      FieldName = "compensation"
	FieldNameWorkMode          FieldName = "work_mode"
	FieldNameEmploymentLoad    FieldName = "employment_load"
	FieldNameEngagement        FieldName = "engagement"
)

func (f FieldName) String() string { return string(f) }

func (f FieldName) Equals(str string) bool { return string(f) == str }

func (f FieldName) Pointer() *FieldName { return &f }

func (f FieldName) FromValue(str string) (FieldName, error) {
	switch FieldName(str) {
	case FieldNameOffice, FieldNameHiringScope, FieldNameWorkAuthorization, FieldNameTimezone,
		FieldNameCompensation, FieldNameWorkMode, FieldNameEmploymentLoad, FieldNameEngagement:
		return FieldName(str), nil
	default:
		return "", fmt.Errorf("unknown FieldName %q: valid values are %v", str, ValuesFieldName())
	}
}

func ValuesFieldName() []FieldName {
	return []FieldName{
		FieldNameOffice, FieldNameHiringScope, FieldNameWorkAuthorization, FieldNameTimezone,
		FieldNameCompensation, FieldNameWorkMode, FieldNameEmploymentLoad, FieldNameEngagement,
	}
}

func FromStringFieldName(str string) (FieldName, error) {
	var z FieldName
	return z.FromValue(str)
}
