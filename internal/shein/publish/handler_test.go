package publish

import (
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
)

func TestStoreDraftEnabled(t *testing.T) {
	enableDraft := true
	disableDraft := false

	if !storeDraftEnabled(&managementapi.StoreRespDTO{EnableDraft: &enableDraft}) {
		t.Fatal("expected enable_draft=true to route into draft mode")
	}
	if storeDraftEnabled(&managementapi.StoreRespDTO{EnableDraft: &disableDraft}) {
		t.Fatal("expected enable_draft=false to keep publish mode")
	}
	if storeDraftEnabled(&managementapi.StoreRespDTO{}) {
		t.Fatal("expected missing enable_draft to keep publish mode")
	}
	if storeDraftEnabled(nil) {
		t.Fatal("expected nil store info to keep publish mode")
	}
}
