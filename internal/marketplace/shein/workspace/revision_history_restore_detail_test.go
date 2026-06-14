package workspace

import "testing"

func TestBuildHistoryRestoreSafetyBlocksWhenCurrentPackageMissing(t *testing.T) {
	safety := BuildHistoryRestoreSafety(
		&HistoryRestoreStateInput{},
		&HistoryRestoreRecordInput{Platform: "shein"},
		&EditorRevisionSkeleton{Shein: &RevisionInput{}},
		nil,
	)

	if safety == nil || safety.CanRestore {
		t.Fatalf("safety = %#v", safety)
	}
	if len(safety.RestoreWarnings) != 1 || safety.RestoreWarnings[0] == "" {
		t.Fatalf("warnings = %#v", safety.RestoreWarnings)
	}
}

func TestBuildHistoryRestoreDetailDataBuildsReadyProjection(t *testing.T) {
	payload := "payload"
	compare := "compare"
	spuName := "SPU-1"
	record := &HistoryRestoreRecordInput{
		RevisionID: "rev-1",
		Platform:   "shein",
		ActionType: "apply",
		Timeline: &HistoryRestoreTimeline{
			Headline:     "历史版本",
			RelationText: "来自第 1 版",
		},
	}
	state := &HistoryRestoreStateInput{
		HasCurrentPackage:     true,
		CategoryResolved:      true,
		AttributeResolved:     true,
		SaleAttributeResolved: true,
	}

	detail := BuildHistoryRestoreDetailData(
		record,
		state,
		&EditorRevisionSkeleton{Shein: &RevisionInput{SpuName: &spuName}},
		&payload,
		"restore_from_revision_id",
		"shein",
		"restore",
		&HistoryRestoreCompareInput{CompareTo: "current", CompareRevisionID: "current", RelationLabel: "当前版本", ChangeCount: 2},
		&compare,
	)

	if detail == nil || detail.Context == nil || detail.Safety == nil || detail.Overview == nil || detail.Messages == nil {
		t.Fatalf("detail = %#v", detail)
	}
	if !detail.Safety.CanRestore || detail.Overview.Status != "ready" || detail.Messages.ConfirmLabel != "恢复历史版本" {
		t.Fatalf("detail state = %#v", detail)
	}
	if detail.RevisionPayload == nil || *detail.RevisionPayload != payload || detail.Compare == nil || *detail.Compare != compare {
		t.Fatalf("detail payload = %#v", detail)
	}
}

func TestBuildHistoryRestorePresentationDataRecommendsInspectionForWarnings(t *testing.T) {
	context := &HistoryRestoreContext{SourceRevisionID: "rev-1", TargetLabel: "当前版本"}
	safety := &HistoryRestoreSafety{
		CanRestore:      true,
		RestoreWarnings: []string{"当前版本的类目骨架仍未完全解析，恢复后建议重新确认 category_id 和 product_type_id"},
	}

	data := BuildHistoryRestorePresentationData(
		&HistoryRestoreRecordInput{Platform: "shein"},
		context,
		safety,
		&HistoryRestoreCompareInput{CompareTo: "current", ChangeCount: 0},
	)

	if data == nil || data.RecommendedView == nil {
		t.Fatalf("presentation = %#v", data)
	}
	if data.Status != "ready_with_warnings" || data.RecommendedView.View != "inspection" {
		t.Fatalf("presentation state = %#v", data)
	}
	if len(data.NextActions) == 0 || data.NextActions[0] == "" {
		t.Fatalf("next actions = %#v", data.NextActions)
	}
}
