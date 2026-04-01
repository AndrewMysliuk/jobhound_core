package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
	pipeutils "github.com/andrewmysliuk/jobhound_core/internal/pipeline/utils"
)

// BroadFilterKeyParts is the semantic input for the canonical broad filter key (006 ingest-watermark-and-filter-key.md §2).
// Callers map workflow/search parameters into these fields before hashing.
type BroadFilterKeyParts struct {
	Role        string
	TimeFromUTC *time.Time
	TimeToUTC   *time.Time
	Sources     []string
	Keywords    []string
}

// BroadFilterKeyPartsFromRules derives key parts from stage-1 rules plus the ingest source set.
// Role synonyms, remote-only, and country allowlist are folded into keywords as stable tokens; time window uses UTC RFC3339 when both bounds are set.
func BroadFilterKeyPartsFromRules(rules pipeline.BroadFilterRules, sources []string) (BroadFilterKeyParts, error) {
	if err := pipeutils.ValidateBroadFilterRules(rules); err != nil {
		return BroadFilterKeyParts{}, err
	}
	syns := normalizeStringSlice(rules.RoleSynonyms)
	slices.Sort(syns)
	syns = slices.Compact(syns)
	role := strings.Join(syns, " ")

	var kws []string
	if rules.RemoteOnly {
		kws = append(kws, "__remote_only__")
	}
	for _, c := range rules.CountryAllowlist {
		t := strings.TrimSpace(strings.ToLower(c))
		if t != "" {
			kws = append(kws, "country:"+t)
		}
	}
	slices.Sort(kws)
	kws = slices.Compact(kws)

	srcs := normalizeStringSlice(sources)
	slices.Sort(srcs)
	srcs = slices.Compact(srcs)

	return BroadFilterKeyParts{
		Role:        role,
		TimeFromUTC: rules.From,
		TimeToUTC:   rules.To,
		Sources:     srcs,
		Keywords:    kws,
	}, nil
}

// CanonicalBroadFilterKeyJSON returns compact JSON with fixed key order: role, time_window, sources, keywords; empty sections omitted.
// String fields are trimmed and lowercased; sources and keywords are sorted and deduplicated (contract §2).
func CanonicalBroadFilterKeyJSON(p BroadFilterKeyParts) (string, error) {
	var parts []string
	if t := strings.TrimSpace(strings.ToLower(p.Role)); t != "" {
		b, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		parts = append(parts, `"role":`+string(b))
	}
	if p.TimeFromUTC != nil && p.TimeToUTC != nil {
		tw := struct {
			From string `json:"from"`
			To   string `json:"to"`
		}{
			From: p.TimeFromUTC.UTC().Format(time.RFC3339Nano),
			To:   p.TimeToUTC.UTC().Format(time.RFC3339Nano),
		}
		b, err := json.Marshal(tw)
		if err != nil {
			return "", err
		}
		parts = append(parts, `"time_window":`+string(b))
	}
	src := normalizeStringSlice(p.Sources)
	slices.Sort(src)
	src = slices.Compact(src)
	if len(src) > 0 {
		b, err := json.Marshal(src)
		if err != nil {
			return "", err
		}
		parts = append(parts, `"sources":`+string(b))
	}
	kws := normalizeStringSlice(p.Keywords)
	slices.Sort(kws)
	kws = slices.Compact(kws)
	if len(kws) > 0 {
		b, err := json.Marshal(kws)
		if err != nil {
			return "", err
		}
		parts = append(parts, `"keywords":`+string(b))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("ingest: broad filter key has no non-empty fields")
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

// BroadFilterKeyHashHex is SHA-256 of CanonicalBroadFilterKeyJSON, lowercase hex (64 chars).
func BroadFilterKeyHashHex(p BroadFilterKeyParts) (string, error) {
	s, err := CanonicalBroadFilterKeyJSON(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:]), nil
}

// BroadFilterKeyHashFromRules combines [BroadFilterKeyPartsFromRules] and [BroadFilterKeyHashHex].
func BroadFilterKeyHashFromRules(rules pipeline.BroadFilterRules, sources []string) (string, error) {
	parts, err := BroadFilterKeyPartsFromRules(rules, sources)
	if err != nil {
		return "", err
	}
	return BroadFilterKeyHashHex(parts)
}

func normalizeStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(s)
		if t == "" {
			continue
		}
		out = append(out, strings.ToLower(t))
	}
	return out
}
