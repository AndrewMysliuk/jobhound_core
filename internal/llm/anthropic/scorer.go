// Package anthropic implements llm.Scorer against the Anthropic Messages API.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/domain"
	llmutils "github.com/andrewmysliuk/jobhound_core/internal/llm/utils"
)

const (
	defaultBaseURL        = "https://api.anthropic.com"
	defaultAnthropicVer   = "2023-06-01"
	defaultScoreMaxTokens = 256
)

// Scorer calls Claude and maps responses to domain.ScoredJob using llm/utils.ParseScoringJSON.
type Scorer struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

// NewScorer returns a Scorer with the given API key and model. Empty apiKey causes Score to error.
func NewScorer(apiKey, model string) *Scorer {
	return &Scorer{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: defaultBaseURL,
	}
}

func (s *Scorer) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return http.DefaultClient
}

func (s *Scorer) baseURL() string {
	if s.BaseURL != "" {
		return strings.TrimRight(s.BaseURL, "/")
	}
	return defaultBaseURL
}

// Score implements llm.Scorer.
func (s *Scorer) Score(ctx context.Context, profile string, job domain.Job) (domain.ScoredJob, error) {
	if strings.TrimSpace(s.APIKey) == "" {
		return domain.ScoredJob{}, fmt.Errorf("anthropic: empty API key")
	}
	if strings.TrimSpace(s.Model) == "" {
		return domain.ScoredJob{}, fmt.Errorf("anthropic: empty model")
	}
	body, err := json.Marshal(messagesRequest{
		Model:     s.Model,
		MaxTokens: defaultScoreMaxTokens,
		Messages: []messageItem{
			{Role: "user", Content: buildUserPrompt(profile, job)},
		},
	})
	if err != nil {
		return domain.ScoredJob{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL()+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return domain.ScoredJob{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.APIKey)
	req.Header.Set("anthropic-version", defaultAnthropicVer)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return domain.ScoredJob{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.ScoredJob{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ScoredJob{}, fmt.Errorf("anthropic: API %s: %s", resp.Status, truncateForErr(respBody))
	}
	text, err := extractAssistantText(respBody)
	if err != nil {
		return domain.ScoredJob{}, err
	}
	score, rationale, err := parseScoringFromAssistantText(text)
	if err != nil {
		return domain.ScoredJob{}, err
	}
	return domain.ScoredJob{Job: job, Score: score, Reason: rationale}, nil
}

func buildUserPrompt(profile string, job domain.Job) string {
	return fmt.Sprintf(`You are a job relevance scorer. Respond with ONLY a JSON object (no markdown fences, no text before or after) with exactly these keys: "score" (integer 0-100) and "rationale" (short string).

User profile:
%s

Job:
Title: %s
Company: %s
Description:
%s
`, profile, job.Title, job.Company, job.Description)
}

type messagesRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []messageItem `json:"messages"`
}

type messageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesResponse struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func extractAssistantText(respBody []byte) (string, error) {
	var mr messagesResponse
	if err := json.Unmarshal(respBody, &mr); err != nil {
		return "", fmt.Errorf("anthropic: decode response: %w", err)
	}
	var b strings.Builder
	for _, c := range mr.Content {
		if c.Type == "text" && c.Text != "" {
			b.WriteString(c.Text)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("anthropic: empty assistant text")
	}
	return out, nil
}

func parseScoringFromAssistantText(s string) (score int, rationale string, err error) {
	s = strings.TrimSpace(s)
	if sc, r, e := llmutils.ParseScoringJSON([]byte(s)); e == nil {
		return sc, r, nil
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return llmutils.ParseScoringJSON([]byte(s[start : end+1]))
	}
	return 0, "", fmt.Errorf("anthropic: scoring JSON not found in assistant output")
}

func truncateForErr(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
