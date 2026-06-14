package workspace

import "testing"

func TestResolveHistoryCompareTargetSupportsRelativeAndExplicitTargets(t *testing.T) {
	records := []HistoryCompareRecord{
		{RevisionID: "rev-1"},
		{RevisionID: "rev-2"},
		{RevisionID: "rev-3"},
	}

	record, index, label, ok := ResolveHistoryCompareTarget(records, 1, "prev")
	if !ok || index != 0 || label != "上一条" || record.RevisionID != "rev-1" {
		t.Fatalf("prev = (%#v, %d, %q, %v)", record, index, label, ok)
	}

	record, index, label, ok = ResolveHistoryCompareTarget(records, 1, "rev-3")
	if !ok || index != 2 || label != "指定记录" || record.RevisionID != "rev-3" {
		t.Fatalf("explicit = (%#v, %d, %q, %v)", record, index, label, ok)
	}
}

func TestBuildCurrentHistoryComparePreviewBuildsDiff(t *testing.T) {
	recordTitle := "record"
	currentTitle := "current"

	preview := BuildCurrentHistoryComparePreview(
		&HistoryCompareRecord{
			RevisionID: "rev-1",
			Draft:      &EditorRevisionSkeleton{Shein: &RevisionInput{SpuName: &recordTitle}},
		},
		&EditorRevisionSkeleton{Shein: &RevisionInput{SpuName: &currentTitle}},
	)

	if preview == nil || preview.CompareTo != "current" || preview.CompareRevisionID != "current" {
		t.Fatalf("preview = %#v", preview)
	}
	if preview.DiffPreview == nil || preview.DiffPreview.ChangeCount != 1 {
		t.Fatalf("diff = %#v", preview.DiffPreview)
	}
}
