package commercetool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileSchemasRejectsInvalidSchemaAtConstruction(t *testing.T) {
	definition := validDefinition()
	definition.InputSchema = json.RawMessage(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"additionalProperties":false,
		"properties":{"task_id":{"type":"not-a-json-schema-type"}}
	}`)

	_, err := compileSchemas(definition)

	require.Error(t, err)
	require.ErrorContains(t, err, "metaschema")
	require.NotContains(t, err.Error(), "schema root")
}

func TestCompileSchemasRejectsUndeclaredInputAuthority(t *testing.T) {
	schemas, err := compileSchemas(validDefinition())
	require.NoError(t, err)

	err = schemas.validateInput(json.RawMessage(`{"task_id":"task-1","tenant_id":"attacker"}`))

	require.Equal(t, ErrorInvalidInput, CodeOf(err))
	require.Equal(t, "invalid_input: tool input does not match schema", err.Error())
	require.NotContains(t, err.Error(), "attacker")
}

func TestCompiledSchemasRejectsInvalidOutput(t *testing.T) {
	schemas, err := compileSchemas(validDefinition())
	require.NoError(t, err)

	err = schemas.validateOutput(json.RawMessage(`{"unexpected":true}`))

	require.Equal(t, ErrorOutputInvalid, CodeOf(err))
	require.Equal(t, "output_invalid: tool output does not match schema", err.Error())
}

func TestCompileSchemasRequiresClosedRootObjects(t *testing.T) {
	definition := validDefinition()
	definition.InputSchema = json.RawMessage(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{"task_id":{"type":"string"}}
	}`)

	_, err := compileSchemas(definition)

	require.Error(t, err)
}

func TestCompileSchemasRejectsTrailingJSON(t *testing.T) {
	definition := validDefinition()
	definition.OutputSchema = append(definition.OutputSchema, []byte(` {}`)...)

	_, err := compileSchemas(definition)

	require.Error(t, err)
}

func TestCompiledSchemasAcceptsValidInputAndOutput(t *testing.T) {
	schemas, err := compileSchemas(validDefinition())
	require.NoError(t, err)

	require.NoError(t, schemas.validateInput(json.RawMessage(`{"task_id":"task-1"}`)))
	require.NoError(t, schemas.validateOutput(json.RawMessage(`{"task_id":"task-1"}`)))
}

func TestCompiledSchemasRejectsMalformedAndTrailingPayloadsAtSafeBoundaries(t *testing.T) {
	schemas, err := compileSchemas(validDefinition())
	require.NoError(t, err)

	inputErr := schemas.validateInput(json.RawMessage(`{"task_id":`))
	trailingInputErr := schemas.validateInput(json.RawMessage(`{"task_id":"task-1"} {}`))
	outputErr := schemas.validateOutput(json.RawMessage(`{"task_id":`))
	trailingOutputErr := schemas.validateOutput(json.RawMessage(`{"task_id":"task-1"} {}`))

	require.Equal(t, ErrorInvalidInput, CodeOf(inputErr))
	require.Equal(t, ErrorInvalidInput, CodeOf(trailingInputErr))
	require.Equal(t, ErrorOutputInvalid, CodeOf(outputErr))
	require.Equal(t, ErrorOutputInvalid, CodeOf(trailingOutputErr))
}

func TestCompileSchemasUsesDistinctStableSchemaLocations(t *testing.T) {
	schemas, err := compileSchemas(validDefinition())
	require.NoError(t, err)

	require.Equal(t, "urn:task-processor:commerce-tool:product.canonical.inspect:v1.0.0:input", schemaLocation(validDefinition(), "input"))
	require.Equal(t, "urn:task-processor:commerce-tool:product.canonical.inspect:v1.0.0:output", schemaLocation(validDefinition(), "output"))
	require.Equal(t, "urn:task-processor:commerce-tool:product.canonical.inspect:v1.0.0:input#", schemas.input.Location)
	require.Equal(t, "urn:task-processor:commerce-tool:product.canonical.inspect:v1.0.0:output#", schemas.output.Location)
}

func TestCompiledSchemasDoesNotDependOnLaterSchemaMutation(t *testing.T) {
	definition := validDefinition()
	schemas, err := compileSchemas(definition)
	require.NoError(t, err)

	copy(definition.InputSchema, []byte(`{"type":"object","additionalProperties":true}`))

	err = schemas.validateInput(json.RawMessage(`{"unexpected":true}`))

	require.Equal(t, ErrorInvalidInput, CodeOf(err))
}
