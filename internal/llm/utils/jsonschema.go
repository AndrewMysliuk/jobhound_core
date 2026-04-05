package utils

import (
	"encoding/json"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const jobScoringSchemaURL = "https://jobhound.local/llm/job_scoring.schema.json"

// ValidateJSONDocument unmarshals documentJSON into a value and validates it against schemaJSON
// (Draft 2020-12 when the schema declares $schema, same as publicapi/utils.ValidateJSONInstance).
func ValidateJSONDocument(schemaJSON, documentJSON []byte) error {
	var instance any
	if err := json.Unmarshal(documentJSON, &instance); err != nil {
		return fmt.Errorf("llm utils: document json: %w", err)
	}
	var schemaRoot any
	if err := json.Unmarshal(schemaJSON, &schemaRoot); err != nil {
		return fmt.Errorf("llm utils: schema json: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(jobScoringSchemaURL, schemaRoot); err != nil {
		return fmt.Errorf("llm utils: schema load: %w", err)
	}
	sch, err := c.Compile(jobScoringSchemaURL)
	if err != nil {
		return fmt.Errorf("llm utils: schema compile: %w", err)
	}
	if err := sch.Validate(instance); err != nil {
		return fmt.Errorf("llm utils: schema validate: %w", err)
	}
	return nil
}
