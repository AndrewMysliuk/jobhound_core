package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ISO aliases and common shorthands (specs/005-job-collectors/contracts/domain-mapping-mvp.md).
var countryAliases = map[string]string{
	"uk":              "GB",
	"u.k.":            "GB",
	"great britain":   "GB",
	"britain":         "GB",
	"usa":             "US",
	"u.s.a.":          "US",
	"u.s.":            "US",
	"united states":   "US",
	"america":         "US",
	"the netherlands": "NL",
	"holland":         "NL",
	"czechia":         "CZ",
	"czech republic":  "CZ",
}

type countryRecord struct {
	Alpha2 string `json:"alpha_2"`
	Name   string `json:"name"`
}

// CountryResolver maps lowercased country names from data/countries.json to ISO 3166-1 alpha-2 codes.
type CountryResolver struct {
	byLowerName map[string]string
}

// LoadCountryResolver reads the full countries.json array (see repo data/countries.json).
func LoadCountryResolver(r io.Reader) (*CountryResolver, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var records []countryRecord
	if err := json.Unmarshal(b, &records); err != nil {
		return nil, fmt.Errorf("decode countries json: %w", err)
	}
	m := make(map[string]string, len(records)+len(countryAliases))
	for _, rec := range records {
		key := strings.ToLower(strings.TrimSpace(rec.Name))
		if key == "" || rec.Alpha2 == "" {
			continue
		}
		m[key] = strings.ToUpper(rec.Alpha2)
	}
	for k, v := range countryAliases {
		m[k] = v
	}
	return &CountryResolver{byLowerName: m}, nil
}

// Alpha2ForName returns the ISO alpha-2 code for a single location fragment (trimmed), or "" if unknown.
// Pass one token at a time (e.g. after splitting "Germany, Remote" on commas).
func (r *CountryResolver) Alpha2ForName(fragment string) string {
	if r == nil {
		return ""
	}
	key := strings.ToLower(strings.TrimSpace(fragment))
	if key == "" {
		return ""
	}
	// Ignore common non-country tokens from job boards.
	switch key {
	case "remote", "worldwide", "anywhere", "global", "flexible", "hybrid":
		return ""
	}
	if code, ok := r.byLowerName[key]; ok {
		return code
	}
	return ""
}
