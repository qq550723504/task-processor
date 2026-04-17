package shein

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSuccessPayloadForApplyFlow(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		SpuName:       "Bottle",
		ProductNameEn: "Bottle",
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{SupplierCode: "SKC-1"}},
		},
	}
	readiness := &SubmitReadiness[string, string]{
		Status:  "ready",
		Ready:   true,
		Summary: []string{"关键字段已满足提交前要求"},
	}

	status := BuildSuccessStatusSummary(pkg, readiness)
	messages := BuildSuccessMessages(SuccessModeApply, "更新 SHEIN 资料", 2, "", status)
	nextActions := BuildSuccessNextActions(pkg)
	view := BuildSuccessRecommendedView(SuccessModeApply, status)
	followUp := BuildSuccessFollowUpOverview(
		SuccessModeApply,
		status,
		messages,
		&SuccessFollowUpChecklist[string]{Recommended: []string{"manual_notes"}},
		nextActions,
	)
	card := BuildSuccessSummaryCard(
		SuccessModeApply,
		"更新 SHEIN 资料",
		"",
		2,
		messages,
		&RevisionDiffPreview{
			ChangeCount: 2,
			Changes: []RevisionFieldChange{
				{Label: "SPU 名称"},
				{Label: "品牌"},
			},
		},
		status,
		view,
		nextActions,
	)
	presentation := BuildSuccessPresentation(SceneApplySuccess, nextActions, messages, view, card)
	payload := BuildSuccessPayload(
		SuccessModeApply,
		"edit",
		"更新 SHEIN 资料",
		"",
		"",
		2,
		status,
		presentation,
		&SuccessFollowUpChecklist[string]{Recommended: []string{"manual_notes"}},
		followUp,
		&EditorRevisionSkeleton{Platform: "shein"},
		&RevisionDiffPreview{ChangeCount: 2},
	)

	if payload == nil || payload.Mode != SuccessModeApply {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Presentation == nil || payload.Presentation.Scene != SceneApplySuccess {
		t.Fatalf("presentation = %+v", payload)
	}
	if payload.Core == nil || payload.Core.ChangeCount != 2 {
		t.Fatalf("core = %+v", payload)
	}
	if payload.Presentation.SummaryCard == nil || payload.Presentation.SummaryCard.Title == "" {
		t.Fatalf("summary card = %+v", payload)
	}
}

func TestBuildSuccessPayloadForRestoreFlow(t *testing.T) {
	t.Parallel()

	messages := BuildSuccessMessages(SuccessModeRestore, "恢复历史版本", 1, "rev-1", &SuccessStatusSummary{
		Status:      "ready",
		Subheadline: "关键字段已满足提交前要求",
	})
	view := BuildSuccessRecommendedView(SuccessModeRestore, &SuccessStatusSummary{Status: "ready"})
	card := BuildSuccessSummaryCard(
		SuccessModeRestore,
		"恢复历史版本",
		"恢复自 rev-1",
		1,
		messages,
		&RevisionDiffPreview{ChangeCount: 1},
		&SuccessStatusSummary{Status: "ready", Subheadline: "关键字段已满足提交前要求"},
		view,
		[]string{"继续提交流程"},
	)
	presentation := BuildSuccessPresentation(SceneRestoreSuccess, []string{"继续提交流程"}, messages, view, card)
	payload := BuildSuccessPayload[string](
		SuccessModeRestore,
		"restore",
		"恢复历史版本",
		"rev-1",
		"恢复自 rev-1",
		1,
		&SuccessStatusSummary{Status: "ready"},
		presentation,
		nil,
		nil,
		nil,
		&RevisionDiffPreview{ChangeCount: 1},
	)

	if payload == nil || payload.Mode != SuccessModeRestore {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Core == nil || payload.Core.SourceRevisionID != "rev-1" {
		t.Fatalf("core = %+v", payload)
	}
	if payload.Presentation == nil || payload.Presentation.Scene != SceneRestoreSuccess {
		t.Fatalf("presentation = %+v", payload)
	}
}
