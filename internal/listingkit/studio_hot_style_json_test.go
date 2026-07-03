package listingkit

import (
	"encoding/json"
	"testing"
)

func TestStudioHotStyleReferenceFieldsSerializeWhenCleared(t *testing.T) {
	t.Run("session", func(t *testing.T) {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(mustMarshalStudioHotStyleJSON(t, SheinStudioSession{
			HotStyleReferenceImageURLs: SheinStudioStringList{},
			HotStyleReferenceBrief:     "",
			HotStyleReferencePrompt:    "",
		}), &payload); err != nil {
			t.Fatalf("unmarshal session json: %v", err)
		}
		assertStudioHotStyleReferenceKeys(t, payload)
	})

	t.Run("batch", func(t *testing.T) {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(mustMarshalStudioHotStyleJSON(t, StudioBatchRecord{
			HotStyleReferenceImageURLs: SheinStudioStringList{},
			HotStyleReferenceBrief:     "",
			HotStyleReferencePrompt:    "",
		}), &payload); err != nil {
			t.Fatalf("unmarshal batch json: %v", err)
		}
		assertStudioHotStyleReferenceKeys(t, payload)
	})

	t.Run("batch list item", func(t *testing.T) {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(mustMarshalStudioHotStyleJSON(t, SheinStudioBatchListItem{
			HotStyleReferenceImageURLs: []string{},
			HotStyleReferenceBrief:     "",
			HotStyleReferencePrompt:    "",
		}), &payload); err != nil {
			t.Fatalf("unmarshal batch list item json: %v", err)
		}
		assertStudioHotStyleReferenceKeys(t, payload)
	})
}

func mustMarshalStudioHotStyleJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return body
}

func assertStudioHotStyleReferenceKeys(
	t *testing.T,
	payload map[string]json.RawMessage,
) {
	t.Helper()
	for _, key := range []string{
		"hot_style_reference_image_urls",
		"hot_style_reference_brief",
		"hot_style_reference_prompt",
	} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected %s to be present in %#v", key, payload)
		}
	}
}
