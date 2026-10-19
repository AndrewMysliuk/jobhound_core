package impl

import (
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

func TestCompareEvalPerfectMatch(t *testing.T) {
	gold := []EvalExample{
		{ID: "a", Facts: unknownModelReply()},
		{ID: "b", Facts: withOffice(unknownModelReply(), schema.Office{
			Countries: []string{"US"},
			Regions:   []string{},
			Quote:     strPtr("US"),
			Unknown:   false,
		})},
	}
	metrics := CompareEval(gold, gold)
	byField := map[schema.FieldName]FieldEvalMetrics{}
	for _, m := range metrics {
		byField[m.Field] = m
	}

	office := byField[schema.FieldNameOffice]
	if office.Labeled != 1 || office.Correct != 1 || office.Accuracy != 1 {
		t.Fatalf("office accuracy: %+v", office)
	}
	if office.ConfidentErrors != 0 {
		t.Fatalf("office confident errors: %+v", office)
	}
	if office.PredictedUnknown != 1 {
		t.Fatalf("office predicted unknown: %+v", office)
	}
}

func TestCompareEvalConfidentError(t *testing.T) {
	gold := []EvalExample{{ID: "a", Facts: unknownModelReply()}}
	pred := []EvalExample{{ID: "a", Facts: withOffice(unknownModelReply(), schema.Office{
		Countries: []string{"US"},
		Regions:   []string{},
		Quote:     strPtr("US"),
		Unknown:   false,
	})}}
	metrics := CompareEval(gold, pred)
	var office FieldEvalMetrics
	for _, m := range metrics {
		if m.Field == schema.FieldNameOffice {
			office = m
			break
		}
	}
	if office.ConfidentErrors != 1 || office.Labeled != 0 || office.PredictedUnknown != 0 {
		t.Fatalf("office: %+v", office)
	}
}
