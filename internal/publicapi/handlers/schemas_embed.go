package handlers

import _ "embed"

// Request body JSON Schemas (validated before typed decode). Same workflow as
// omg-ap handlers: //go:embed + validate, then bind.

//go:embed json_schema/create_slot.schema.json
var schemaCreateSlot []byte

//go:embed json_schema/profile_put.schema.json
var schemaProfilePut []byte

//go:embed json_schema/stage2_run.schema.json
var schemaStage2Run []byte

//go:embed json_schema/stage3_run.schema.json
var schemaStage3Run []byte

//go:embed json_schema/patch_job_bucket.schema.json
var schemaPatchJobBucket []byte
