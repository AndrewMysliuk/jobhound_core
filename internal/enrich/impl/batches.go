package impl

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBatchesBaseURL is the Anthropic API host for Message Batches.
	DefaultBatchesBaseURL = "https://api.anthropic.com"
	anthropicVersion      = "2023-06-01"
	// maxTokensModelReply fits one ModelReply JSON object with headroom.
	maxTokensModelReply = 4096
	defaultHTTPTimeout  = 60 * time.Second
)

// VacancyBatchItem is one vacancy to include in a Message Batch request.
type VacancyBatchItem struct {
	JobID       string
	Title       string
	Description string
	LocationRaw string
	SalaryRaw   string
}

// BatchesClient talks to Anthropic Message Batches. It does not call the network
// when HTTPClient and BaseURL are pointed at a test server.
type BatchesClient struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
	MaxTokens  int
}

// JobCustomID returns the hex SHA-256 of jobID. Anthropic custom_id allows only
// ^[A-Za-z0-9_-]{1,64}$; raw job ids contain 0x1e and can exceed 64 characters.
func JobCustomID(jobID string) string {
	sum := sha256.Sum256([]byte(jobID))
	return hex.EncodeToString(sum[:])
}

// Create submits POST /v1/messages/batches. It returns the batch id and a map
// from custom_id back to job id. No tools and no streaming.
func (c *BatchesClient) Create(ctx context.Context, items []VacancyBatchItem) (batchID string, customToJob map[string]string, err error) {
	if len(items) == 0 {
		return "", nil, fmt.Errorf("batches create: empty requests")
	}
	customToJob = make(map[string]string, len(items))
	reqs := make([]batchWireRequest, 0, len(items))
	system := PromptSystem()
	for _, item := range items {
		cid := JobCustomID(item.JobID)
		if prev, ok := customToJob[cid]; ok && prev != item.JobID {
			return "", nil, fmt.Errorf("batches create: custom_id collision for job %q and %q", prev, item.JobID)
		}
		customToJob[cid] = item.JobID
		reqs = append(reqs, batchWireRequest{
			CustomID: cid,
			Params: batchWireParams{
				Model:     c.model(),
				MaxTokens: c.maxTokens(),
				System:    system,
				Messages: []batchWireMessage{{
					Role:    "user",
					Content: PromptUser(item.Title, item.Description, item.LocationRaw, item.SalaryRaw),
				}},
			},
		})
	}
	body, err := json.Marshal(batchCreateBody{Requests: reqs})
	if err != nil {
		return "", nil, fmt.Errorf("batches create encode: %w", err)
	}
	var resp batchWire
	if err := c.doJSON(ctx, http.MethodPost, "/v1/messages/batches", bytes.NewReader(body), &resp); err != nil {
		return "", nil, err
	}
	if resp.ID == "" {
		return "", nil, fmt.Errorf("batches create: empty batch id")
	}
	return resp.ID, customToJob, nil
}

// Status returns processing_status from GET /v1/messages/batches/{id}.
func (c *BatchesClient) Status(ctx context.Context, batchID string) (string, error) {
	if batchID == "" {
		return "", fmt.Errorf("batches status: empty batch id")
	}
	var resp batchWire
	path := "/v1/messages/batches/" + batchID
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return "", err
	}
	if resp.ProcessingStatus == "" {
		return "", fmt.Errorf("batches status: empty processing_status")
	}
	return resp.ProcessingStatus, nil
}

// WaitEnded polls Status until processing_status is "ended" or ctx is done.
func (c *BatchesClient) WaitEnded(ctx context.Context, batchID string, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	for {
		status, err := c.Status(ctx, batchID)
		if err != nil {
			return err
		}
		if status == "ended" {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Results reads GET /v1/messages/batches/{id}/results JSONL. Only result.type
// "succeeded" entries are returned as job id → model text. errored, canceled,
// and expired are omitted (no row to write).
func (c *BatchesClient) Results(ctx context.Context, batchID string, customToJob map[string]string) (map[string]string, error) {
	if batchID == "" {
		return nil, fmt.Errorf("batches results: empty batch id")
	}
	path := "/v1/messages/batches/" + batchID + "/results"
	body, err := c.doRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	sc := bufio.NewScanner(bytes.NewReader(body))
	// Model replies are small; raise the default 64KiB scan limit for safety.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row batchResultLine
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("batches results line %d: %w", lineNo, err)
		}
		jobID, ok := customToJob[row.CustomID]
		if !ok {
			return nil, fmt.Errorf("batches results line %d: unknown custom_id %q", lineNo, row.CustomID)
		}
		if row.Result.Type != "succeeded" {
			continue
		}
		text := messageText(row.Result.Message.Content)
		if text == "" {
			continue
		}
		out[jobID] = text
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("batches results scan: %w", err)
	}
	return out, nil
}

func (c *BatchesClient) model() string {
	return c.Model
}

func (c *BatchesClient) maxTokens() int {
	if c.MaxTokens > 0 {
		return c.MaxTokens
	}
	return maxTokensModelReply
}

func (c *BatchesClient) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return DefaultBatchesBaseURL
}

func (c *BatchesClient) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func (c *BatchesClient) doJSON(ctx context.Context, method, path string, body io.Reader, dest any) error {
	raw, err := c.doRaw(ctx, method, path, body)
	if err != nil {
		return err
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("batches %s %s decode: %w", method, path, err)
	}
	return nil
}

func (c *BatchesClient) doRaw(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	url := c.base() + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("batches %s %s: %w", method, path, err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	res, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("batches %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("batches %s %s read: %w", method, path, err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("batches %s %s: status %d: %s", method, path, res.StatusCode, truncate(string(raw), 512))
	}
	return raw, nil
}

func messageText(blocks []batchContentBlock) string {
	var b strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

type batchCreateBody struct {
	Requests []batchWireRequest `json:"requests"`
}

type batchWireRequest struct {
	CustomID string          `json:"custom_id"`
	Params   batchWireParams `json:"params"`
}

type batchWireParams struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []batchWireMessage `json:"messages"`
}

type batchWireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type batchWire struct {
	ID               string `json:"id"`
	ProcessingStatus string `json:"processing_status"`
}

type batchResultLine struct {
	CustomID string             `json:"custom_id"`
	Result   batchResultPayload `json:"result"`
}

type batchResultPayload struct {
	Type    string       `json:"type"`
	Message batchMessage `json:"message"`
}

type batchMessage struct {
	Content []batchContentBlock `json:"content"`
}

type batchContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
