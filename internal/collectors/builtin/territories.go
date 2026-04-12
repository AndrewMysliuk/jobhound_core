package builtin

// territory is one Built In listing filter (ISO 3166-1 alpha-3 → alpha-2) from resources/builtin.md.
type territory struct {
	Alpha3 string
	Alpha2 string
}

// normativeTerritories is the Built In listing filter order: large EU tech/remote markets + GB + UA.
// Subset of the former EU-27+GB+UA scope for fewer round-trips and faster ingest (resources/builtin.md).
var normativeTerritories = []territory{
	{"DEU", "DE"}, {"NLD", "NL"}, {"POL", "PL"}, {"FRA", "FR"}, {"ESP", "ES"}, {"ITA", "IT"},
	{"IRL", "IE"}, {"SWE", "SE"}, {"BEL", "BE"}, {"AUT", "AT"}, {"CZE", "CZ"}, {"PRT", "PT"},
	{"ROU", "RO"}, {"GRC", "GR"}, {"FIN", "FI"}, {"DNK", "DK"}, {"HUN", "HU"},
	{"GBR", "GB"}, {"UKR", "UA"},
}
