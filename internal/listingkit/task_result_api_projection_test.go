package listingkit

import (
	"encoding/json"
	"testing"
)

func TestTaskResultJSONContainsSourceReferenceWithoutRequest(t *testing.T) {
	task := &Task{
		ID:       "task-detail-source-json",
		TenantID: "tenant-a",
		Request: &GenerateRequest{
			Source: &SourceReference{
				Key:      "crawler:1688:888",
				Type:     "crawler",
				Platform: "1688",
				ID:       "888",
				URL:      "https://detail.1688.com/offer/888.html",
			},
		},
	}

	payload, err := json.Marshal(buildTaskResult(task, nil))
	if err != nil {
		t.Fatalf("marshal task result: %v", err)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal task result: %v", err)
	}
	if _, ok := body["request"]; ok {
		t.Fatal("task result exposes persisted request payload")
	}

	var source map[string]string
	if err := json.Unmarshal(body["source_reference"], &source); err != nil {
		t.Fatalf("unmarshal source_reference: %v", err)
	}
	for field, want := range map[string]string{
		"key":      "crawler:1688:888",
		"type":     "crawler",
		"platform": "1688",
		"id":       "888",
		"url":      "https://detail.1688.com/offer/888.html",
	} {
		if source[field] != want {
			t.Errorf("source_reference.%s = %q, want %q", field, source[field], want)
		}
	}
}
