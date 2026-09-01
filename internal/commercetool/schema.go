package commercetool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type compiledSchemas struct {
	input  *jsonschema.Schema
	output *jsonschema.Schema
}

func compileSchemas(definition Definition) (compiledSchemas, error) {
	input, err := compileSchema(schemaLocation(definition, "input"), definition.InputSchema)
	if err != nil {
		return compiledSchemas{}, fmt.Errorf("compile input schema: %w", err)
	}

	output, err := compileSchema(schemaLocation(definition, "output"), definition.OutputSchema)
	if err != nil {
		return compiledSchemas{}, fmt.Errorf("compile output schema: %w", err)
	}

	return compiledSchemas{input: input, output: output}, nil
}

func (schemas compiledSchemas) validateInput(payload json.RawMessage) error {
	return validatePayload(schemas.input, payload, ErrorInvalidInput, "tool input does not match schema")
}

func (schemas compiledSchemas) validateOutput(payload json.RawMessage) error {
	return validatePayload(schemas.output, payload, ErrorOutputInvalid, "tool output does not match schema")
}

func compileSchema(location string, raw json.RawMessage) (*jsonschema.Schema, error) {
	document, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}

	if err := requireClosedRootObject(document); err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(location, document); err != nil {
		return nil, err
	}

	return compiler.Compile(location)
}

func validatePayload(schema *jsonschema.Schema, payload json.RawMessage, code ErrorCode, safeMessage string) error {
	document, err := decodeJSON(payload)
	if err == nil {
		err = schema.Validate(document)
	}
	if err != nil {
		return NewError(code, safeMessage, err)
	}

	return nil
}

func decodeJSON(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("JSON must contain exactly one document")
	}

	return document, nil
}

func requireClosedRootObject(document any) error {
	root, ok := document.(map[string]any)
	if !ok {
		return errors.New("schema root must be an object")
	}
	if root["type"] != "object" {
		return errors.New("schema root type must be object")
	}
	if additionalProperties, ok := root["additionalProperties"].(bool); !ok || additionalProperties {
		return errors.New("schema root additionalProperties must be false")
	}

	return nil
}

func schemaLocation(definition Definition, direction string) string {
	return fmt.Sprintf(
		"urn:task-processor:commerce-tool:%s:%s:%s",
		definition.Ref.ID,
		definition.Ref.Version,
		direction,
	)
}
