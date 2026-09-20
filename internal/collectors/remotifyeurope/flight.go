package remotifyeurope

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// flightJobStub is a listing card from a Flight POST (description refs like "$8" are ignored).
type flightJobStub struct {
	ID          string `json:"id"`
	JobTitle    string `json:"job_title"`
	CompanyName string `json:"company_name"`
	CreatedAt   string `json:"created_at"`
}

var flightArrayRE = regexp.MustCompile(`1:\[`)

// ParseFlightListing extracts job stubs from a Next.js Flight response body.
func ParseFlightListing(body []byte) ([]flightJobStub, error) {
	text := string(body)
	loc := flightArrayRE.FindStringIndex(text)
	if loc == nil {
		return nil, fmt.Errorf("remotify europe: flight jobs array not found")
	}
	start := loc[0] + len("1:")
	end, err := matchingJSONArrayEnd(text, start)
	if err != nil {
		return nil, err
	}
	raw := text[start:end]
	var rows []flightJobStub
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, fmt.Errorf("remotify europe: flight decode: %w", err)
	}
	out := make([]flightJobStub, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			continue
		}
		row.ID = id
		out = append(out, row)
	}
	return out, nil
}

func matchingJSONArrayEnd(s string, start int) (int, error) {
	if start >= len(s) || s[start] != '[' {
		return 0, fmt.Errorf("remotify europe: flight array start")
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			switch c {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("remotify europe: unterminated flight array")
}
