package listingkit

import "testing"

func TestBuildRevisionApplySummaryCardDataUsesSaveSemantics(t *testing.T) {
	t.Parallel()

	appliedChanges := &RevisionDiffPreview{
		ChangeCount: 2,
		Changes: []RevisionFieldChange{
			{Label: "SPU 名称"},
			{Label: "品牌名"},
		},
	}

	data := buildRevisionApplySummaryCardData("更新 SHEIN 资料", 2, &RevisionResultMessages{
		Description: "本次已保存 2 个字段的更新。",
	}, appliedChanges, &ListingKitResult{
		Shein: &SheinPackage{},
	})
	if data.Subtitle != "本次已保存 2 个字段的更新。" {
		t.Fatalf("subtitle = %q", data.Subtitle)
	}
	if data.PrimaryView == "" {
		t.Fatalf("primary view = %q", data.PrimaryView)
	}
	if len(data.Highlights) == 0 {
		t.Fatalf("highlights = %+v", data.Highlights)
	}
}

func TestBuildRevisionHistoryRestoreOverviewDataNormalizesHighlights(t *testing.T) {
	t.Parallel()

	record := &ListingKitRevisionRecord{
		Timeline: &ListingKitRevisionTimelineSummary{
			Headline:     "恢复历史版本",
			RelationText: "恢复自 rev-1",
		},
	}
	safety := &RevisionHistoryRestoreSafety{
		CanRestore:      true,
		RestoreWarnings: []string{"需要复查属性", "需要复查属性"},
	}
	comparePreview := &RevisionHistoryComparePreview{
		RelationLabel: "当前版本",
		DiffPreview: &RevisionDiffPreview{
			ChangeCount: 3,
		},
	}

	data := buildRevisionHistoryRestoreOverviewData(record, safety, comparePreview)
	if data.Status != "ready_with_warnings" {
		t.Fatalf("status = %q", data.Status)
	}
	if data.PrimaryAction != "恢复历史版本" {
		t.Fatalf("primary action = %q", data.PrimaryAction)
	}
	if len(data.NextActions) == 0 || len(data.Highlights) == 0 {
		t.Fatalf("overview data = %+v", data)
	}
}

func TestBuildRevisionSuccessFollowUpDataBuildsSharedPayload(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		Shein: &SheinPackage{
			ReviewNotes: []string{"需要人工备注确认"},
		},
	}
	summary := &RevisionStatusSummary{
		Status:      "ready_with_warnings",
		Subheadline: "保存后仍建议继续确认",
		NeedsReview: true,
	}
	messages := &RevisionResultMessages{
		Description: "本次已保存 1 个字段的更新。",
	}
	readinessProjection := buildRevisionSuccessReadinessProjection(result)

	followUp := buildRevisionSuccessFollowUpData(
		revisionSuccessModeApply,
		result,
		summary,
		messages,
		[]string{"处理人工备注"},
		readinessProjection,
	)
	if followUp == nil {
		t.Fatal("expected follow-up data")
	}
	if followUp.Overview == nil || followUp.Overview.Headline == "" {
		t.Fatalf("overview = %+v", followUp.Overview)
	}
	if followUp.SuggestedRevision == nil || followUp.SuggestedRevision.Reason != "follow-up after apply" {
		t.Fatalf("suggested revision = %+v", followUp.SuggestedRevision)
	}
}
