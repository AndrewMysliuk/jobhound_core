package impl

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
)

// UpsertFromAggregator is the discover path: blocklist check, then
// merge-or-insert. Never creates a second row for a known company.
// Key is fixed at birth and never promoted. An INACTIVE company sighted
// again on an aggregator wakes back to ACTIVE.
func (s *Service) UpsertFromAggregator(ctx context.Context, p schema.UpsertCompanyParams) (schema.UpsertCompanyResult, error) {
	if s == nil || s.companies == nil {
		return schema.UpsertCompanyResult{}, errors.New("companies: service not configured")
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return schema.UpsertCompanyResult{}, errors.New("companies: name is required")
	}
	nameKey := NormalizeName(name)
	domainKey := NormalizeDomain(p.Website)
	decision := BlocklistDecision{}
	if s.blocklist != nil {
		decision = s.blocklist.Check(nameKey, domainKey)
	}
	if decision.Blocked {
		return schema.UpsertCompanyResult{Blocked: true}, nil
	}

	birthKey := ChooseKey(name, p.Website)
	now := s.now()

	existing, otherForm, err := s.lookupExisting(ctx, birthKey, nameKey, domainKey, strings.TrimSpace(p.Website))
	if err != nil {
		return schema.UpsertCompanyResult{}, err
	}
	if existing.Key != "" {
		merged := mergeAggregatorSighting(existing, name, p.Website, p.ApplyURL, otherForm, decision.NeedsReview, now)
		if err := s.companies.Upsert(ctx, merged); err != nil {
			return schema.UpsertCompanyResult{}, err
		}
		if err := s.backfillJobCompanyKeyRef(ctx, merged); err != nil {
			return schema.UpsertCompanyResult{}, err
		}
		return schema.UpsertCompanyResult{Key: merged.Key, Merged: true}, nil
	}

	created := newPendingCompany(birthKey, name, p.Website, p.ApplyURL, decision.NeedsReview, now)
	// If the key is domain-form, keep the name-form in aliases so a later
	// name-only sighting merges instead of inserting a second row.
	if nameKey != "" && nameKey != birthKey {
		created.Aliases = appendAlias(created.Aliases, nameKey)
	}
	if err := s.companies.Upsert(ctx, created); err != nil {
		return schema.UpsertCompanyResult{}, err
	}
	if err := s.backfillJobCompanyKeyRef(ctx, created); err != nil {
		return schema.UpsertCompanyResult{}, err
	}
	return schema.UpsertCompanyResult{Key: created.Key, Created: true}, nil
}

// backfillJobCompanyKeyRef links jobs whose content company_key maps to c
// (key or alias). No-op when jobs store is unset.
func (s *Service) backfillJobCompanyKeyRef(ctx context.Context, c schema.Company) error {
	if s.jobs == nil || c.Key == "" {
		return nil
	}
	contentKeys := uniqueNonEmpty(append([]string{c.Key}, c.Aliases...)...)
	_, err := s.jobs.BackfillCompanyKeyRef(ctx, c.Key, contentKeys)
	return err
}

func (s *Service) lookupExisting(
	ctx context.Context,
	birthKey, nameKey, domainKey, website string,
) (schema.Company, string, error) {
	tryKeys := uniqueNonEmpty(birthKey, nameKey, domainKey)
	for _, k := range tryKeys {
		c, err := s.companies.GetByKey(ctx, k)
		if err == nil {
			other := ""
			if birthKey != c.Key {
				other = birthKey
			} else if nameKey != "" && nameKey != c.Key {
				other = nameKey
			}
			return c, other, nil
		}
		if !errors.Is(err, companies.ErrCompanyNotFound) {
			return schema.Company{}, "", err
		}
	}
	if website != "" {
		c, err := s.companies.FindByWebsite(ctx, website)
		if err == nil {
			other := birthKey
			if other == c.Key {
				other = nameKey
			}
			return c, other, nil
		}
		if !errors.Is(err, companies.ErrCompanyNotFound) {
			return schema.Company{}, "", err
		}
		if domainKey != "" && domainKey != website {
			c, err = s.companies.FindByWebsite(ctx, domainKey)
			if err == nil {
				return c, birthKey, nil
			}
			if !errors.Is(err, companies.ErrCompanyNotFound) {
				return schema.Company{}, "", err
			}
		}
	}
	for _, alias := range tryKeys {
		c, err := s.companies.FindByAlias(ctx, alias)
		if err == nil {
			return c, alias, nil
		}
		if !errors.Is(err, companies.ErrCompanyNotFound) {
			return schema.Company{}, "", err
		}
	}
	return schema.Company{}, "", nil
}

func newPendingCompany(key, name, website, applyURL string, needsReview bool, now time.Time) schema.Company {
	c := schema.Company{
		Key:              key,
		Name:             name,
		Website:          strings.TrimSpace(website),
		ResolutionStatus: schema.ResolutionStatusPending,
		PollState:        schema.PollStateActive,
		FirstSeenAt:      now,
		LastSeenAt:       now,
		NeedsReview:      needsReview,
	}
	// Stash apply URL as BoardURL until Resolve overwrites it with the board page.
	if applyURL = strings.TrimSpace(applyURL); applyURL != "" {
		c.BoardURL = applyURL
	}
	return c
}

func mergeAggregatorSighting(
	existing schema.Company,
	name, website, applyURL, otherForm string,
	needsReview bool,
	now time.Time,
) schema.Company {
	out := existing
	out.LastSeenAt = now
	if name != "" {
		out.Name = name
	}
	if out.Website == "" && strings.TrimSpace(website) != "" {
		out.Website = strings.TrimSpace(website)
	}
	if otherForm != "" && otherForm != out.Key {
		out.Aliases = appendAlias(out.Aliases, otherForm)
	}
	if needsReview {
		out.NeedsReview = true
	}
	// Wake INACTIVE companies when the aggregator still shows them.
	if out.PollState == schema.PollStateInactive {
		out.PollState = schema.PollStateActive
		out.EmptyPollStreak = 0
	}
	// Keep a resolve hint while still PENDING/UNRESOLVED and BoardURL empty.
	if out.BoardURL == "" && strings.TrimSpace(applyURL) != "" {
		switch out.ResolutionStatus {
		case schema.ResolutionStatusPending, schema.ResolutionStatusUnresolved, "":
			out.BoardURL = strings.TrimSpace(applyURL)
		}
	}
	return out
}

func appendAlias(aliases []string, form string) []string {
	form = strings.TrimSpace(form)
	if form == "" {
		return aliases
	}
	for _, a := range aliases {
		if a == form {
			return aliases
		}
	}
	return append(append([]string{}, aliases...), form)
}

func uniqueNonEmpty(vals ...string) []string {
	seen := make(map[string]struct{}, len(vals))
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
