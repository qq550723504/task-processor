package canonicalinspect

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
)

func TestSchemasCompileInCommerceToolRegistry(t *testing.T) {
	_, err := commercetool.NewRegistry(commercetool.Tool{
		Definition: Definition(),
		Executor: commercetool.ExecutorFunc(func(_ context.Context, _ commercetool.ExecutionEnvelope, _ json.RawMessage) (commercetool.ExecutionResult, error) {
			return commercetool.ExecutionResult{}, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
}

func TestInputSchemaIsStrictAndTaskOnly(t *testing.T) {
	schema := decodeSchema(t, InputSchema())
	if schema["additionalProperties"] != false {
		t.Fatalf("input additionalProperties = %#v", schema["additionalProperties"])
	}
	required := stringSet(schema["required"])
	if len(required) != 1 || !required["task_id"] {
		t.Fatalf("input required = %#v", schema["required"])
	}
	properties := object(schema["properties"])
	if len(properties) != 1 || properties["task_id"] == nil {
		t.Fatalf("input properties = %#v", properties)
	}
}

func TestOutputSchemaTracksAllCatalogSnapshotFields(t *testing.T) {
	schema := decodeSchema(t, OutputSchema())
	defs := object(schema["$defs"])
	snapshot := object(object(defs["product_snapshot"])["properties"])
	want := jsonFieldSet(reflect.TypeOf(catalog.ProductSnapshot{}))
	if !reflect.DeepEqual(stringSetFromMap(snapshot), want) {
		t.Fatalf("snapshot schema fields = %#v, want %#v", stringSetFromMap(snapshot), want)
	}
	if object(defs["product_snapshot"])["additionalProperties"] != false {
		t.Fatal("product snapshot schema is not strict")
	}
	for _, field := range []string{"task_id", "product_key", "snapshot_version", "snapshot", "source_lineage", "diagnostics"} {
		if !stringSet(schema["required"])[field] {
			t.Fatalf("output field %q is not required", field)
		}
	}
}

func decodeSchema(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	return result
}

func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func stringSet(value any) map[string]bool {
	result := map[string]bool{}
	values, _ := value.([]any)
	for _, item := range values {
		text, _ := item.(string)
		result[text] = true
	}
	return result
}

func stringSetFromMap(values map[string]any) map[string]bool {
	result := make(map[string]bool, len(values))
	for value := range values {
		result[value] = true
	}
	return result
}

func jsonFieldSet(typ reflect.Type) map[string]bool {
	result := map[string]bool{}
	for index := 0; index < typ.NumField(); index++ {
		name := typ.Field(index).Tag.Get("json")
		for offset, char := range name {
			if char == ',' {
				name = name[:offset]
				break
			}
		}
		if name != "" && name != "-" {
			result[name] = true
		}
	}
	return result
}

func validateOutputAgainstRegistry(t *testing.T, output json.RawMessage) {
	t.Helper()
	definition := Definition()
	registry, err := commercetool.NewRegistry(commercetool.Tool{
		Definition: definition,
		Executor: commercetool.ExecutorFunc(func(context.Context, commercetool.ExecutionEnvelope, json.RawMessage) (commercetool.ExecutionResult, error) {
			return commercetool.ExecutionResult{Output: output}, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	bound, err := registry.Bind(commercetool.AgentDefinition{ID: "test.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{definition.Ref}}, commercetool.InvocationDependencies{
		PrincipalResolver: schemaPrincipalResolver{}, Authorizer: schemaAuthorizer{}, Recorder: schemaAuditRecorder{},
		Tracer: otel.Tracer("canonicalinspect-schema-test"), Now: time.Now, AuditTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Bind(): %v", err)
	}
	_, err = bound.Invoke(context.Background(), commercetool.Call{
		Tool:      definition.Ref,
		Metadata:  commercetool.CallMetadata{CallID: "call-1", AgentID: "test.agent", AgentVersion: "v1.0.0", AgentRunID: "run-1", BusinessTaskID: "task-1"},
		Arguments: json.RawMessage(`{"task_id":"task-1"}`),
	})
	if err != nil {
		t.Fatalf("Invoke() output schema error = %v", err)
	}
}

type schemaPrincipalResolver struct{}

func (schemaPrincipalResolver) ResolvePrincipal(context.Context) (commercetool.Principal, error) {
	return commercetool.Principal{TenantID: "tenant-1", UserID: "user-1", Roles: []string{"listingkit_operator"}}, nil
}

type schemaAuthorizer struct{}

func (schemaAuthorizer) Authorize(context.Context, commercetool.Principal, commercetool.PermissionRequirement) error {
	return nil
}

type schemaAuditRecorder struct{}

func (schemaAuditRecorder) RecordToolCall(context.Context, commercetool.AuditRecord) error {
	return nil
}
