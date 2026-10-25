package schema

// PromptVersion is the facts prompt version stored on each verdict row.
const PromptVersion = 1

// MinDescriptionChars is the default minimum job description length for enrich.
// Shorter vacancies are skipped (MissingIDs) and reported as facts_status=off.
const MinDescriptionChars = 800
