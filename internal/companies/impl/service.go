package impl

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

const (
	emptyPollStreakToInactive = 5
	deadSlugStreakToPending   = 3
)

// companyStore is the persistence surface the service needs.
type companyStore interface {
	Upsert(ctx context.Context, c schema.Company) error
	GetByKey(ctx context.Context, key string) (schema.Company, error)
	FindByWebsite(ctx context.Context, website string) (schema.Company, error)
	FindByAlias(ctx context.Context, alias string) (schema.Company, error)
	List(ctx context.Context, f schema.ListFilter) ([]schema.Company, error)
	Count(ctx context.Context, f schema.ListFilter) (int, error)
	ListPendingResolve(ctx context.Context, now time.Time, limit int) ([]schema.Company, error)
	ListDuePollable(ctx context.Context, dueBefore time.Time, limit int) ([]schema.Company, error)
	MarkPolled(ctx context.Context, p schema.MarkPolledParams) error
	MarkResolution(ctx context.Context, p schema.MarkResolutionParams) error
}

// jobBoardStore upserts ATS vacancies and applies close/reopen for a company.
type jobBoardStore interface {
	UpsertBoardJobs(ctx context.Context, companyKey string, jobs []jobdata.Job) error
	CloseAbsent(ctx context.Context, companyKey string, seenIDs []string) (int64, error)
	ReopenPresent(ctx context.Context, companyKey string, seenIDs []string) (int64, error)
	// BackfillCompanyKeyRef links aggregator jobs to a company after discover.
	BackfillCompanyKeyRef(ctx context.Context, companyKey string, contentKeys []string) (int64, error)
	// CountOpenByCompanyKeyRef counts open jobs linked to the company (aggregator
	// fallback signal). May be nil on the Service — then I7 logging is skipped.
	CountOpenByCompanyKeyRef(ctx context.Context, companyKey string) (int64, error)
}

// Service is the companies use-case surface. It does not import Temporal.
type Service struct {
	companies companyStore
	jobs      jobBoardStore
	blocklist *BlocklistMatcher
	fetchers  map[schema.ATS]companies.CompanyBoardFetcher
	cfg       config.Companies
	http      *http.Client
	log       *slog.Logger
	now       func() time.Time
}

// New wires companies storage, optional jobs board store, blocklist, and fetchers.
// jobs may be nil when only discover/resolve are exercised; Poll then fails on
// outcomes that need upsert/close. fetchers may be empty until F* lands.
func New(
	companiesRepo companyStore,
	jobs jobBoardStore,
	blocklist *BlocklistMatcher,
	fetchers []companies.CompanyBoardFetcher,
	cfg config.Companies,
) *Service {
	fm := make(map[schema.ATS]companies.CompanyBoardFetcher, len(fetchers))
	for _, f := range fetchers {
		if f == nil {
			continue
		}
		fm[f.ATS()] = f
	}
	return &Service{
		companies: companiesRepo,
		jobs:      jobs,
		blocklist: blocklist,
		fetchers:  fm,
		cfg:       cfg,
		http: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				return nil
			},
		},
		log: slog.Default(),
		now: func() time.Time { return time.Now().UTC() },
	}
}

var _ companies.API = (*Service)(nil)

// ATSConcurrency is the parallel poll limit per ATS from config.
func (s *Service) ATSConcurrency() int {
	if s == nil || s.cfg.ATSConcurrency < 1 {
		return config.DefaultCompaniesATSConcurrency
	}
	return s.cfg.ATSConcurrency
}

// List returns a page of companies matching the filter.
func (s *Service) List(ctx context.Context, f schema.ListFilter) ([]schema.Company, error) {
	if s == nil || s.companies == nil {
		return nil, errors.New("companies: service not configured")
	}
	return s.companies.List(ctx, f)
}

// Count returns how many companies match the filter (ignoring page/limit).
func (s *Service) Count(ctx context.Context, f schema.ListFilter) (int, error) {
	if s == nil || s.companies == nil {
		return 0, errors.New("companies: service not configured")
	}
	return s.companies.Count(ctx, f)
}
