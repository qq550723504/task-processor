package commercetool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

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
	if err := auditSchema(document, "#"); err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	if err := compiler.AddResource(location, document); err != nil {
		return nil, err
	}

	return compiler.Compile(location)
}

const draft202012SchemaURL = "https://json-schema.org/draft/2020-12/schema"

func auditSchema(value any, path string) error {
	switch schema := value.(type) {
	case bool:
		if schema {
			return fmt.Errorf("schema at %s cannot prove object schema excludes reserved authority fields", path)
		}
		return nil
	case map[string]any:
		return auditSchemaObject(schema, path)
	default:
		return nil
	}
}

func auditSchemaObject(schema map[string]any, path string) error {
	if dialect, exists := schema["$schema"]; exists {
		if dialect != draft202012SchemaURL {
			return fmt.Errorf("schema dialect must be JSON Schema Draft 2020-12 at %s", path)
		}
	}

	for _, keyword := range []string{"$ref", "$dynamicRef"} {
		if reference, exists := schema[keyword]; exists {
			if text, ok := reference.(string); !ok || !strings.HasPrefix(text, "#") {
				return fmt.Errorf("schema reference must use a current-document fragment at %s/%s", path, escapeJSONPointerToken(keyword))
			}
		}
	}

	if err := auditReservedPropertyNames(schema, path); err != nil {
		return err
	}
	if err := auditSchemaLiterals(schema, path); err != nil {
		return err
	}
	if err := auditSubschemas(schema, path); err != nil {
		return err
	}
	if err := requireStaticallySafeValueShape(schema, path); err != nil {
		return err
	}

	return nil
}

func auditReservedPropertyNames(schema map[string]any, path string) error {
	for _, keyword := range []string{"properties", "dependentSchemas"} {
		entries, _ := schema[keyword].(map[string]any)
		for _, name := range sortedSchemaKeys(entries) {
			if isReservedAuthorityField(name) {
				return fmt.Errorf("reserved authority field %q declared at %s/%s", name, path, escapeJSONPointerToken(keyword))
			}
		}
	}

	for _, keyword := range []string{"required"} {
		names, _ := schema[keyword].([]any)
		for _, value := range names {
			if name, ok := value.(string); ok && isReservedAuthorityField(name) {
				return fmt.Errorf("reserved authority field %q declared at %s/%s", name, path, escapeJSONPointerToken(keyword))
			}
		}
	}

	dependentRequired, _ := schema["dependentRequired"].(map[string]any)
	for _, name := range sortedSchemaKeys(dependentRequired) {
		if isReservedAuthorityField(name) {
			return fmt.Errorf("reserved authority field %q declared at %s/dependentRequired", name, path)
		}
		values, _ := dependentRequired[name].([]any)
		for _, value := range values {
			if dependency, ok := value.(string); ok && isReservedAuthorityField(dependency) {
				return fmt.Errorf("reserved authority field %q declared at %s/dependentRequired", dependency, path)
			}
		}
	}

	patterns, _ := schema["patternProperties"].(map[string]any)
	for _, pattern := range sortedSchemaKeys(patterns) {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid patternProperties pattern %q at %s: %w", pattern, path, err)
		}
		for _, reserved := range reservedAuthorityFields {
			if compiled.MatchString(reserved) {
				return fmt.Errorf("patternProperties pattern matches reserved authority field %q at %s", reserved, path)
			}
		}
	}

	return nil
}

func auditSchemaLiterals(schema map[string]any, path string) error {
	if constant, exists := schema["const"]; exists {
		if reserved, found := findReservedAuthorityField(constant); found {
			return fmt.Errorf("reserved authority field %q declared at %s/const", reserved, path)
		}
	}
	if enum, ok := schema["enum"].([]any); ok {
		for _, value := range enum {
			if reserved, found := findReservedAuthorityField(value); found {
				return fmt.Errorf("reserved authority field %q declared at %s/enum", reserved, path)
			}
		}
	}

	return nil
}

