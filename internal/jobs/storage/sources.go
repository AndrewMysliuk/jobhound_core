package storage

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/jobs/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// textArray is jobs.sources: Postgres TEXT[], scanned from a text literal.
// SQLite tests store that same literal, or a JSON array written by hand.
type textArray []string

func (a textArray) GormValue(_ context.Context, db *gorm.DB) clause.Expr {
	lit := formatTextArray([]string(a))
	if db != nil && db.Dialector != nil && db.Name() == "postgres" {
		return clause.Expr{SQL: "?::text[]", Vars: []any{lit}}
	}
	return clause.Expr{SQL: "?", Vars: []any{lit}}
}

func (a textArray) Value() (driver.Value, error) {
	return formatTextArray([]string(a)), nil
}

func (a *textArray) Scan(src any) error {
	if a == nil {
		return fmt.Errorf("sources: scan on nil textArray")
	}
	switch v := src.(type) {
	case nil:
		*a = nil
		return nil
	case string:
		parsed, err := utils.ParseSourceIDs(v)
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	case []byte:
		parsed, err := utils.ParseSourceIDs(string(v))
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	default:
		return fmt.Errorf("sources: unsupported type %T", src)
	}
}

func formatTextArray(ids []string) string {
	if len(ids) == 0 {
		return "{}"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = `"` + strings.ReplaceAll(strings.ReplaceAll(id, `\`, `\\`), `"`, `\"`) + `"`
	}
	return "{" + strings.Join(parts, ",") + "}"
}
