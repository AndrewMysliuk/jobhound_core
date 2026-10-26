package ingest

// architectureSourceIDs use a fixed twelve-hour cooldown (see coordinator).
var architectureSourceIDs = map[string]struct{}{
	"bestjobs":            {},
	"hipo":                {},
	"ejobs":               {},
	"dezeen_jobs":         {},
	"riba_jobs":           {},
	"landscape_institute": {},
	"archgee":             {},
	"archinect":           {},
	"competitionline":     {},
	"baunetz":             {},
	"bdla":                {},
	"world_architects":    {},
}
