package storage

import (
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
)

// Company is the GORM model for the companies table.
type Company struct {
	Key              string     `gorm:"column:key;primaryKey;type:text"`
	Name             string     `gorm:"column:name;type:text;not null"`
	Aliases          textArray  `gorm:"column:aliases;type:text[];not null;default:'{}'"`
	Website          string     `gorm:"column:website;type:text;not null;default:''"`
	ATS              string     `gorm:"column:ats;type:text;not null;default:''"`
	ATSSlug          string     `gorm:"column:ats_slug;type:text;not null;default:''"`
	BoardURL         string     `gorm:"column:board_url;type:text;not null;default:''"`
	BoardETag        string     `gorm:"column:board_etag;type:text;not null;default:''"`
	ResolutionStatus string     `gorm:"column:resolution_status;type:text;not null"`
	ResolutionMethod string     `gorm:"column:resolution_method;type:text;not null;default:''"`
	PollState        string     `gorm:"column:poll_state;type:text;not null;default:'ACTIVE'"`
	NextResolveAt    *time.Time `gorm:"column:next_resolve_at"`
	FirstSeenAt      time.Time  `gorm:"column:first_seen_at;not null"`
	LastSeenAt       time.Time  `gorm:"column:last_seen_at;not null"`
	LastPolledAt     *time.Time `gorm:"column:last_polled_at"`
	PollFailCount    int        `gorm:"column:poll_fail_count;not null;default:0"`
	EmptyPollStreak  int        `gorm:"column:empty_poll_streak;not null;default:0"`
	NeedsReview      bool       `gorm:"column:needs_review;not null;default:false"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null"`
}

// TableName implements schema.Tabler.
func (Company) TableName() string { return "companies" }

func newCompany(c schema.Company) Company {
	row := Company{
		Key:              c.Key,
		Name:             c.Name,
		Aliases:          textArray(c.Aliases),
		Website:          c.Website,
		ATS:              string(c.ATS),
		ATSSlug:          c.ATSSlug,
		BoardURL:         c.BoardURL,
		BoardETag:        c.BoardETag,
		ResolutionStatus: string(c.ResolutionStatus),
		ResolutionMethod: string(c.ResolutionMethod),
		PollState:        string(c.PollState),
		NextResolveAt:    c.NextResolveAt,
		FirstSeenAt:      c.FirstSeenAt,
		LastSeenAt:       c.LastSeenAt,
		LastPolledAt:     c.LastPolledAt,
		PollFailCount:    c.PollFailCount,
		EmptyPollStreak:  c.EmptyPollStreak,
		NeedsReview:      c.NeedsReview,
	}
	if row.PollState == "" {
		row.PollState = string(schema.PollStateActive)
	}
	return row
}

func (m Company) toDomain() schema.Company {
	return schema.Company{
		Key:              m.Key,
		Name:             m.Name,
		Aliases:          nilIfEmpty([]string(m.Aliases)),
		Website:          m.Website,
		ATS:              schema.ATS(m.ATS),
		ATSSlug:          m.ATSSlug,
		BoardURL:         m.BoardURL,
		BoardETag:        m.BoardETag,
		ResolutionStatus: schema.ResolutionStatus(m.ResolutionStatus),
		ResolutionMethod: schema.ResolutionMethod(m.ResolutionMethod),
		PollState:        schema.PollState(m.PollState),
		NextResolveAt:    m.NextResolveAt,
		FirstSeenAt:      m.FirstSeenAt,
		LastSeenAt:       m.LastSeenAt,
		LastPolledAt:     m.LastPolledAt,
		PollFailCount:    m.PollFailCount,
		EmptyPollStreak:  m.EmptyPollStreak,
		NeedsReview:      m.NeedsReview,
	}
}

func nilIfEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return in
}
