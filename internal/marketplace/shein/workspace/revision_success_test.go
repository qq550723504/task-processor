package workspace

import "testing"

func TestBuildSuccessPayloadClonesCoreData(t *testing.T) {
	status := &SuccessStatusSummary{Status: "ready", Highlights: []string{"one"}}
	checklist := &SuccessFollowUpChecklist[string]{Recommended: []string{"manual_notes"}}
	applied := &RevisionDiffPreview{
		ChangeCount: 1,
		Changes:     []RevisionFieldChange{{Label: "SPU 名称"}},
	}

	payload := BuildSuccessPayload(
		SuccessModeApply,
		"edit",
		"更新 SHEIN 资料",
		"",
		"",
		1,
		status,
		BuildSuccessPresentation(SceneApplySuccess, []string{"继续提交流程"}, nil, nil, nil),
		checklist,
		&SuccessFollowUpOverview{NextActions: []string{"继续提交流程"}},
		&EditorRevisionSkeleton{Platform: "shein"},
		applied,
	)

	if payload == nil || payload.Core == nil || payload.Presentation == nil {
		t.Fatalf("payload = %#v", payload)
	}
	status.Highlights[0] = "changed"
	checklist.Recommended[0] = "changed"
	applied.Changes[0].Label = "changed"
	if payload.Core.StatusSummary.Highlights[0] != "one" {
		t.Fatalf("status was not cloned = %#v", payload.Core.StatusSummary)
	}
	if payload.Core.FollowUpChecklist.Recommended[0] != "manual_notes" {
		t.Fatalf("checklist was not cloned = %#v", payload.Core.FollowUpChecklist)
	}
	if payload.Core.AppliedChanges.Changes[0].Label != "SPU 名称" {
		t.Fatalf("diff was not cloned = %#v", payload.Core.AppliedChanges)
	}
}
