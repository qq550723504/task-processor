package sourceevidenceinspect

import (
	"context"
	"encoding/json"
	"testing"

	"task-processor/internal/commercetool"
)

func TestRegistryOutputSchemaRejectsRawAndUnboundedFields(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"raw payload", func(o map[string]any) { o["raw_html"] = "private" }},
		{"nested metadata", func(o map[string]any) {
			o["source_identity"].(map[string]any)["metadata"] = map[string]any{"cookie": "private"}
		}},
		{"source URL", func(o map[string]any) { o["source_identity"].(map[string]any)["source_url"] = "https://private" }},
		{"warning text", func(o map[string]any) { o["warnings"].([]any)[0].(map[string]any)["message"] = "private" }},
		{"raw code", func(o map[string]any) { o["warnings"].([]any)[0].(map[string]any)["code"] = "private" }},
		{"guessed category", func(o map[string]any) { o["warnings"].([]any)[0].(map[string]any)["kind"] = "critical" }},
		{"raw reason", func(o map[string]any) { o["missing_facts"].([]any)[0].(map[string]any)["reason"] = "private" }},
		{"unapproved field", func(o map[string]any) { o["missing_facts"].([]any)[0].(map[string]any)["field"] = "private" }},
		{"invalid identifier", func(o map[string]any) { o["source_identity"].(map[string]any)["source_id"] = "private-token" }},
		{"other platform", func(o map[string]any) { o["source_identity"].(map[string]any)["source_platform"] = "unreviewed" }},
		{"oversized warnings", func(o map[string]any) {
			a := make([]any, 257)
			for i := range a {
				a[i] = map[string]any{"kind": "source_warning"}
			}
			o["warnings"] = a
		}},
		{"false raw disclosure", func(o map[string]any) { o["disclosure"].(map[string]any)["raw_text_returned"] = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _, _, agent, deps, _ := invocationFixture(t)
			raw, err := Project(projectionFixture())
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			_ = json.Unmarshal(raw, &payload)
			tc.change(payload)
			raw, _ = json.Marshal(payload)
			executor := commercetool.ExecutorFunc(func(context.Context, commercetool.ExecutionEnvelope, json.RawMessage) (commercetool.ExecutionResult, error) {
				return commercetool.ExecutionResult{Output: raw}, nil
			})
			registry, err := commercetool.NewRegistry(commercetool.Tool{Definition: Definition(), Executor: executor})
			if err != nil {
				t.Fatal(err)
			}
			agent.AllowedTools = []commercetool.ToolRef{Definition().Ref}
			bound, err := registry.Bind(agent, deps)
			if err != nil {
				t.Fatal(err)
			}
			result, err := bound.Invoke(ctx, commercetool.Call{Tool: Definition().Ref, Metadata: callMetadata(), Arguments: json.RawMessage(`{"product_key":"product","catalog_version":"1"}`)})
			if commercetool.CodeOf(err) != commercetool.ErrorOutputInvalid || len(result.Output) != 0 {
				t.Fatalf("unsafe schema accepted: %s %v", result.Output, err)
			}
		})
	}
}

func TestSchemaAccessorsDoNotExposeMutableSharedBytes(t *testing.T) {
	for _, access := range []func() json.RawMessage{InputSchema, OutputSchema} {
		before := string(access())
		v := access()
		v[0] = '!'
		if string(access()) != before {
			t.Fatal("schema is mutable")
		}
	}
}
