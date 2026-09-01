package commercetool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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

func TestCompileSchemasRejectsReservedAuthorityInReferencedAnnotationTargets(t *testing.T) {
	tests := []struct {
		name     string
		schema   json.RawMessage
		reserved string
	}{
		{
			name: "default target",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$ref":"#/default"}},
				"default":{"type":"object","additionalProperties":false,"properties":{"tenant_id":{"type":"string"}}}
			}`),
			reserved: "tenant_id",
		},
		{
			name: "examples target",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$ref":"#/examples/0"}},
				"examples":[{"type":"object","additionalProperties":false,"properties":{"user_id":{"type":"string"}}}]
			}`),
			reserved: "user_id",
		},
		{
			name: "escaped pointer target",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$ref":"#/x~1target~0schema"}},
				"x/target~schema":{"type":"object","additionalProperties":false,"properties":{"roles":{"type":"array","items":{"type":"string"}}}}
			}`),
			reserved: "roles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileSchema("urn:test:referenced-annotation", tt.schema)

			require.Error(t, err)
			require.ErrorContains(t, err, fmt.Sprintf("reserved authority field %q", tt.reserved))
		})
	}
}

func TestCompileSchemasRejectsUnsafeReferencedAnnotationTargets(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		errorMatch string
	}{
		{
			name:       "old dialect",
			target:     `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","additionalProperties":false}`,
			errorMatch: "schema dialect must be JSON Schema Draft 2020-12",
		},
		{
			name:       "open object",
			target:     `{"type":"object"}`,
			errorMatch: "cannot prove object schema excludes reserved authority fields",
		},
		{
			name:       "reserved pattern",
			target:     `{"type":"object","additionalProperties":false,"patternProperties":{"^tenant_.*$":{"type":"string"}}}`,
			errorMatch: "patternProperties pattern matches reserved authority field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := json.RawMessage(fmt.Sprintf(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$ref":"#/default"}},
				"default":%s
			}`, tt.target))

			_, err := compileSchema("urn:test:unsafe-referenced-annotation", schema)

			require.Error(t, err)
			require.ErrorContains(t, err, tt.errorMatch)
		})
	}
}

func TestCompileSchemasDoesNotAuditUnreferencedAnnotationBusinessData(t *testing.T) {
	tests := []struct {
		name   string
		schema json.RawMessage
	}{
		{
			name: "business examples",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"task_id":{"type":"string"}},
				"default":{"tenant_id":"business-example"},
				"examples":[{"user_id":"business-example"}]
			}`),
		},
		{
			name: "irrelevant annotation anchor",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$dynamicRef":"#payload"}},
				"$defs":{"payload":{"$dynamicAnchor":"payload","type":"object","additionalProperties":false}},
				"examples":[{"$dynamicAnchor":"payload","tenant_id":"business-example"}]
			}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := compileSchema("urn:test:unreferenced-annotations", tt.schema)

			require.NoError(t, err)
			require.NoError(t, compiled.Validate(map[string]any{}))
		})
	}
}

func TestCompileSchemasAcceptsReachableLocalAnchorsAndCycles(t *testing.T) {
	tests := []struct {
		name   string
		schema json.RawMessage
	}{
		{
			name: "anchor",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$ref":"#payload"}},
				"$defs":{"payload":{"$anchor":"payload","type":"object","additionalProperties":false,"properties":{"sku":{"type":"string"}}}}
			}`),
		},
		{
			name: "dynamic anchor",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$dynamicRef":"#payload"}},
				"$defs":{"payload":{"$dynamicAnchor":"payload","type":"object","additionalProperties":false,"properties":{"sku":{"type":"string"}}}}
			}`),
		},
		{
			name: "circular annotation targets",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$ref":"#/default"}},
				"default":{"type":"object","additionalProperties":false,"properties":{"next":{"$ref":"#/examples/0"}}},
				"examples":[{"type":"object","additionalProperties":false,"properties":{"next":{"$ref":"#/default"}}}]
			}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := compileSchema("urn:test:local-reference-graph", tt.schema)

			require.NoError(t, err)
			require.NoError(t, compiled.Validate(map[string]any{"payload": map[string]any{}}))
		})
	}
}

