package pipeline_activities

import (
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipelineschema "github.com/andrewmysliuk/jobhound_core/internal/pipeline/schema"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
	"github.com/stretchr/testify/require"
)

func evalStage2(t *testing.T, j schema.Job, rules []pipelineschema.Stage2Rule) pipeutils.Stage2Evaluation {
	t.Helper()
	ev, err := pipeutils.EvaluateStage2(j, rules, nil)
	require.NoError(t, err)
	return ev
}

func TestStage2Rules_phraseCSharpAndNegation(t *testing.T) {
	t.Parallel()
	jC := schema.Job{Title: "C# Developer", Description: ""}
	ev := evalStage2(t, jC, []pipelineschema.Stage2Rule{{
		ID: "csharp", Field: pipelineschema.RuleFieldTitle, Op: pipelineschema.RuleOpPhrase,
		Values: []string{"c#"}, Action: pipelineschema.RuleActionReject,
	}})
	require.Equal(t, pipeline.RunJobRejectedStage2, ev.Status)

	jOnCall := schema.Job{Title: "SRE", Description: "includes on-call rotation"}
	ev = evalStage2(t, jOnCall, []pipelineschema.Stage2Rule{{
		ID: "oncall", Field: pipelineschema.RuleFieldBody, Op: pipelineschema.RuleOpPhrase,
		Values: []string{"on-call"}, Action: pipelineschema.RuleActionFlag,
	}})
	require.Equal(t, pipeline.RunJobUnknownStage2, ev.Status)
	require.Len(t, ev.Hits, 1)

	jNoOnCall := schema.Job{Title: "SRE", Description: "no on-call duty here"}
	ev = evalStage2(t, jNoOnCall, []pipelineschema.Stage2Rule{{
		ID: "oncall", Field: pipelineschema.RuleFieldBody, Op: pipelineschema.RuleOpPhrase,
		Values: []string{"on call"}, NegationWindow: 3, Action: pipelineschema.RuleActionFlag,
	}})
	require.Equal(t, pipeline.RunJobUnknownStage2, ev.Status)
	require.Empty(t, ev.Hits)

	jNetEmbed := schema.Job{Title: "Architect", Description: "service.network design"}
	ev = evalStage2(t, jNetEmbed, []pipelineschema.Stage2Rule{{
		ID: "dotnet", Field: pipelineschema.RuleFieldBody, Op: pipelineschema.RuleOpPhrase,
		Values: []string{".net"}, Action: pipelineschema.RuleActionReject,
	}})
	require.Equal(t, pipeline.RunJobUnknownStage2, ev.Status)
	require.Empty(t, ev.Hits)
}

func TestStage2Rules_titlePhraseDoesNotSeeBody(t *testing.T) {
	t.Parallel()
	j := schema.Job{Title: "PM", Description: "must know backend systems"}
	ev := evalStage2(t, j, []pipelineschema.Stage2Rule{{
		ID: "t", Field: pipelineschema.RuleFieldTitle, Op: pipelineschema.RuleOpPhrase,
		Values: []string{"backend"}, Action: pipelineschema.RuleActionReject,
	}})
	require.Equal(t, pipeline.RunJobUnknownStage2, ev.Status)

	ev = evalStage2(t, j, []pipelineschema.Stage2Rule{{
		ID: "tb", Field: pipelineschema.RuleFieldTitleBody, Op: pipelineschema.RuleOpPhrase,
		Values: []string{"backend"}, Action: pipelineschema.RuleActionReject,
	}})
	require.Equal(t, pipeline.RunJobRejectedStage2, ev.Status)
}

func TestStage2Rules_countriesAllowedParsing(t *testing.T) {
	t.Parallel()
	jUS := schema.Job{HiringCountries: []string{"US"}}
	parsed, err := pipeutils.ParseListing(jUS)
	require.NoError(t, err)
	require.Contains(t, parsed.CountriesAllowed, "US")

	jBare := schema.Job{Title: "IT contractor", Description: "not US based"}
	parsed, err = pipeutils.ParseListing(jBare)
	require.NoError(t, err)
	require.Empty(t, parsed.CountriesAllowed)
}

func TestStage2Rules_countriesExcludesWhen(t *testing.T) {
	t.Parallel()
	j := schema.Job{Title: "Anywhere", Description: "fully remote"}
	explicit := pipelineschema.Stage2Rule{
		ID: "geo", Field: pipelineschema.RuleFieldCountriesAllowed, Op: pipelineschema.RuleOpExcludes,
		Values: []string{"RO"}, Action: pipelineschema.RuleActionReject,
	}
	ev := evalStage2(t, j, []pipelineschema.Stage2Rule{explicit})
	require.Equal(t, pipeline.RunJobUnknownStage2, ev.Status)

	always := explicit
	always.When = pipelineschema.RuleWhenAlways
	ev = evalStage2(t, j, []pipelineschema.Stage2Rule{always})
	require.Equal(t, pipeline.RunJobRejectedStage2, ev.Status)
}

func TestStage2Rules_flagPenaltyBoostRejectPrecedence(t *testing.T) {
	t.Parallel()
	j := schema.Job{Title: "Go Dev", Description: "backend"}
	wBoost := 5
	wPen := -2
	ev := evalStage2(t, j, []pipelineschema.Stage2Rule{
		{ID: "f", Field: pipelineschema.RuleFieldBody, Op: pipelineschema.RuleOpPhrase,
			Values: []string{"backend"}, Action: pipelineschema.RuleActionFlag},
	})
	require.Equal(t, pipeline.RunJobUnknownStage2, ev.Status)
	require.Len(t, ev.Hits, 1)

	ev = evalStage2(t, j, []pipelineschema.Stage2Rule{
		{ID: "p", Field: pipelineschema.RuleFieldBody, Op: pipelineschema.RuleOpPhrase,
			Values: []string{"backend"}, Action: pipelineschema.RuleActionPenalty, Weight: &wPen},
	})
	require.Equal(t, pipeline.RunJobUnknownStage2, ev.Status)
	require.Equal(t, -2, ev.Boost)

	ev = evalStage2(t, j, []pipelineschema.Stage2Rule{
		{ID: "rej", Field: pipelineschema.RuleFieldTitle, Op: pipelineschema.RuleOpPhrase,
			Values: []string{"go"}, Action: pipelineschema.RuleActionReject},
		{ID: "b", Field: pipelineschema.RuleFieldTitle, Op: pipelineschema.RuleOpPhrase,
			Values: []string{"go"}, Action: pipelineschema.RuleActionBoost, Weight: &wBoost},
	})
	require.Equal(t, pipeline.RunJobRejectedStage2, ev.Status)
}
