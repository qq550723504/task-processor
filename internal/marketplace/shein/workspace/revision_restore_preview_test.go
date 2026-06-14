package workspace

import "testing"

func TestBuildHistoryNavigationOmitsEmptyNavigation(t *testing.T) {
	if got := BuildHistoryNavigation("", ""); got != nil {
		t.Fatalf("navigation = %#v, want nil", got)
	}

	got := BuildHistoryNavigation("prev", "next")
	if got == nil || got.PrevRevisionID != "prev" || got.NextRevisionID != "next" {
		t.Fatalf("navigation = %#v", got)
	}
}

func TestBuildRestorePreviewPayloadBuildsCoreAndRebuildsCompare(t *testing.T) {
	draft := &EditorRevisionSkeleton{Platform: "shein"}
	revisionPayload := "payload"
	context := "context"
	safety := "safety"
	compare := "compare"
	presentation := "presentation"

	payload := BuildRestorePreviewPayload(draft, &revisionPayload, &context, &safety, &compare, &presentation)
	if payload == nil || payload.Core == nil {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.Core.Draft != draft || *payload.Core.RevisionPayload != revisionPayload || *payload.Presentation != presentation {
		t.Fatalf("payload fields = %#v", payload)
	}

	replacementCompare := "replacement"
	rebuilt := RebuildRestorePreviewPayload(payload, &replacementCompare)
	if rebuilt == nil || rebuilt.Core == nil || rebuilt.Core.Compare == nil || *rebuilt.Core.Compare != replacementCompare {
		t.Fatalf("rebuilt compare = %#v", rebuilt)
	}
}