func TestCompileSchemasLeavesInvalidOrUnresolvedFragmentsToCompiler(t *testing.T) {
	tests := []struct {
		reference   string
		compilerErr string
	}{
		{reference: "#/missing", compilerErr: "json-pointer in"},
		{reference: "#/bad~2pointer", compilerErr: "invalid json-pointer"},
	}
	for _, tt := range tests {
		t.Run(tt.reference, func(t *testing.T) {
			schema := json.RawMessage(fmt.Sprintf(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"payload":{"$ref":%q}}
			}`, tt.reference))

			_, err := compileSchema("urn:test:invalid-fragment", schema)

			require.Error(t, err)
			require.ErrorContains(t, err, tt.compilerErr)
		})
	}
}

func TestCompileSchemasRejectsEmbeddedResourceIdentifiers(t *testing.T) {
	schema := json.RawMessage(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"additionalProperties":false,
		"properties":{
			"payload":{
				"$id":"urn:test:nested-resource",
				"$ref":"#/default",
				"default":{"type":"object","additionalProperties":false,"properties":{"tenant_id":{"type":"string"}}}
			}
		}
	}`)

	_, err := compileSchema("urn:test:embedded-resource", schema)

	require.Error(t, err)
	require.ErrorContains(t, err, "schema resource identifiers are not supported")
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
	tests := []struct {
		name    string
		schema  json.RawMessage
		message string
	}{
		{
			name:    "root is not an object",
			schema:  json.RawMessage(`[]`),
			message: "schema root must be an object",
		},
		{
			name:    "type is not object",
			schema:  json.RawMessage(`{"type":"array","additionalProperties":false}`),
			message: "schema root type must be object",
		},
		{
			name:    "type is not an exact object string",
			schema:  json.RawMessage(`{"type":["object"],"additionalProperties":false}`),
			message: "schema root type must be object",
		},
		{
			name:    "additional properties missing",
			schema:  json.RawMessage(`{"type":"object"}`),
			message: "schema root additionalProperties must be false",
		},
		{
			name:    "additional properties true",
			schema:  json.RawMessage(`{"type":"object","additionalProperties":true}`),
			message: "schema root additionalProperties must be false",
		},
		{
			name:    "additional properties is not boolean",
			schema:  json.RawMessage(`{"type":"object","additionalProperties":"false"}`),
			message: "schema root additionalProperties must be false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			definition.InputSchema = tt.schema

			_, err := compileSchemas(definition)

			require.ErrorContains(t, err, tt.message)
		})
	}
}

func TestCompileSchemasRejectsUnregisteredExternalSchemaReferences(t *testing.T) {
	externalSchemaPath := filepath.Join(t.TempDir(), "external-schema.json")
	require.NoError(t, os.WriteFile(externalSchemaPath, []byte(`{"type":"string"}`), 0o600))

	definition := validDefinition()
	externalSchemaURL := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(externalSchemaPath)}).String()
	definition.InputSchema = json.RawMessage(fmt.Sprintf(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"additionalProperties":false,
		"properties":{"task_id":{"$ref":%q}}
	}`, externalSchemaURL))

	_, err := compileSchemas(definition)

	require.Error(t, err)
	require.ErrorContains(t, err, "schema reference must use a current-document fragment")
}

func TestCompileSchemasRejectsNonDraft202012Dialects(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		schema    json.RawMessage
	}{
		{
			name:      "input root draft 7",
			direction: "input",
			schema: json.RawMessage(`{
				"$schema":"http://json-schema.org/draft-07/schema#",
				"type":"object",
				"additionalProperties":false,
				"properties":{"task_id":{"type":"string"}}
			}`),
		},
		{
			name:      "output root draft 2019-09",
			direction: "output",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2019-09/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"task_id":{"type":"string"}}
			}`),
		},
		{
			name:      "input nested resource changes dialect",
			direction: "input",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"task_id":{"$ref":"#/$defs/task"}},
				"$defs":{"task":{"$id":"nested-task","$schema":"http://json-schema.org/draft-07/schema#","type":"string"}}
			}`),
		},
		{
			name:      "output nested resource changes dialect",
			direction: "output",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"task_id":{"type":"string"}},
				"$defs":{"legacy":{"$id":"nested-legacy","$schema":"https://json-schema.org/draft/2019-09/schema","type":"string"}}
			}`),
		},
		{
			name:      "input content schema changes dialect",
			direction: "input",
			schema: json.RawMessage(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"document":{
					"type":"string",
					"contentMediaType":"application/json",
					"contentSchema":{
						"$schema":"http://json-schema.org/draft-07/schema#",
						"type":"object",
						"additionalProperties":false
					}
				}}
			}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			setDefinitionSchema(&definition, tt.direction, tt.schema)

			_, err := compileSchemas(definition)

			require.ErrorContains(t, err, "schema dialect must be JSON Schema Draft 2020-12")
			require.NotContains(t, err.Error(), "schema root")
		})
	}
}

