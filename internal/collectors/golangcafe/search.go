package golangcafe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
)

const warmupOrigin = siteOrigin + "/"

// jobPost is one element from GET /api/jobPosts/search.
type jobPost struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Description string   `json:"description"`
	Link        string   `json:"link"`
	Date        int64    `json:"date"`
	Location    string   `json:"location"`
	Country     string   `json:"country"`
	Remote      string   `json:"remote"`
	SalaryFrom  salaryAmount `json:"salaryFrom"`
	SalaryTo    salaryAmount `json:"salaryTo"`
	Currency    string       `json:"currency"`
	Tags        []string     `json:"tags"`
}

// salaryAmount accepts a JSON number or string; junk / empty → unset (do not fail the catalog).
type salaryAmount struct {
	n *float64
}

func (s *salaryAmount) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		s.n = nil
		return nil
	}
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		s.n = &num
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		s.n = nil
		return nil
	}
	str = strings.TrimSpace(str)
	if str == "" {
		s.n = nil
		return nil
	}
	num, err := strconv.ParseFloat(str, 64)
	if err != nil {
		s.n = nil
		return nil
	}
	s.n = &num
	return nil
}

func (c *GolangCafe) fetchAllSearchPosts(ctx context.Context, query string) ([]jobPost, error) {
	if c == nil || c.HTMLDocumentFetcher == nil {
		return nil, fmt.Errorf("golang cafe: HTMLDocumentFetcher is nil")
	}
	if _, err := c.HTMLDocumentFetcher.FetchHTMLDocument(ctx, warmupOrigin); err != nil {
		return nil, fmt.Errorf("golang cafe: warmup: %w", err)
	}
	maxRounds := c.maxSearchRoundsEffective()
	var all []jobPost
	var beforeMs int64
	for round := 1; maxRounds == 0 || round <= maxRounds; round++ {
		batch, err := fetchSearchJSON(ctx, c.HTMLDocumentFetcher, query, beforeMs)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		oldest := batch[len(batch)-1].Date
		if oldest <= 0 || oldest == beforeMs {
			break
		}
		beforeMs = oldest
	}
	return all, nil
}

func fetchSearchJSON(ctx context.Context, fetcher browserfetch.HTMLDocumentFetcher, query string, beforeMs int64) ([]jobPost, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("golang cafe: fetcher is nil")
	}
	rawURL := SearchAPIURL(query, beforeMs)
	body, err := fetcher.FetchHTMLDocument(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("golang cafe: GET search: %w", err)
	}
	body = trimJSONBody(body)
	var posts []jobPost
	if err := json.Unmarshal(body, &posts); err != nil {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("golang cafe: decode search JSON: %w (body %q)", err, strings.TrimSpace(snippet))
	}
	return posts, nil
}

func trimJSONBody(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	// Rod may wrap JSON in a minimal HTML document on some responses.
	if strings.HasPrefix(s, "<") {
		if i := strings.Index(s, "["); i >= 0 {
			if j := strings.LastIndex(s, "]"); j > i {
				return []byte(s[i : j+1])
			}
		}
	}
	return []byte(s)
}
