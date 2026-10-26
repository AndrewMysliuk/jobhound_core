package impl

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestJobCustomID(t *testing.T) {
	t.Parallel()
	raw := "src\x1e" + strings.Repeat("a", 80)
	cid := JobCustomID(raw)
	if len(cid) != 64 {
		t.Fatalf("custom_id length = %d, want 64", len(cid))
	}
	for _, r := range cid {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !ok {
			t.Fatalf("custom_id has non-hex rune %q", r)
		}
	}
	if JobCustomID(raw) != cid {
		t.Fatal("custom_id not stable")
	}
	if JobCustomID(raw+"x") == cid {
		t.Fatal("different job ids must not collide")
	}
}

func TestBatchesClientCreateStatusResults(t *testing.T) {
	t.Parallel()

	const (
		apiKey  = "test-key"
		model   = "claude-test"
		batchID = "msgbatch_test1"
		jobA    = "board\x1ejob-a"
		jobB    = "board\x1ejob-b"
		textA   = `{"office":{"countries":[],"regions":[],"quote":null,"unknown":true}}`
	)
	cidA := JobCustomID(jobA)
	cidB := JobCustomID(jobB)

	var statusCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages/batches", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("create method = %s", r.Method)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("x-api-key"); got != apiKey {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicVersion {
			t.Errorf("anthropic-version = %q", got)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			http.Error(w, "read", http.StatusBadRequest)
			return
		}
		var body batchCreateBody
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "json", http.StatusBadRequest)
			return
		}
		if len(body.Requests) != 2 {
			t.Errorf("requests len = %d", len(body.Requests))
		}
		seen := map[string]bool{}
		for _, req := range body.Requests {
			seen[req.CustomID] = true
			if req.Params.Model != model {
				t.Errorf("model = %q", req.Params.Model)
			}
			if req.Params.MaxTokens != maxTokensModelReply {
				t.Errorf("max_tokens = %d", req.Params.MaxTokens)
			}
			if req.Params.System == "" {
				t.Error("system prompt empty")
			}
			if len(req.Params.Messages) != 1 || req.Params.Messages[0].Role != "user" {
				t.Errorf("messages = %+v", req.Params.Messages)
			}
			if !strings.Contains(req.Params.Messages[0].Content, vacancyDataBegin) {
				t.Error("user message missing vacancy delimiters")
			}
		}
		if !seen[cidA] || !seen[cidB] {
			t.Errorf("custom_ids = %v, want %s and %s", seen, cidA, cidB)
		}
		_ = json.NewEncoder(w).Encode(batchWire{
			ID:               batchID,
			ProcessingStatus: "in_progress",
		})
	})
	mux.HandleFunc("/v1/messages/batches/"+batchID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		n := statusCalls.Add(1)
		status := "in_progress"
		if n >= 2 {
			status = "ended"
		}
		_ = json.NewEncoder(w).Encode(batchWire{
			ID:               batchID,
			ProcessingStatus: status,
		})
	})
	mux.HandleFunc("/v1/messages/batches/"+batchID+"/results", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("x-api-key"); got != apiKey {
			t.Errorf("results x-api-key = %q", got)
		}
		lines := []string{
			mustJSON(t, batchResultLine{
				CustomID: cidA,
				Result: batchResultPayload{
					Type: "succeeded",
					Message: batchMessage{
						Content: []batchContentBlock{{Type: "text", Text: textA}},
					},
				},
			}),
			mustJSON(t, batchResultLine{
				CustomID: cidB,
				Result:   batchResultPayload{Type: "errored"},
			}),
		}
		_, _ = io.WriteString(w, strings.Join(lines, "\n")+"\n")
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &BatchesClient{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}

	ctx := context.Background()
	gotBatchID, customToJob, err := client.Create(ctx, []VacancyBatchItem{
		{JobID: jobA, Title: "A", Description: "da", LocationRaw: "la", SalaryRaw: "sa"},
		{JobID: jobB, Title: "B", Description: "db", LocationRaw: "lb", SalaryRaw: "sb"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotBatchID != batchID {
		t.Fatalf("batch id = %q", gotBatchID)
	}
	if customToJob[cidA] != jobA || customToJob[cidB] != jobB {
		t.Fatalf("customToJob = %#v", customToJob)
	}

	if err := client.WaitEnded(ctx, batchID, time.Millisecond); err != nil {
		t.Fatalf("WaitEnded: %v", err)
	}
	if statusCalls.Load() < 2 {
		t.Fatalf("status calls = %d, want >= 2", statusCalls.Load())
	}

	texts, err := client.Results(ctx, batchID, customToJob)
	if err != nil {
		t.Fatalf("Results: %v", err)
	}
	if texts[jobA] != textA {
		t.Fatalf("jobA text = %q", texts[jobA])
	}
	if _, ok := texts[jobB]; ok {
		t.Fatalf("errored jobB must write no row, got %q", texts[jobB])
	}
}

func TestFirstBatchChunkSplitsByBytes(t *testing.T) {
	t.Parallel()

	items := make([]VacancyBatchItem, 0, 3)
	for i := 0; i < 3; i++ {
		items = append(items, VacancyBatchItem{
			JobID:       "job-" + strings.Repeat("x", i+1),
			Title:       "T",
			Description: strings.Repeat("d", 200),
			LocationRaw: "L",
			SalaryRaw:   "S",
		})
	}
	system := PromptSystem()
	one, err := json.Marshal(wireRequest(items[0], system, "m", maxTokensModelReply))
	if err != nil {
		t.Fatal(err)
	}
	// Fit the first request only: envelope + one request, but not two (comma + second).
	maxBytes := batchCreateEnvelope + len(one) + 1
	first, rest, err := firstBatchChunk(items, "m", maxTokensModelReply, maxBytes, maxBatchRequests)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("first len = %d, want 1", len(first))
	}
	if len(rest) != 2 {
		t.Fatalf("rest len = %d, want 2", len(rest))
	}
	if first[0].JobID != items[0].JobID {
		t.Fatalf("first job = %q", first[0].JobID)
	}
}

func TestFirstBatchChunkSplitsByRequestCount(t *testing.T) {
	t.Parallel()

	items := []VacancyBatchItem{
		{JobID: "a", Title: "A", Description: "1"},
		{JobID: "b", Title: "B", Description: "2"},
		{JobID: "c", Title: "C", Description: "3"},
	}
	first, rest, err := firstBatchChunk(items, "m", maxTokensModelReply, maxBatchBodyBytes, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(rest) != 1 {
		t.Fatalf("first=%d rest=%d", len(first), len(rest))
	}
	if rest[0].JobID != "c" {
		t.Fatalf("rest job = %q", rest[0].JobID)
	}
}

func TestFirstBatchChunkKeepsOversizedSingleItem(t *testing.T) {
	t.Parallel()

	item := VacancyBatchItem{JobID: "big", Title: "T", Description: strings.Repeat("x", 500)}
	system := PromptSystem()
	raw, err := json.Marshal(wireRequest(item, system, "m", maxTokensModelReply))
	if err != nil {
		t.Fatal(err)
	}
	maxBytes := batchCreateEnvelope + len(raw) - 10 // smaller than the only item
	first, rest, err := firstBatchChunk([]VacancyBatchItem{item}, "m", maxTokensModelReply, maxBytes, maxBatchRequests)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(rest) != 0 {
		t.Fatalf("first=%d rest=%d, want single oversized item alone", len(first), len(rest))
	}
}

func TestBatchesClientResultsSkipsNonSucceeded(t *testing.T) {
	t.Parallel()

	jobOK := "ok\x1e1"
	jobErr := "err\x1e1"
	jobCan := "can\x1e1"
	jobExp := "exp\x1e1"
	customToJob := map[string]string{
		JobCustomID(jobOK):  jobOK,
		JobCustomID(jobErr): jobErr,
		JobCustomID(jobCan): jobCan,
		JobCustomID(jobExp): jobExp,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages/batches/b1/results", func(w http.ResponseWriter, r *http.Request) {
		lines := []string{
			mustJSON(t, batchResultLine{
				CustomID: JobCustomID(jobOK),
				Result: batchResultPayload{
					Type: "succeeded",
					Message: batchMessage{
						Content: []batchContentBlock{
							{Type: "text", Text: "part1"},
							{Type: "text", Text: "-part2"},
						},
					},
				},
			}),
			mustJSON(t, batchResultLine{CustomID: JobCustomID(jobErr), Result: batchResultPayload{Type: "errored"}}),
			mustJSON(t, batchResultLine{CustomID: JobCustomID(jobCan), Result: batchResultPayload{Type: "canceled"}}),
			mustJSON(t, batchResultLine{CustomID: JobCustomID(jobExp), Result: batchResultPayload{Type: "expired"}}),
		}
		_, _ = io.WriteString(w, strings.Join(lines, "\n")+"\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := &BatchesClient{BaseURL: srv.URL, HTTPClient: srv.Client(), APIKey: "k", Model: "m"}
	got, err := client.Results(context.Background(), "b1", customToJob)
	if err != nil {
		t.Fatalf("Results: %v", err)
	}
	if got[jobOK] != "part1-part2" {
		t.Fatalf("succeeded text = %q", got[jobOK])
	}
	if len(got) != 1 {
		t.Fatalf("got %#v, want only succeeded", got)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
