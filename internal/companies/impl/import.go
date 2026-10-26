package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
)

type seedATSClass int

const (
	seedATSNone seedATSClass = iota
	seedATSPollable
	seedATSUnsupported
)

// ImportFromSeed reads path (companies_template.xlsx) and upserts companies with
// board verify for pollable ATS rows. Idempotent: re-running merges.
func (s *Service) ImportFromSeed(ctx context.Context, path string) (schema.ImportStats, error) {
	rows, err := ReadSeedXLSX(path)
	if err != nil {
		return schema.ImportStats{}, err
	}
	return s.ImportSeedRows(ctx, rows)
}

// ImportSeedRows applies the seed import rules to already-parsed rows.
func (s *Service) ImportSeedRows(ctx context.Context, rows []schema.SeedRow) (schema.ImportStats, error) {
	if s == nil || s.companies == nil {
		return schema.ImportStats{}, errors.New("companies: service not configured")
	}
	var stats schema.ImportStats
	for _, row := range rows {
		stats.Rows++
		if err := s.importOneSeedRow(ctx, row, &stats); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func (s *Service) importOneSeedRow(ctx context.Context, row schema.SeedRow, stats *schema.ImportStats) error {
	name := strings.TrimSpace(row.Name)
	if name == "" || isSeedExampleRow(name) || isSeedSkipFit(row.Fit) {
		stats.Skipped++
		return nil
	}

	nameKey := NormalizeName(name)
	domainKey := NormalizeDomain(row.Website)
	decision := BlocklistDecision{}
	if s.blocklist != nil {
		decision = s.blocklist.Check(nameKey, domainKey)
	}
	if decision.Blocked {
		stats.Blocked++
		return nil
	}

	birthKey := ChooseKey(name, row.Website)
	now := s.now()
	existing, otherForm, err := s.lookupExisting(ctx, birthKey, nameKey, domainKey, strings.TrimSpace(row.Website))
	if err != nil {
		return err
	}

	c := schema.Company{
		Key:              birthKey,
		Name:             name,
		Website:          strings.TrimSpace(row.Website),
		ResolutionStatus: schema.ResolutionStatusPending,
		PollState:        schema.PollStateActive,
		FirstSeenAt:      now,
		LastSeenAt:       now,
		NeedsReview:      decision.NeedsReview,
	}
	created := true
	if existing.Key != "" {
		created = false
		c = existing
		c.Name = name
		c.LastSeenAt = now
		if c.Website == "" && strings.TrimSpace(row.Website) != "" {
			c.Website = strings.TrimSpace(row.Website)
		}
		if otherForm != "" && otherForm != c.Key {
			c.Aliases = appendAlias(c.Aliases, otherForm)
		}
		if decision.NeedsReview {
			c.NeedsReview = true
		}
		if c.PollState == schema.PollStateInactive {
			c.PollState = schema.PollStateActive
			c.EmptyPollStreak = 0
		}
	} else if nameKey != "" && nameKey != birthKey {
		c.Aliases = appendAlias(c.Aliases, nameKey)
	}

	ats, class := classifySeedATS(row.ATSRaw)
	slug := strings.TrimSpace(row.Slug)
	boardHint := strings.TrimSpace(row.VacanciesURL)

	switch class {
	case seedATSUnsupported:
		c.ATS = ats
		c.ATSSlug = ""
		c.BoardURL = boardHint
		c.ResolutionStatus = schema.ResolutionStatusUnsupportedATS
		c.ResolutionMethod = schema.ResolutionMethodSeed
		c.NextResolveAt = nil
		stats.Unsupported++
	case seedATSPollable:
		if slug == "" {
			c.ATS = ""
			c.ATSSlug = ""
			c.BoardURL = boardHint
			c.ResolutionStatus = schema.ResolutionStatusPending
			c.ResolutionMethod = ""
			c.NextResolveAt = nil
			stats.Pending++
			break
		}
		outcome, boardURL, err := s.verifySeedBoard(ctx, ats, slug, name, boardHint)
		if err != nil {
			return fmt.Errorf("seed verify %s/%s: %w", ats, slug, err)
		}
		switch outcome {
		case schema.PollOutcomeComplete, schema.PollOutcomeEmpty, schema.PollOutcomePartial, schema.PollOutcomeNotModified:
			// 200 (incl. empty list) → RESOLVED; the slug is verified.
			c.ATS = ats
			c.ATSSlug = slug
			c.BoardURL = boardURL
			c.ResolutionStatus = schema.ResolutionStatusResolved
			c.ResolutionMethod = schema.ResolutionMethodSeed
			c.NextResolveAt = nil
			stats.Resolved++
		case schema.PollOutcomeDeadSlug:
			// 404 → PENDING, slug discarded.
			c.ATS = ""
			c.ATSSlug = ""
			c.BoardURL = boardHint
			c.ResolutionStatus = schema.ResolutionStatusPending
			c.ResolutionMethod = ""
			c.NextResolveAt = nil
			stats.Pending++
		default:
			c.ATS = ""
			c.ATSSlug = ""
			c.BoardURL = boardHint
			c.ResolutionStatus = schema.ResolutionStatusPending
			c.ResolutionMethod = ""
			c.NextResolveAt = nil
			stats.Pending++
		}
	default:
		// Custom / UNKNOWN / NONE → PENDING, no slug.
		c.ATS = ""
		c.ATSSlug = ""
		c.BoardURL = boardHint
		c.ResolutionStatus = schema.ResolutionStatusPending
		c.ResolutionMethod = ""
		c.NextResolveAt = nil
		stats.Pending++
	}

	if err := s.companies.Upsert(ctx, c); err != nil {
		return err
	}
	if created {
		stats.Created++
	} else {
		stats.Merged++
	}
	return nil
}

func (s *Service) verifySeedBoard(ctx context.Context, ats schema.ATS, slug, companyName, boardHint string) (schema.PollOutcome, string, error) {
	fetcher, ok := s.fetchers[ats]
	if !ok || fetcher == nil {
		return "", "", fmt.Errorf("no fetcher for %s", ats)
	}
	res, err := fetcher.FetchBoard(ctx, slug, companyName, "")
	if err != nil {
		return "", "", err
	}
	boardURL := boardHint
	if boardURL == "" {
		boardURL = seedBoardURL(ats, slug)
	}
	return res.Outcome, boardURL, nil
}

func seedBoardURL(ats schema.ATS, slug string) string {
	switch ats {
	case schema.ATSGreenhouse:
		return fmt.Sprintf("https://boards.greenhouse.io/%s", slug)
	case schema.ATSLever:
		return fmt.Sprintf("https://jobs.lever.co/%s", slug)
	case schema.ATSAshby:
		return fmt.Sprintf("https://jobs.ashbyhq.com/%s", slug)
	default:
		return ""
	}
}

func isSeedSkipFit(fit string) bool {
	return strings.EqualFold(strings.TrimSpace(fit), "SKIP")
}

func isSeedExampleRow(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	if strings.Contains(n, "ПРИМЕР") {
		return true
	}
	upper := strings.ToUpper(n)
	return strings.Contains(upper, "EXAMPLE") || strings.Contains(upper, "(ПРИМЕР)")
}

// classifySeedATS maps the sheet ATS-тип cell. Parenthetical notes are stripped
// ("Greenhouse (встроен в сайт)" → Greenhouse). Custom/UNKNOWN/NONE/Notion → none.
func classifySeedATS(raw string) (schema.ATS, seedATSClass) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", seedATSNone
	}
	if i := strings.Index(raw, "("); i >= 0 {
		raw = strings.TrimSpace(raw[:i])
	}
	key := strings.ToLower(raw)
	// Drop trailing punctuation / spaces left after stripping notes.
	key = strings.TrimFunc(key, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_'
	})
	switch key {
	case "greenhouse":
		return schema.ATSGreenhouse, seedATSPollable
	case "lever":
		return schema.ATSLever, seedATSPollable
	case "ashby":
		return schema.ATSAshby, seedATSPollable
	case "workday":
		return schema.ATSWorkday, seedATSUnsupported
	case "teamtailor":
		return schema.ATSTeamtailor, seedATSUnsupported
	case "workable":
		return schema.ATSWorkable, seedATSUnsupported
	case "recruitee":
		return schema.ATSRecruitee, seedATSUnsupported
	case "personio":
		return schema.ATSPersonio, seedATSUnsupported
	case "rippling":
		return schema.ATSRippling, seedATSUnsupported
	case "smartrecruiters":
		return schema.ATSSmartRecruiters, seedATSUnsupported
	case "kula":
		return schema.ATSOther, seedATSUnsupported
	case "custom", "unknown", "none", "notion":
		return "", seedATSNone
	default:
		if key == "" {
			return "", seedATSNone
		}
		return schema.ATSOther, seedATSUnsupported
	}
}
