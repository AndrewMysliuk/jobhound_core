package companies_workflows

import (
	"strconv"
	"strings"
)

// ResolveWorkflowID is resolve-{company_key}. One active resolve per company.
func ResolveWorkflowID(companyKey string) string {
	return "resolve-" + strings.TrimSpace(companyKey)
}

// PollWorkflowID is poll-companies-{run_id}.
func PollWorkflowID(runID string) string {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return "poll-companies-unknown"
	}
	return "poll-companies-" + runID
}

// PollWorkflowIDFromRun formats an int64 profile run id for PollWorkflowID.
func PollWorkflowIDFromRun(runID int64) string {
	return PollWorkflowID(strconv.FormatInt(runID, 10))
}

// RunIDString formats a profile run id for PollCompaniesPayload.
func RunIDString(runID int64) string {
	return strconv.FormatInt(runID, 10)
}
