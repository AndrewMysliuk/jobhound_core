package impl

import (
	"fmt"
	"slices"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

// EvalExample is one gold or prediction line: id plus facts.
type EvalExample struct {
	ID    string            `json:"id"`
	Facts schema.ModelReply `json:"facts"`
}

// FieldEvalMetrics are the three printouts for one facts field.
type FieldEvalMetrics struct {
	Field             schema.FieldName
	Labeled           int     // gold not unknown
	Correct           int     // labeled and prediction matches gold
	Accuracy          float64 // Correct / Labeled; 0 when Labeled is 0
	PredictedUnknown  int
	PredictedUnknownP float64 // PredictedUnknown / N
	ConfidentErrors   int     // predicted a value; gold differs or is unknown
	ConfidentErrorsP  float64 // ConfidentErrors / N
	N                 int
}

// CompareEval joins gold and predictions by id and scores each field.
// Missing ids on either side are skipped for that pair.
func CompareEval(gold, predictions []EvalExample) []FieldEvalMetrics {
	predByID := make(map[string]schema.ModelReply, len(predictions))
	for _, p := range predictions {
		predByID[p.ID] = p.Facts
	}

	var pairs []struct {
		gold, pred schema.ModelReply
	}
	for _, g := range gold {
		pred, ok := predByID[g.ID]
		if !ok {
			continue
		}
		pairs = append(pairs, struct {
			gold, pred schema.ModelReply
		}{gold: g.Facts, pred: pred})
	}

	out := make([]FieldEvalMetrics, 0, len(schema.ValuesFieldName()))
	for _, field := range schema.ValuesFieldName() {
		m := FieldEvalMetrics{Field: field, N: len(pairs)}
		for _, pair := range pairs {
			gUnknown := fieldIsUnknown(field, pair.gold)
			pUnknown := fieldIsUnknown(field, pair.pred)
			if pUnknown {
				m.PredictedUnknown++
			}
			if !gUnknown {
				m.Labeled++
				if fieldEqual(field, pair.gold, pair.pred) {
					m.Correct++
				}
			}
			if !pUnknown && (gUnknown || !fieldEqual(field, pair.gold, pair.pred)) {
				m.ConfidentErrors++
			}
		}
		if m.Labeled > 0 {
			m.Accuracy = float64(m.Correct) / float64(m.Labeled)
		}
		if m.N > 0 {
			m.PredictedUnknownP = float64(m.PredictedUnknown) / float64(m.N)
			m.ConfidentErrorsP = float64(m.ConfidentErrors) / float64(m.N)
		}
		out = append(out, m)
	}
	return out
}

// FormatEvalMetrics prints one line per field for make eval.
func FormatEvalMetrics(metrics []FieldEvalMetrics) string {
	var b strings.Builder
	for _, m := range metrics {
		fmt.Fprintf(&b,
			"%s\taccuracy=%.4f (%d/%d)\tpredicted_unknown=%.4f (%d/%d)\tconfident_errors=%.4f (%d/%d)\n",
			m.Field.String(),
			m.Accuracy, m.Correct, m.Labeled,
			m.PredictedUnknownP, m.PredictedUnknown, m.N,
			m.ConfidentErrorsP, m.ConfidentErrors, m.N,
		)
	}
	return b.String()
}

func fieldIsUnknown(field schema.FieldName, r schema.ModelReply) bool {
	switch field {
	case schema.FieldNameOffice:
		return r.Office.Unknown
	case schema.FieldNameHiringScope:
		return r.HiringScope.Value == unknownAnswer || r.HiringScope.Value == ""
	case schema.FieldNameWorkAuthorization:
		return r.WorkAuthorization.Value == unknownAnswer || r.WorkAuthorization.Value == ""
	case schema.FieldNameTimezone:
		return r.Timezone.Value == unknownAnswer || r.Timezone.Value == ""
	case schema.FieldNameCompensation:
		return r.Compensation.Unknown
	case schema.FieldNameWorkMode:
		return r.WorkMode.Value == unknownAnswer || r.WorkMode.Value == ""
	case schema.FieldNameEmploymentLoad:
		return r.EmploymentLoad.Value == unknownAnswer || r.EmploymentLoad.Value == ""
	case schema.FieldNameEngagement:
		return r.Engagement.Value == unknownAnswer || r.Engagement.Value == ""
	default:
		return true
	}
}

func fieldEqual(field schema.FieldName, a, b schema.ModelReply) bool {
	switch field {
	case schema.FieldNameOffice:
		if a.Office.Unknown != b.Office.Unknown {
			return false
		}
		if a.Office.Unknown {
			return true
		}
		return sameStringSet(a.Office.Countries, b.Office.Countries) &&
			sameStringSet(a.Office.Regions, b.Office.Regions)
	case schema.FieldNameHiringScope:
		if a.HiringScope.Value != b.HiringScope.Value {
			return false
		}
		if a.HiringScope.Value == unknownAnswer {
			return true
		}
		return sameStringSet(a.HiringScope.Countries, b.HiringScope.Countries) &&
			sameStringSet(a.HiringScope.Regions, b.HiringScope.Regions)
	case schema.FieldNameWorkAuthorization:
		if a.WorkAuthorization.Value != b.WorkAuthorization.Value {
			return false
		}
		if a.WorkAuthorization.Value == unknownAnswer {
			return true
		}
		return sameStringSet(a.WorkAuthorization.Countries, b.WorkAuthorization.Countries)
	case schema.FieldNameTimezone:
		if a.Timezone.Value != b.Timezone.Value {
			return false
		}
		if a.Timezone.Value == unknownAnswer {
			return true
		}
		return sameStringSet(a.Timezone.Zones, b.Timezone.Zones)
	case schema.FieldNameCompensation:
		if a.Compensation.Unknown != b.Compensation.Unknown {
			return false
		}
		if a.Compensation.Unknown {
			return true
		}
		return a.Compensation.Currency == b.Compensation.Currency &&
			a.Compensation.Period == b.Compensation.Period &&
			floatPtrEqual(a.Compensation.Min, b.Compensation.Min) &&
			floatPtrEqual(a.Compensation.Max, b.Compensation.Max)
	case schema.FieldNameWorkMode:
		return a.WorkMode.Value == b.WorkMode.Value
	case schema.FieldNameEmploymentLoad:
		return a.EmploymentLoad.Value == b.EmploymentLoad.Value
	case schema.FieldNameEngagement:
		return a.Engagement.Value == b.Engagement.Value
	default:
		return false
	}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	slices.Sort(ac)
	slices.Sort(bc)
	return slices.Equal(ac, bc)
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