func TestCompileSchemasRejectsNonLocalSchemaReferences(t *testing.T) {
	tests := []struct {
		name      string
		keyword   string
		reference string
	}{
		{
			name:      "built-in metaschema URL",
			keyword:   "$ref",
			reference: "https://json-schema.org/draft/2020-12/schema",
		},
		{
			name:      "HTTP URL",
			keyword:   "$ref",
			reference: "https://schemas.example.test/task.json#/$defs/task",
		},
		{
			name:      "relative URL",
			keyword:   "$ref",
			reference: "task.json#/$defs/task",
		},
		{
			name:      "file URL",
			keyword:   "$ref",
			reference: "file:///tmp/task.json",
		},
		{
			name:      "dynamic HTTP URL",
			keyword:   "$dynamicRef",
			reference: "https://schemas.example.test/dynamic.json#task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			definition.InputSchema = json.RawMessage(fmt.Sprintf(`{
				"$schema":"https://json-schema.org/draft/2020-12/schema",
				"type":"object",
				"additionalProperties":false,
				"properties":{"task_id":{%q:%q}}
			}`, tt.keyword, tt.reference))

			_, err := compileSchemas(definition)

			require.ErrorContains(t, err, "schema reference must use a current-document fragment")
		})
	}
}

func TestCompileSchemasRejectsReservedAuthorityPropertiesInInputAndOutput(t *testing.T) {
	reservedFields := []string{
		"tenant_id",
		"user_id",
		"roles",
		"call_id",
		"agent_id",
		"agent_version",
		"agent_run_id",
		"business_task_id",
		"trace_id",
		"idempotency_key",
		"tool_id",
		"tool_version",
		"permission",
	}

	for _, direction := range []string{"input", "output"} {
		for _, field := range reservedFields {
			t.Run(direction+" "+field, func(t *testing.T) {
				definition := validDefinition()
				schema := json.RawMessage(fmt.Sprintf(`{
					"$schema":"https://json-schema.org/draft/2020-12/schema",
					"type":"object",
					"additionalProperties":false,
					"properties":{%q:{"type":"string"}}
				}`, field))
				setDefinitionSchema(&definition, direction, schema)

				_, err := compileSchemas(definition)

				require.ErrorContains(t, err, fmt.Sprintf("reserved authority field %q", field))
			})
		}
	}
}

func TestCompileSchemasRejectsReservedAuthorityPaths(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		schema    json.RawMessage
		field     string
	}{
		{
			name:      "nested property",
			direction: "input",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"payload":{"type":"object","additionalProperties":false,"properties":{"tenant_id":{"type":"string"}}}}
			}`),
			field: "tenant_id",
		},
		{
			name:      "$defs property",
			direction: "output",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"payload":{"$ref":"#/$defs/payload"}},
				"$defs":{"payload":{"type":"object","additionalProperties":false,"properties":{"user_id":{"type":"string"}}}}
			}`),
			field: "user_id",
		},
		{
			name:      "definitions property",
			direction: "input",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"task_id":{"type":"string"}},
				"definitions":{"metadata":{"type":"object","additionalProperties":false,"properties":{"trace_id":{"type":"string"}}}}
			}`),
			field: "trace_id",
		},
		{
			name:      "combination branch",
			direction: "output",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"payload":{"oneOf":[
					{"type":"object","additionalProperties":false,"properties":{"agent_id":{"type":"string"}}},
					{"type":"string"}
				]}}
			}`),
			field: "agent_id",
		},
		{
			name:      "required name",
			direction: "input",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"required":["tool_version"],"properties":{}
			}`),
			field: "tool_version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			setDefinitionSchema(&definition, tt.direction, tt.schema)

			_, err := compileSchemas(definition)

			require.ErrorContains(t, err, fmt.Sprintf("reserved authority field %q", tt.field))
		})
	}
}

