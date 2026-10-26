package impl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"gopkg.in/yaml.v3"
)

// BlocklistDecision is the discover-path verdict for one name/key.
type BlocklistDecision struct {
	Blocked     bool // exact or anonymous hit — do not create a company row
	NeedsReview bool // review substring hit — create/merge but flag
}

// BlocklistMatcher matches normalized keys against blocklist.yaml.
// Exact/anonymous: equality on the normalized key only. Review: case-insensitive
// substring. No fuzzy matching.
type BlocklistMatcher struct {
	exact     map[string]struct{}
	anonymous map[string]struct{}
	review    []string
}

// LoadBlocklist reads blocklist.yaml from dir (JOBHOUND_COMPANIES_DIR).
func LoadBlocklist(dir string) (*BlocklistMatcher, error) {
	path := filepath.Join(dir, "blocklist.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read blocklist: %w", err)
	}
	var bl schema.Blocklist
	if err := yaml.Unmarshal(raw, &bl); err != nil {
		return nil, fmt.Errorf("parse blocklist: %w", err)
	}
	return NewBlocklistMatcher(bl), nil
}

// NewBlocklistMatcher builds a matcher from an already-parsed Blocklist.
// Exact and anonymous entries are normalized the same way as company keys.
func NewBlocklistMatcher(bl schema.Blocklist) *BlocklistMatcher {
	m := &BlocklistMatcher{
		exact:     make(map[string]struct{}, len(bl.Exact)),
		anonymous: make(map[string]struct{}, len(bl.Anonymous)),
		review:    make([]string, 0, len(bl.Review)),
	}
	for _, e := range bl.Exact {
		k := NormalizeName(e)
		if k == "" {
			k = NormalizeDomain(e)
		}
		if k != "" {
			m.exact[k] = struct{}{}
		}
	}
	for _, e := range bl.Anonymous {
		k := NormalizeName(e)
		if k != "" {
			m.anonymous[k] = struct{}{}
		}
	}
	for _, e := range bl.Review {
		p := strings.ToLower(strings.TrimSpace(e))
		if p != "" {
			m.review = append(m.review, p)
		}
	}
	return m
}

// Check reports whether the normalized key is blocked or needs review.
// nameKey should already be NormalizeName(displayName); domainKey is optional.
func (m *BlocklistMatcher) Check(nameKey, domainKey string) BlocklistDecision {
	if m == nil {
		return BlocklistDecision{}
	}
	if m.isBlocked(nameKey) || m.isBlocked(domainKey) {
		return BlocklistDecision{Blocked: true}
	}
	return BlocklistDecision{NeedsReview: m.needsReview(nameKey)}
}

func (m *BlocklistMatcher) isBlocked(key string) bool {
	if key == "" {
		return false
	}
	if _, ok := m.exact[key]; ok {
		return true
	}
	_, ok := m.anonymous[key]
	return ok
}

func (m *BlocklistMatcher) needsReview(nameKey string) bool {
	if nameKey == "" {
		return false
	}
	lower := strings.ToLower(nameKey)
	for _, p := range m.review {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
