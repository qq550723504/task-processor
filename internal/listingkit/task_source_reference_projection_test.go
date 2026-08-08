package listingkit

import (
	"encoding/json"
	"strings"
	"task-processor/internal/listingkit/core"
	"testing"
)

func TestBuildTaskListItemIncludesSourceReferenceForPendingTask(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID:     "task-source-reference",
		Status: core.TaskStatusPending,
		Request: &GenerateRequest{Source: &SourceReference{
			Key:      "crawler:1688:888",
			Type:     "crawler",
			Platform: "1688",
			ID:       "888",
			URL:      "https://detail.1688.com/offer/888.html",
		}},
	}

	item := buildTaskListItem(task)
	if item.SourceReference == nil {
		t.Fatal("source_reference = nil, want normalized source identity")
	}
	if item.SourceReference == task.Request.Source {
		t.Fatal("source_reference shares request pointer, want defensive copy")
	}
	if item.SourceReference.Key != "crawler:1688:888" ||
		item.SourceReference.Type != "crawler" ||
		item.SourceReference.Platform != "1688" ||
		item.SourceReference.ID != "888" ||
		item.SourceReference.URL != "https://detail.1688.com/offer/888.html" {
		t.Fatalf("source_reference = %+v, want normalized identity", item.SourceReference)
	}
	if item.SourceType != "crawler" {
		t.Fatalf("source_type = %q, want crawler fallback", item.SourceType)
	}
}

func TestBuildTaskListItemOmitsLegacySourceReference(t *testing.T) {
	t.Parallel()

	item := buildTaskListItem(&Task{
		ID:      "task-legacy",
		Status:  core.TaskStatusPending,
		Request: &GenerateRequest{Text: "legacy"},
	})
	if item.SourceReference != nil {
		t.Fatalf("source_reference = %+v, want nil for legacy task", item.SourceReference)
	}

	payload, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal task list item: %v", err)
	}
	if strings.Contains(string(payload), `"source_reference"`) {
		t.Fatalf("payload = %s, want source_reference omitted", payload)
	}
}