func TestCompileSchemasRejectsPatternsMatchingReservedAuthorityFields(t *testing.T) {
	definition := validDefinition()
	definition.OutputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"patternProperties":{"^(tenant_id|sku)$":{"type":"string"}}
	}`)

	_, err := compileSchemas(definition)

	require.ErrorContains(t, err, "patternProperties pattern matches reserved authority field \"tenant_id\"")
}

func TestCompileSchemasRejectsObjectPathsThatCannotProveAuthorityExclusion(t *testing.T) {
	tests := []struct {
		name   string
		schema json.RawMessage
	}{
		{
			name: "nested open object",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"payload":{"type":"object","properties":{"sku":{"type":"string"}}}}
			}`),
		},
		{
			name: "open object in $defs",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"task_id":{"type":"string"}},
				"$defs":{"payload":{"type":"object","additionalProperties":true}}
			}`),
		},
		{
			name: "open combination branch",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"payload":{"anyOf":[{"type":"object"},{"type":"string"}]}}
			}`),
		},
		{
			name: "unconstrained pattern value",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"patternProperties":{"^business_[a-z]+$":{}}
			}`),
		},
		{
			name: "unconstrained array item",
			schema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"properties":{"items":{"type":"array","items":{}}}
			}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := validDefinition()
			definition.InputSchema = tt.schema

			_, err := compileSchemas(definition)

			require.ErrorContains(t, err, "cannot prove object schema excludes reserved authority fields")
		})
	}
}

func TestCompileSchemasAcceptsLocalRefsAndClosedNestedBusinessObjects(t *testing.T) {
	definition := validDefinition()
	definition.InputSchema = json.RawMessage(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"additionalProperties":false,
		"required":["product"],
		"properties":{"product":{"$ref":"#/$defs/product"}},
		"$defs":{"product":{
			"type":"object",
			"additionalProperties":false,
			"required":["sku","facts"],
			"properties":{
				"sku":{"type":"string"},
				"facts":{"type":"object","additionalProperties":false,"patternProperties":{"^business_[a-z]+$":{"type":"string"}}}
			}
		}}
	}`)
	definition.OutputSchema = append(json.RawMessage(nil), definition.InputSchema...)

	schemas, err := compileSchemas(definition)
	require.NoError(t, err)
	payload := json.RawMessage(`{"product":{"sku":"sku-1","facts":{"business_color":"blue"}}}`)

	require.NoError(t, schemas.validateInput(payload))
	require.NoError(t, schemas.validateOutput(payload))
}

func setDefinitionSchema(definition *Definition, direction string, schema json.RawMessage) {
	if direction == "input" {
		definition.InputSchema = schema
		return
	}

	definition.OutputSchema = schema
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

	stringTypeOffset := bytes.Index(definition.InputSchema, []byte(`"string"`))
	require.GreaterOrEqual(t, stringTypeOffset, 0)
	copy(definition.InputSchema[stringTypeOffset:stringTypeOffset+len(`"string"`)], []byte(`"number"`))

	require.NoError(t, schemas.validateInput(json.RawMessage(`{"task_id":"task-1"}`)))
	err = schemas.validateInput(json.RawMessage(`{"task_id":1}`))

	require.Equal(t, ErrorInvalidInput, CodeOf(err))
}
