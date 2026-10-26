package schema

// PromptVersion is the facts prompt version stored on each verdict row.
const PromptVersion = 1

// MinDescriptionChars is the default minimum job description length for enrich.
// Shorter vacancies are skipped (MissingIDs) and reported as facts_status=off.
const MinDescriptionChars = 800

// WindowDays is the default first_seen_at window for enrich candidates.
// Wider than scoring's ScoreWindowDays so a once-enriched vacancy can stay listed.
const WindowDays = 30

// PostedAtMaxAgeDays skips enrich for vacancies whose known posted_at is older
// than this many days: scoring already rejects them via CUT_POSTED_AT, and a
// non-fact gate forces REJECTED before fact signals are considered
// (scoring/impl/evaluate.go chooseBucket), so the verdict cannot change the bucket.
// Unknown (zero) posted_at stays eligible, same as scoring.
const PostedAtMaxAgeDays = 30

// MaxJobsPerSubmit caps one SubmitBatch / ListMissing page so Temporal activity
// payloads stay under the default ~2 MiB blob limit (Anthropic allows far more).
const MaxJobsPerSubmit = 1000
