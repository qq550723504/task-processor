package assetinspect

import (
	"encoding/json"

	"task-processor/internal/product/catalog/tools/canonicalinspect"
)

func InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["product_key","catalog_version","target_platform"],"properties":{"product_key":{"type":"string","minLength":1,"maxLength":128},"catalog_version":{"type":"string","pattern":"^[1-9][0-9]{0,18}$"},"target_platform":{"type":"string","minLength":1,"maxLength":128}}}`)
}

// Reuse the existing approved Catalog projection schema; this tool adds only
// exact Asset facts. Each call returns a detached schema document.
func OutputSchema() json.RawMessage {
	var schema map[string]any
	if err := json.Unmarshal(canonicalinspect.OutputSchema(), &schema); err != nil {
		panic(err)
	}
	properties := schema["properties"].(map[string]any)
	var extra map[string]any
	if err := json.Unmarshal([]byte(`{
	"asset_binding":{"type":"object","additionalProperties":false,"required":["target_platform","source_snapshot_version"],"properties":{"target_platform":{"type":"string","minLength":1,"maxLength":128},"source_snapshot_version":{"type":"string","pattern":"^[1-9][0-9]{0,18}$"}}},
	"approved_assets":{"type":"array","minItems":1,"items":{"type":"object","additionalProperties":false,"required":["id","run_id","plan_revision","slot_id","attempt","role","url"],"properties":{"id":{"type":"string","minLength":1,"maxLength":128},"run_id":{"type":"string","minLength":1,"maxLength":128},"plan_revision":{"type":"integer","minimum":1},"slot_id":{"type":"string","minLength":1,"maxLength":128},"attempt":{"type":"integer","minimum":1},"role":{"type":"string","enum":["design","main","white_background","gallery"]},"url":{"type":"string","minLength":1},"source_asset_id":{"type":"string","maxLength":128},"width":{"type":"integer","minimum":0},"height":{"type":"integer","minimum":0},"operations":{"type":"array","items":{"type":"string"}}}}}}`), &extra); err != nil {
		panic(err)
	}
	for key, value := range extra {
		properties[key] = value
	}
	schema["required"] = append(schema["required"].([]any), "asset_binding", "approved_assets")
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return raw
}
