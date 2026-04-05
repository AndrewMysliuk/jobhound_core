package schema

import _ "embed"

// JobScoringJSON is the JSON Schema for stage-3 LLM scoring (Anthropic output_config + local validation).
//
//go:embed json_schema/job_scoring.schema.json
var JobScoringJSON []byte
