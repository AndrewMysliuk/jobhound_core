package utils

import (
	"strings"

	jobschema "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// NormalizePlainText trims and collapses inner whitespace (specs/005 domain-mapping-mvp.md).
func NormalizePlainText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

// InferPosition applies MVP keyword groups to title + description + tags (domain-mapping-mvp.md).
func InferPosition(title, description string, tags []string) *string {
	b := strings.Builder{}
	b.WriteString(strings.ToLower(strings.TrimSpace(title)))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(strings.TrimSpace(description)))
	b.WriteByte(' ')
	for _, t := range tags {
		b.WriteString(strings.ToLower(strings.TrimSpace(t)))
		b.WriteByte(' ')
	}
	text := b.String()
	groups := []struct {
		label jobschema.PositionLabel
		keys  []string
	}{
		{jobschema.PositionLabelFullStack, []string{"full-stack", "full stack", "fullstack"}},
		{jobschema.PositionLabelFrontend, []string{"frontend", "front-end", "front end"}},
		{jobschema.PositionLabelBackend, []string{"backend", "back-end", "back end"}},
	}
	for _, g := range groups {
		for _, k := range g.keys {
			if strings.Contains(text, k) {
				s := g.label.String()
				return &s
			}
		}
	}
	return nil
}

// RemoteMVPRule sets Remote true when title, description, tags, or optional location
// hints (e.g. DOU listing cities + detail place line) contain English "remote",
// Ukrainian "віддалено", or the phrase "віддалена робота" (e.g. Djinni work-format line).
func RemoteMVPRule(title, description string, tags []string, locationHints ...string) *bool {
	b := strings.Builder{}
	b.WriteString(strings.ToLower(strings.TrimSpace(title)))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(strings.TrimSpace(description)))
	b.WriteByte(' ')
	for _, t := range tags {
		b.WriteString(strings.ToLower(strings.TrimSpace(t)))
		b.WriteByte(' ')
	}
	for _, h := range locationHints {
		b.WriteString(strings.ToLower(strings.TrimSpace(h)))
		b.WriteByte(' ')
	}
	text := b.String()
	v := strings.Contains(text, "remote") ||
		strings.Contains(text, "віддалено") ||
		strings.Contains(text, "віддалена робота")
	return &v
}
