package enrich_workflows

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// WorkflowID is enrich-{promptVersion}-{model}-{hash of the sorted job ids}.
// An open run with this id is the same run; a failed run may start again.
func WorkflowID(promptVersion int, model string, jobIDs []string) string {
	if len(jobIDs) == 0 {
		return DrainWorkflowID(promptVersion, model)
	}
	sorted := append([]string(nil), jobIDs...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return fmt.Sprintf("enrich-%d-%s-%s", promptVersion, model, hex.EncodeToString(sum[:]))
}

// DrainWorkflowID is the stable id for a JobIDs-empty enrich that pages ListMissing.
func DrainWorkflowID(promptVersion int, model string) string {
	return fmt.Sprintf("enrich-%d-%s-drain", promptVersion, model)
}
