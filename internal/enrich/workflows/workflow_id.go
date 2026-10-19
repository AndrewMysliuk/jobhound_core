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
	sorted := append([]string(nil), jobIDs...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return fmt.Sprintf("enrich-%d-%s-%s", promptVersion, model, hex.EncodeToString(sum[:]))
}
