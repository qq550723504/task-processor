package grsaitext

import (
	"encoding/json"
	"sort"

	"task-processor/internal/agent"
)

// This is a model-input view for the title consumer, not a new Tool output or
// fact owner. Original outputs remain unchanged in durable run history. Six KiB
// per observation leaves room for eight tool observations, maximally escaped
// feedback, binding/trace metadata and the existing 128 KiB transport envelope.
const promptEvidenceBytes = 6 << 10

type titleEvidenceView struct {
	Projection     string                     `json:"projection"`
	OriginalSHA256 string                     `json:"original_sha256"`
	Complete       bool                       `json:"complete"`
	Evidence       map[string]json.RawMessage `json:"evidence"`
	OmittedFields  []string                   `json:"omitted_fields"`
}

// EvidenceForPrompt is shared with deterministic validation so fields omitted
// from model input cannot later be treated as evidence the model observed.
func EvidenceForPrompt(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	if evidenceFits(raw) {
		return raw, nil
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil || root == nil || len(root) > 32 {
		return nil, agent.ErrInvalid
	}
	var snapshot map[string]json.RawMessage
	if value, ok := root["snapshot"]; ok {
		if json.Unmarshal(value, &snapshot) != nil || snapshot == nil || len(snapshot) > 32 {
			return nil, agent.ErrInvalid
		}
	}
	view := titleEvidenceView{Projection: "title-evidence-v1", OriginalSHA256: agentTextHash(raw), Evidence: map[string]json.RawMessage{}, OmittedFields: []string{}}
	for field := range root {
		if field != "snapshot" {
			view.OmittedFields = append(view.OmittedFields, field)
		}
	}
	for field := range snapshot {
		view.OmittedFields = append(view.OmittedFields, "snapshot."+field)
	}
	sort.Strings(view.OmittedFields)
	selectedSnapshot := map[string]json.RawMessage{}
	// Include only entire authoritative field values. Oversized values are
	// explicitly omitted, never truncated or interpreted as absent facts.
	add := func(field string, value json.RawMessage, nested bool) {
		if len(value) == 0 || len(value) > promptEvidenceBytes {
			return
		}
		path := field
		if nested {
			path = "snapshot." + field
			selectedSnapshot[field] = value
			view.Evidence["snapshot"], _ = json.Marshal(selectedSnapshot)
		} else {
			view.Evidence[field] = value
		}
		previous := view.OmittedFields
		view.OmittedFields = make([]string, 0, len(previous))
		for _, omitted := range previous {
			if omitted != path {
				view.OmittedFields = append(view.OmittedFields, omitted)
			}
		}
		encoded, _ := json.Marshal(view)
		if !evidenceFits(encoded) {
			view.OmittedFields = previous
			if nested {
				delete(selectedSnapshot, field)
				if len(selectedSnapshot) == 0 {
					delete(view.Evidence, "snapshot")
				} else {
					view.Evidence["snapshot"], _ = json.Marshal(selectedSnapshot)
				}
			} else {
				delete(view.Evidence, field)
			}
		}
	}
	for _, field := range []string{"product_key", "catalog_version", "catalog_publication_id", "publication_id", "product_version", "target_platform", "source_identity", "lineage", "producer", "capture", "published_at", "asset_binding", "scope", "status", "input_rule_version", "marketplace", "disclosure"} {
		add(field, root[field], false)
	}
	for _, field := range []string{"sources", "title", "brand", "category_path", "description", "selling_points", "seo_keywords", "attributes", "specifications", "review", "warnings", "variants", "images"} {
		add(field, snapshot[field], true)
	}
	for _, field := range []string{"approved_assets", "reasons", "missing_facts", "warnings", "diagnostics"} {
		add(field, root[field], false)
	}
	encoded, err := json.Marshal(view)
	if err != nil || !evidenceFits(encoded) {
		return nil, agent.ErrInvalid
	}
	return encoded, nil
}

func evidenceFits(raw []byte) bool {
	// Prompt itself is a JSON string in the SDK request; count that escaping.
	encoded, err := json.Marshal(string(raw))
	return err == nil && len(encoded) <= promptEvidenceBytes
}
