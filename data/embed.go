// Package data holds embedded reference JSON used by stage-2 parsing.
package data

import _ "embed"

//go:embed geo_regions.json
var GeoRegionsJSON []byte
