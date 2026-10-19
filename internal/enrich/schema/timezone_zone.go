package schema

import "fmt"

// TimezoneZone is a named zone or zone band used in timezone constraints.
type TimezoneZone string

const (
	TimezoneZoneCET        TimezoneZone = "CET"
	TimezoneZoneGMT        TimezoneZone = "GMT"
	TimezoneZoneEET        TimezoneZone = "EET"
	TimezoneZoneUSEastern  TimezoneZone = "US_EASTERN"
	TimezoneZoneUSCentral  TimezoneZone = "US_CENTRAL"
	TimezoneZoneUSMountain TimezoneZone = "US_MOUNTAIN"
	TimezoneZoneUSPacific  TimezoneZone = "US_PACIFIC"
	TimezoneZoneIST        TimezoneZone = "IST"
	TimezoneZoneBRT        TimezoneZone = "BRT"
	TimezoneZoneAmericas   TimezoneZone = "AMERICAS"
	TimezoneZoneAPAC       TimezoneZone = "APAC"
)

func (z TimezoneZone) String() string { return string(z) }

func (z TimezoneZone) Equals(str string) bool { return string(z) == str }

func (z TimezoneZone) Pointer() *TimezoneZone { return &z }

func (z TimezoneZone) FromValue(str string) (TimezoneZone, error) {
	switch TimezoneZone(str) {
	case TimezoneZoneCET, TimezoneZoneGMT, TimezoneZoneEET,
		TimezoneZoneUSEastern, TimezoneZoneUSCentral, TimezoneZoneUSMountain, TimezoneZoneUSPacific,
		TimezoneZoneIST, TimezoneZoneBRT, TimezoneZoneAmericas, TimezoneZoneAPAC:
		return TimezoneZone(str), nil
	default:
		return "", fmt.Errorf("unknown TimezoneZone %q: valid values are %v", str, ValuesTimezoneZone())
	}
}

func ValuesTimezoneZone() []TimezoneZone {
	return []TimezoneZone{
		TimezoneZoneCET, TimezoneZoneGMT, TimezoneZoneEET,
		TimezoneZoneUSEastern, TimezoneZoneUSCentral, TimezoneZoneUSMountain, TimezoneZoneUSPacific,
		TimezoneZoneIST, TimezoneZoneBRT, TimezoneZoneAmericas, TimezoneZoneAPAC,
	}
}

func FromStringTimezoneZone(str string) (TimezoneZone, error) {
	var z TimezoneZone
	return z.FromValue(str)
}
