package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseSourceIDs reads a Postgres text-array literal or a JSON string array.
func ParseSourceIDs(raw string) ([]string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "{}" || s == "[]" || s == "null" {
		return nil, nil
	}
	if strings.HasPrefix(s, "[") {
		var out []string
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			return nil, fmt.Errorf("sources: %w", err)
		}
		return out, nil
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return parsePGArray(s)
	}
	return nil, fmt.Errorf("sources: %q", s)
}

func parsePGArray(s string) ([]string, error) {
	body := s[1 : len(s)-1]
	if strings.TrimSpace(body) == "" {
		return nil, nil
	}
	var out []string
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if inQuote {
			if c == '\\' && i+1 < len(body) {
				i++
				b.WriteByte(body[i])
				continue
			}
			if c == '"' {
				inQuote = false
				continue
			}
			b.WriteByte(c)
			continue
		}
		switch c {
		case '"':
			inQuote = true
		case ',':
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("sources: unclosed quote in %q", s)
	}
	out = append(out, b.String())
	return out, nil
}
