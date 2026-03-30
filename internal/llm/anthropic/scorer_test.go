package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestScorer_Score_happyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "sk-test", r.Header.Get("x-api-key"))
		require.Equal(t, defaultAnthropicVer, r.Header.Get("anthropic-version"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
  "content": [
    {"type": "text", "text": "{\"score\": 77, \"rationale\": \"good fit\"}"}
  ]
}`))
	}))
	t.Cleanup(srv.Close)

	s := NewScorer("sk-test", "claude-test")
	s.BaseURL = srv.URL
	s.HTTPClient = srv.Client()

	sj, err := s.Score(context.Background(), "profile", domain.Job{ID: "1", Title: "Dev"})
	require.NoError(t, err)
	require.Equal(t, 77, sj.Score)
	require.Equal(t, "good fit", sj.Reason)
	require.Equal(t, "1", sj.Job.ID)
}

func TestScorer_Score_emptyKey(t *testing.T) {
	s := NewScorer("", "m")
	_, err := s.Score(context.Background(), "p", domain.Job{})
	require.Error(t, err)
}

func TestScorer_Score_emptyModel(t *testing.T) {
	s := NewScorer("k", "")
	_, err := s.Score(context.Background(), "p", domain.Job{})
	require.Error(t, err)
}

func TestScorer_Score_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error"}}`))
	}))
	t.Cleanup(srv.Close)

	s := NewScorer("k", "m")
	s.BaseURL = srv.URL
	s.HTTPClient = srv.Client()

	_, err := s.Score(context.Background(), "p", domain.Job{})
	require.Error(t, err)
}

func TestParseScoringFromAssistantText_fencedFallback(t *testing.T) {
	text := "Here is JSON:\n```json\n{\"score\": 10, \"rationale\": \"x\"}\n```"
	sc, r, err := parseScoringFromAssistantText(text)
	require.NoError(t, err)
	require.Equal(t, 10, sc)
	require.Equal(t, "x", r)
}
