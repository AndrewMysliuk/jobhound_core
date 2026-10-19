package schema

import "fmt"

// Region is a named hiring or office region used in facts answers.
type Region string

const (
	RegionWorldwide Region = "WORLDWIDE"
	RegionEurope    Region = "EUROPE"
	RegionEMEA      Region = "EMEA"
	RegionAmericas  Region = "AMERICAS"
	RegionLATAM     Region = "LATAM"
	RegionAPAC      Region = "APAC"
	RegionAfrica    Region = "AFRICA"
	RegionMENA      Region = "MENA"
)

func (r Region) String() string { return string(r) }

func (r Region) Equals(str string) bool { return string(r) == str }

func (r Region) Pointer() *Region { return &r }

func (r Region) FromValue(str string) (Region, error) {
	switch Region(str) {
	case RegionWorldwide, RegionEurope, RegionEMEA, RegionAmericas, RegionLATAM, RegionAPAC, RegionAfrica, RegionMENA:
		return Region(str), nil
	default:
		return "", fmt.Errorf("unknown Region %q: valid values are %v", str, ValuesRegion())
	}
}

func ValuesRegion() []Region {
	return []Region{
		RegionWorldwide, RegionEurope, RegionEMEA, RegionAmericas,
		RegionLATAM, RegionAPAC, RegionAfrica, RegionMENA,
	}
}

func FromStringRegion(str string) (Region, error) {
	var z Region
	return z.FromValue(str)
}
