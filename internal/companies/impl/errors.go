package impl

import (
	"errors"
	"log/slog"

	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
)

// mapPollFetchError classifies fetcher errors for the poll path.
// ErrBoardFormat is terminal for that company this run and is logged loudly.
// Other errors are returned as-is for Temporal retry.
func mapPollFetchError(log *slog.Logger, companyKey string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, companies.ErrBoardFormat) {
		logger := log
		if logger == nil {
			logger = slog.Default()
		}
		logger.Error("companies: board response format mismatch — fetcher needs updating",
			"failure_stage", schema.FailureStagePoll.String(),
			"company_key", companyKey,
			"error", err.Error(),
		)
		return companies.ErrBoardFormat
	}
	return err
}

// IsTerminalCompanyError reports whether err should not be retried for this
// company in the current run (business skip or board-format break).
func IsTerminalCompanyError(err error) bool {
	return errors.Is(err, companies.ErrBoardFormat) ||
		errors.Is(err, companies.ErrNotPollable)
}
