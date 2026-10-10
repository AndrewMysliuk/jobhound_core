package storage

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// textArray is jobs.sources: Postgres TEXT[], scanned from a text literal.
// SQLite tests store that same literal, or a JSON array written by hand.
type textArray []string

func (a textArray) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	lit := formatTextArray([]string(a))
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		return clause.Expr{SQL: "?::text[]", Vars: []any{lit}}
	}
	return clause.Expr{SQL: "?", Vars: []any{lit}}
}

func (a textArray) Value() (driver.Value, error) {
	return formatTextArray([]string(a)), nil
}

func (a *textArray) Scan(src any) error {
	if a == nil {
		return fmt.Errorf("sources: scan on nil textArray")
	}
	switch v := src.(type) {
	case nil:
		*a = nil
		return nil
	case string:
		parsed, err := ParseSourceIDs(v)
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	case []byte:
		parsed, err := ParseSourceIDs(string(v))
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	default:
		return fmt.Errorf("sources: unsupported type %T", src)
	}
}

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

func formatTextArray(ids []string) string {
	if len(ids) == 0 {
		return "{}"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = `"` + strings.ReplaceAll(strings.ReplaceAll(id, `\`, `\\`), `"`, `\"`) + `"`
	}
	return "{" + strings.Join(parts, ",") + "}"
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
