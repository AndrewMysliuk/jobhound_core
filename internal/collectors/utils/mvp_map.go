package utils

import "strings"

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
		label string
		keys  []string
	}{
		{"full-stack", []string{"full-stack", "full stack", "fullstack"}},
		{"frontend", []string{"frontend", "front-end", "front end"}},
		{"backend", []string{"backend", "back-end", "back end"}},
	}
	for _, g := range groups {
		for _, k := range g.keys {
			if strings.Contains(text, k) {
				l := g.label
				return &l
			}
		}
	}
	return nil
}

// RemoteMVPRule sets Remote true when title, description, tags, or optional location
// hints (e.g. DOU listing cities + detail place line) contain English "remote" or
// Ukrainian "віддалено" (DOU.ua marks remote roles in the place span).
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
	v := strings.Contains(text, "remote") || strings.Contains(text, "віддалено")
	return &v
}