func auditSubschemas(schema map[string]any, path string) error {
	mapKeywords := []string{
		"$defs",
		"definitions",
		"properties",
		"patternProperties",
		"dependentSchemas",
	}
	for _, keyword := range mapKeywords {
		children, _ := schema[keyword].(map[string]any)
		for _, name := range sortedSchemaKeys(children) {
			if err := auditSchema(children[name], schemaPath(path, keyword, name)); err != nil {
				return err
			}
		}
	}

	singleKeywords := []string{
		"additionalProperties",
		"unevaluatedProperties",
		"propertyNames",
		"items",
		"contains",
		"not",
		"if",
		"then",
		"else",
		"unevaluatedItems",
		"contentSchema",
	}
	for _, keyword := range singleKeywords {
		child, exists := schema[keyword]
		if !exists {
			continue
		}
		if err := auditSchema(child, schemaPath(path, keyword)); err != nil {
			return err
		}
	}

	arrayKeywords := []string{"allOf", "anyOf", "oneOf", "prefixItems"}
	for _, keyword := range arrayKeywords {
		children, _ := schema[keyword].([]any)
		for index, child := range children {
			if err := auditSchema(child, schemaPath(path, keyword, fmt.Sprintf("%d", index))); err != nil {
				return err
			}
		}
	}

	if legacyDependencies, ok := schema["dependencies"].(map[string]any); ok {
		for _, name := range sortedSchemaKeys(legacyDependencies) {
			child := legacyDependencies[name]
			if _, isPropertyList := child.([]any); isPropertyList {
				continue
			}
			if err := auditSchema(child, schemaPath(path, "dependencies", name)); err != nil {
				return err
			}
		}
	}

	return nil
}

func requireStaticallySafeValueShape(schema map[string]any, path string) error {
	allowsObject, allowsArray, explicitTypes, validTypes := schemaTypeCapabilities(schema["type"])
	if !validTypes {
		return nil
	}

	if !explicitTypes {
		if schemaDelegatesValueShape(schema) || schemaHasFiniteValues(schema) {
			return nil
		}
		return fmt.Errorf("schema at %s cannot prove object schema excludes reserved authority fields", path)
	}

	if allowsObject {
		additionalProperties, ok := schema["additionalProperties"].(bool)
		if !ok || additionalProperties {
			return fmt.Errorf("schema at %s cannot prove object schema excludes reserved authority fields: additionalProperties must be false", path)
		}
	}
	if allowsArray {
		items, exists := schema["items"]
		if !exists {
			return fmt.Errorf("schema at %s cannot prove object schema excludes reserved authority fields: array items must be constrained", path)
		}
		if allowAll, ok := items.(bool); ok && allowAll {
			return fmt.Errorf("schema at %s cannot prove object schema excludes reserved authority fields: array items must be constrained", path)
		}
	}

	return nil
}

func schemaTypeCapabilities(value any) (allowsObject, allowsArray, explicit, valid bool) {
	if value == nil {
		return false, false, false, true
	}

	addType := func(name string) bool {
		switch name {
		case "object":
			allowsObject = true
		case "array":
			allowsArray = true
		case "null", "boolean", "number", "integer", "string":
		default:
			return false
		}
		return true
	}

	switch types := value.(type) {
	case string:
		valid := addType(types)
		return allowsObject, allowsArray, true, valid
	case []any:
		for _, item := range types {
			name, ok := item.(string)
			if !ok || !addType(name) {
				return false, false, true, false
			}
		}
		return allowsObject, allowsArray, true, true
	default:
		return false, false, true, false
	}
}

func schemaDelegatesValueShape(schema map[string]any) bool {
	for _, keyword := range []string{"$ref", "$dynamicRef", "allOf", "anyOf", "oneOf"} {
		if _, exists := schema[keyword]; exists {
			return true
		}
	}
	return false
}

func schemaHasFiniteValues(schema map[string]any) bool {
	if _, exists := schema["const"]; exists {
		return true
	}
	_, exists := schema["enum"]
	return exists
}

func findReservedAuthorityField(value any) (string, bool) {
	switch value := value.(type) {
	case map[string]any:
		for _, name := range sortedSchemaKeys(value) {
			if isReservedAuthorityField(name) {
				return name, true
			}
			if reserved, found := findReservedAuthorityField(value[name]); found {
				return reserved, true
			}
		}
	case []any:
		for _, item := range value {
			if reserved, found := findReservedAuthorityField(item); found {
				return reserved, true
			}
		}
	}

	return "", false
}

func sortedSchemaKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func schemaPath(parent string, tokens ...string) string {
	path := parent
	for _, token := range tokens {
		path += "/" + escapeJSONPointerToken(token)
	}
	return path
}

func escapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
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
