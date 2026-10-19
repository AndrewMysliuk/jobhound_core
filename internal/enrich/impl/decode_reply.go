package impl

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

const modelReplySchemaURL = "https://jobhound.local/enrich/model_reply.schema.json"

// decodeModelReply validates body against the embedded ModelReply JSON Schema,
// then decodes into schema.ModelReply with DisallowUnknownFields.
func decodeModelReply(body []byte) (schema.ModelReply, error) {
	var instance any
	if err := json.Unmarshal(body, &instance); err != nil {
		return schema.ModelReply{}, fmt.Errorf("model reply json: %w", err)
	}
	if err := validateModelReplyInstance(instance); err != nil {
		return schema.ModelReply{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var out schema.ModelReply
	if err := dec.Decode(&out); err != nil {
		return schema.ModelReply{}, fmt.Errorf("model reply decode: %w", err)
	}
	return out, nil
}

func validateModelReplyInstance(instance any) error {
	var schemaRoot any
	if err := json.Unmarshal(schema.ModelReplyJSONSchema, &schemaRoot); err != nil {
		return fmt.Errorf("model reply schema json: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(modelReplySchemaURL, schemaRoot); err != nil {
		return fmt.Errorf("model reply schema load: %w", err)
	}
	sch, err := c.Compile(modelReplySchemaURL)
	if err != nil {
		return fmt.Errorf("model reply schema compile: %w", err)
	}
	if err := sch.Validate(instance); err != nil {
		return fmt.Errorf("model reply schema: %w", err)
	}
	return nil
}
