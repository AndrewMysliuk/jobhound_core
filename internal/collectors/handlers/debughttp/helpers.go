package debughttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dou"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
)

const (
	defaultDebugLimit = 200
	maxDebugLimit     = 10000
	maxDebugBodyBytes = 512 << 10
)

func readDebugRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxDebugBodyBytes)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return b, nil
}

func parseCollectorsPOSTBody(b []byte) (schema.CollectorsPOSTBody, error) {
	var req schema.CollectorsPOSTBody
	if len(bytesTrimSpace(b)) == 0 {
		return req, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return schema.CollectorsPOSTBody{}, err
	}
	return req, nil
}

func resolveLimit(p *int) (int, error) {
	if p == nil {
		return defaultDebugLimit, nil
	}
	n := *p
	if n < 0 {
		return 0, fmt.Errorf("invalid limit (non-negative integer or 0 for unlimited)")
	}
	if n == 0 {
		return 0, nil
	}
	if n > maxDebugLimit {
		return 0, fmt.Errorf("limit exceeds max (%d)", maxDebugLimit)
	}
	return n, nil
}

func applyEuropeRemotelyOverrides(req *schema.CollectorsPOSTBody, c *europeremotely.EuropeRemotely) {
	c.FeedForm = europeremotely.CloneFeedForm(c.FeedForm)
	for k, v := range req.FeedForm {
		c.FeedForm.Set(k, v)
	}
	if req.SearchKeywords != nil {
		c.FeedForm.Set("search_keywords", *req.SearchKeywords)
	}
}

func applyDouOverrides(req *schema.CollectorsPOSTBody, c *dou.DOU) {
	if req.Search != nil && strings.TrimSpace(*req.Search) != "" {
		c.Search = strings.TrimSpace(*req.Search)
	}
	if req.DouInterRequestDelayMs != nil {
		c.InterRequestDelay = time.Duration(*req.DouInterRequestDelayMs) * time.Millisecond
	}
}

func applyWorkingNomadsOverrides(req *schema.CollectorsPOSTBody, c *workingnomads.WorkingNomads) {
	if req.Query != nil {
		c.Query = cloneRawMessage(*req.Query)
	}
	if req.Sort != nil {
		c.Sort = cloneRawMessage(*req.Sort)
	}
	if req.PageSize != nil && *req.PageSize > 0 {
		c.PageSize = *req.PageSize
	}
	if len(req.SourceFieldNames) > 0 {
		c.SourceFieldNames = append([]string(nil), req.SourceFieldNames...)
	}
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\t' || b[i] == '\r') {
		i++
	}
	return b[i:j]
}

func cloneRawMessage(m json.RawMessage) json.RawMessage {
	if len(m) == 0 {
		return nil
	}
	out := make([]byte, len(m))
	copy(out, m)
	return out
}
