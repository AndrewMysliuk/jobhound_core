package multi

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/domain"
)

type stubCollector struct {
	name string
	jobs []domain.Job
	err  error
}

func (s stubCollector) Name() string { return s.name }

func (s stubCollector) Fetch(context.Context) ([]domain.Job, error) {
	return s.jobs, s.err
}

func TestAll_Fetch_mergesAndContinuesOnError(t *testing.T) {
	var logged []string
	a := &All{
		Collectors: []collectors.Collector{
			stubCollector{name: "bad", err: errors.New("boom")},
			stubCollector{name: "good", jobs: []domain.Job{{Title: "A"}, {Title: "B"}}},
		},
		OnSourceError: func(name string, err error) {
			logged = append(logged, name+":"+err.Error())
		},
	}
	out, err := a.Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, "A", out[0].Title)
	require.Equal(t, []string{"bad:boom"}, logged)
}

func TestAll_Fetch_allFail(t *testing.T) {
	a := &All{
		Collectors: []collectors.Collector{
			stubCollector{name: "a", err: errors.New("e1")},
			stubCollector{name: "b", err: errors.New("e2")},
		},
		OnSourceError: func(string, error) {},
	}
	_, err := a.Fetch(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "e1")
	require.Contains(t, err.Error(), "e2")
}
