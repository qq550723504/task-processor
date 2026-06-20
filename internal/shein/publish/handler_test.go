package publish

import (
	"testing"

	"task-processor/internal/listingruntime"
)

func TestStoreDraftEnabled(t *testing.T) {
	enableDraft := true
	disableDraft := false

	if !storeDraftEnabled(&listingruntime.StoreInfo{EnableDraft: &enableDraft}) {
		t.Fatal("expected enable_draft=true to route into draft mode")
	}
	if storeDraftEnabled(&listingruntime.StoreInfo{EnableDraft: &disableDraft}) {
		t.Fatal("expected enable_draft=false to keep publish mode")
	}
	if storeDraftEnabled(&listingruntime.StoreInfo{}) {
		t.Fatal("expected missing enable_draft to keep publish mode")
	}
	if storeDraftEnabled(nil) {
		t.Fatal("expected nil store info to keep publish mode")
	}
}
