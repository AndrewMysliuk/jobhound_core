package builtin

// territory is one Built In listing filter (ISO 3166-1 alpha-3 → alpha-2) from resources/builtin.md.
type territory struct {
	Alpha3 string
	Alpha2 string
}

// normativeTerritories is the fixed EU-27 + GB + UA set in listing order (resources/builtin.md).
var normativeTerritories = []territory{
	{"AUT", "AT"}, {"BEL", "BE"}, {"BGR", "BG"}, {"HRV", "HR"}, {"CYP", "CY"}, {"CZE", "CZ"},
	{"DEU", "DE"}, {"DNK", "DK"}, {"EST", "EE"}, {"ESP", "ES"}, {"FIN", "FI"}, {"FRA", "FR"},
	{"GRC", "GR"}, {"HUN", "HU"}, {"IRL", "IE"}, {"ITA", "IT"}, {"LVA", "LV"}, {"LTU", "LT"},
	{"LUX", "LU"}, {"MLT", "MT"}, {"NLD", "NL"}, {"POL", "PL"}, {"PRT", "PT"}, {"ROU", "RO"},
	{"SVK", "SK"}, {"SVN", "SI"}, {"SWE", "SE"}, {"GBR", "GB"}, {"UKR", "UA"},
}
