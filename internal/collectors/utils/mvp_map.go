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

// RemoteMVPRule sets Remote true iff "remote" appears in title, plain description, or tags.
func RemoteMVPRule(title, description string, tags []string) *bool {
	b := strings.Builder{}
	b.WriteString(strings.ToLower(title))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(description))
	b.WriteByte(' ')
	for _, t := range tags {
		b.WriteString(strings.ToLower(t))
		b.WriteByte(' ')
	}
	v := strings.Contains(b.String(), "remote")
	return &v
}
