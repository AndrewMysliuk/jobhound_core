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
	"sync"

	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	llmschema "github.com/andrewmysliuk/jobhound_core/internal/llm/schema"
	llmutils "github.com/andrewmysliuk/jobhound_core/internal/llm/utils"
)

const (
	defaultBaseURL        = "https://api.anthropic.com"
	defaultAnthropicVer   = "2023-06-01"
	defaultScoreMaxTokens = 256
)

var (
	scoringSchemaAPIJSON json.RawMessage
	scoringSchemaAPIErr  error
	scoringSchemaAPIOnce sync.Once
)

// scoringSchemaForAPI returns the JSON Schema object for Anthropic output_config (no $schema key; some APIs reject it).
func scoringSchemaForAPI() (json.RawMessage, error) {
	scoringSchemaAPIOnce.Do(func() {
		var m map[string]any
		if err := json.Unmarshal(llmschema.JobScoringJSON, &m); err != nil {
			scoringSchemaAPIErr = err
			return
		}
		delete(m, "$schema")
		b, err := json.Marshal(m)
		if err != nil {
			scoringSchemaAPIErr = err
			return
		}
		scoringSchemaAPIJSON = b
	})
	return scoringSchemaAPIJSON, scoringSchemaAPIErr
}

// Scorer calls Claude with structured JSON output (output_config) and maps to schema.ScoredJob.
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
func (s *Scorer) Score(ctx context.Context, profile string, job schema.Job) (schema.ScoredJob, error) {
	if strings.TrimSpace(s.APIKey) == "" {
		return schema.ScoredJob{}, fmt.Errorf("anthropic: empty API key")
	}
	if strings.TrimSpace(s.Model) == "" {
		return schema.ScoredJob{}, fmt.Errorf("anthropic: empty model")
	}
	apiSchema, err := scoringSchemaForAPI()
	if err != nil {
		return schema.ScoredJob{}, fmt.Errorf("anthropic: scoring schema: %w", err)
	}
	body, err := json.Marshal(messagesRequest{
		Model:     s.Model,
		MaxTokens: defaultScoreMaxTokens,
		Messages: []messageItem{
			{Role: "user", Content: buildUserPrompt(profile, job)},
		},
		OutputConfig: &outputConfigParam{
			Format: jsonSchemaOutputFormatParam{
				Type:   "json_schema",
				Schema: apiSchema,
			},
		},
	})
	if err != nil {
		return schema.ScoredJob{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL()+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return schema.ScoredJob{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.APIKey)
	req.Header.Set("anthropic-version", defaultAnthropicVer)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return schema.ScoredJob{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return schema.ScoredJob{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return schema.ScoredJob{}, fmt.Errorf("anthropic: API %s: %s", resp.Status, truncateForErr(respBody))
	}
	text, err := extractAssistantText(respBody)
	if err != nil {
		return schema.ScoredJob{}, err
	}
	if err := llmutils.ValidateJSONDocument(llmschema.JobScoringJSON, []byte(text)); err != nil {
		return schema.ScoredJob{}, fmt.Errorf("anthropic: scoring output: %w", err)
	}
	score, rationale, err := llmutils.ParseScoringJSON([]byte(text))
	if err != nil {
		return schema.ScoredJob{}, err
	}
	return schema.ScoredJob{Job: job, Score: score, Reason: rationale}, nil
}

func buildUserPrompt(profile string, job schema.Job) string {
	return fmt.Sprintf(`You are a job relevance scorer. Return a JSON object matching the requested schema only.

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
	Model        string             `json:"model"`
	MaxTokens    int                `json:"max_tokens"`
	Messages     []messageItem      `json:"messages"`
	OutputConfig *outputConfigParam `json:"output_config,omitempty"`
}

type outputConfigParam struct {
	Format jsonSchemaOutputFormatParam `json:"format"`
}

type jsonSchemaOutputFormatParam struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
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

func truncateForErr(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
