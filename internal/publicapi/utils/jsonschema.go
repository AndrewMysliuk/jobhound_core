package utils

import (
	"encoding/json"
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const inlineSchemaURL = "https://jobhound.local/request.schema.json"

// ValidateJSONInstance checks a decoded JSON value (typically from json.Unmarshal into any)
// against a JSON Schema document (Draft 2020-12 by default). Same role as go-common's
// ValidateRequestBySchema before binding a Gin body — here for net/http after syntax check.
func ValidateJSONInstance(schema []byte, instance any) error {
	var schemaRoot any
	if err := json.Unmarshal(schema, &schemaRoot); err != nil {
		return fmt.Errorf("schema json: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(inlineSchemaURL, schemaRoot); err != nil {
		return fmt.Errorf("schema load: %w", err)
	}
	sch, err := c.Compile(inlineSchemaURL)
	if err != nil {
		return fmt.Errorf("schema compile: %w", err)
	}
	if err := sch.Validate(instance); err != nil {
		return err
	}
	return nil
}
