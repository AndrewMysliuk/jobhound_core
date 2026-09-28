package schema

import "fmt"

// RegionCode is a named hiring region from a job board (not ISO country).
type RegionCode string

const (
	RegionCodeWorldwide    RegionCode = "WORLDWIDE"
	RegionCodeEurope       RegionCode = "EUROPE"
	RegionCodeEU           RegionCode = "EU"
	RegionCodeEMEA         RegionCode = "EMEA"
	RegionCodeNorthAmerica RegionCode = "NORTH_AMERICA"
	RegionCodeLATAM        RegionCode = "LATAM"
	RegionCodeAPAC         RegionCode = "APAC"
	RegionCodeAfrica       RegionCode = "AFRICA"
	RegionCodeMiddleEast   RegionCode = "MIDDLE_EAST"
)

func (c RegionCode) String() string { return string(c) }

func (c RegionCode) Equals(s string) bool { return string(c) == s }

func (c RegionCode) Pointer() *RegionCode { return &c }

func (c RegionCode) FromValue(s string) (RegionCode, error) {
	switch s {
	case string(RegionCodeWorldwide):
		return RegionCodeWorldwide, nil
	case string(RegionCodeEurope):
		return RegionCodeEurope, nil
	case string(RegionCodeEU):
		return RegionCodeEU, nil
	case string(RegionCodeEMEA):
		return RegionCodeEMEA, nil
	case string(RegionCodeNorthAmerica):
		return RegionCodeNorthAmerica, nil
	case string(RegionCodeLATAM):
		return RegionCodeLATAM, nil
	case string(RegionCodeAPAC):
		return RegionCodeAPAC, nil
	case string(RegionCodeAfrica):
		return RegionCodeAfrica, nil
	case string(RegionCodeMiddleEast):
		return RegionCodeMiddleEast, nil
	default:
		return "", fmt.Errorf("unknown RegionCode %q: valid values are %v", s, ValuesRegionCode())
	}
}

func ValuesRegionCode() []RegionCode {
	return []RegionCode{
		RegionCodeWorldwide, RegionCodeEurope, RegionCodeEU, RegionCodeEMEA,
		RegionCodeNorthAmerica, RegionCodeLATAM, RegionCodeAPAC, RegionCodeAfrica, RegionCodeMiddleEast,
	}
}

func FromStringRegionCode(s string) (RegionCode, error) {
	var z RegionCode
	return z.FromValue(s)
}

// PositionLabel is the canonical engineering role inferred from listing text (collectors MVP).
type PositionLabel string

const (
	PositionLabelFrontend  PositionLabel = "frontend"
	PositionLabelBackend   PositionLabel = "backend"
	PositionLabelFullStack PositionLabel = "full-stack"
)

func (p PositionLabel) String() string { return string(p) }

func (p PositionLabel) Equals(s string) bool { return string(p) == s }

func (p PositionLabel) Pointer() *PositionLabel { return &p }

func (p PositionLabel) FromValue(s string) (PositionLabel, error) {
	switch s {
	case string(PositionLabelFrontend):
		return PositionLabelFrontend, nil
	case string(PositionLabelBackend):
		return PositionLabelBackend, nil
	case string(PositionLabelFullStack):
		return PositionLabelFullStack, nil
	default:
		return "", fmt.Errorf("unknown PositionLabel %q: valid values are %v", s, ValuesPositionLabel())
	}
}

func ValuesPositionLabel() []PositionLabel {
	return []PositionLabel{PositionLabelFrontend, PositionLabelBackend, PositionLabelFullStack}
}

func FromStringPositionLabel(s string) (PositionLabel, error) {
	var z PositionLabel
	return z.FromValue(s)
}
