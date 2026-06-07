package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetTaskRevisionHistoryDetailReturnsMatchedRecord(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	spuName := "Snapshot Bottle"
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-1",
			SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
				StoreID:          903,
				Site:             "GB",
				Strategy:         "country",
				Reason:           "根据任务国家信息命中了对应店铺。",
				MatchedRuleKinds: []string{"country"},
				MatchedProfileID: 17,
				ResolvedAt:       now,
			},
			Result: &ListingKitResult{
				TaskID: "task-history-detail-1",
				Shein: &SheinPackage{
					SpuName:       "Current Bottle",
					ProductNameEn: "Current Bottle",
				},
				RevisionHistory: []ListingKitRevisionRecord{
					{RevisionID: "rev-1", Platform: "shein", UpdatedAt: now.Add(-time.Minute)},
					{
						RevisionID: "rev-2",
						Platform:   "shein",
						UpdatedAt:  now,
						Reason:     "manual adjustment",
						EditorContext: &SheinEditorContext{
							RevisionSkeleton: &SheinEditorRevisionSkeleton{
								Platform: "shein",
								Actor:    "desktop-client",
								Reason:   "manual adjustment",
								Shein: &SheinRevisionInput{
									SpuName: &spuName,
								},
							},
						},
					},
				},
				RevisionHistoryTotal: 2,
			},
		},
	}
	svc := &service{repo: repo}

	detail, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-1", "rev-2", nil)
	if err != nil {
		t.Fatalf("get history detail: %v", err)
	}
	if detail.Record == nil || detail.Record.RevisionID != "rev-2" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Record.StoreResolution == nil || detail.Record.StoreResolution.StoreID != 903 {
		t.Fatalf("store resolution = %+v, want snapshot-backed detail", detail.Record.StoreResolution)
	}
	if detail.Record.StoreResolution.MatchedProfileID != 17 || detail.Record.StoreResolution.ResolvedAt == "" {
		t.Fatalf("store resolution = %+v, want audit metadata", detail.Record.StoreResolution)
	}
	if detail.Navigation == nil || detail.Navigation.PrevRevisionID != "rev-1" || detail.Navigation.NextRevisionID != "" {
		t.Fatalf("navigation = %+v", detail.Navigation)
	}
	if detail.RestorePayload == nil || detail.RestorePayload.Core == nil || detail.RestorePayload.Core.Draft == nil || detail.RestorePayload.Core.Draft.Shein == nil || detail.RestorePayload.Core.Draft.Shein.SpuName == nil || *detail.RestorePayload.Core.Draft.Shein.SpuName != spuName {
		t.Fatalf("restore payload = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Core.Draft.Reason != "restore: manual adjustment" {
		t.Fatalf("restore reason = %q", detail.RestorePayload.Core.Draft.Reason)
	}
	if detail.RestorePayload.Core.RevisionPayload == nil || detail.RestorePayload.Core.RevisionPayload.RestoreFromRevisionID != "rev-2" {
		t.Fatalf("restore payload = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Core.Context == nil || detail.RestorePayload.Core.Context.SourceRevisionID != "rev-2" {
		t.Fatalf("restore context = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Presentation == nil || detail.RestorePayload.Presentation.Scene != revisionPresentationSceneRestorePreview || detail.RestorePayload.Presentation.Messages == nil || detail.RestorePayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("restore messages = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Core.Safety == nil || !detail.RestorePayload.Core.Safety.CanRestore {
		t.Fatalf("restore safety = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Presentation == nil || detail.RestorePayload.Presentation.SummaryCard == nil || detail.RestorePayload.Presentation.SummaryCard.PrimaryAction == "" {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
	if len(detail.RestorePayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
}

func TestGetTaskRevisionHistoryDetailSupportsLegacyRecordID(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	legacy := ListingKitRevisionRecord{Platform: "shein", UpdatedAt: now}
	legacyID := revisionHistoryRecordID(legacy, 0)
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-2",
			Result: &ListingKitResult{
				TaskID:          "task-history-detail-2",
				RevisionHistory: []ListingKitRevisionRecord{legacy},
			},
		},
	}
	svc := &service{repo: repo}

	detail, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-2", legacyID, nil)
	if err != nil {
		t.Fatalf("get history detail: %v", err)
	}
	if detail.Record == nil || detail.Record.RevisionID != legacyID {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Navigation != nil {
		t.Fatalf("navigation = %+v", detail.Navigation)
	}
}

func TestGetTaskRevisionHistoryDetailReturnsNotFound(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-3",
			Result: &ListingKitResult{
				TaskID:          "task-history-detail-3",
				RevisionHistory: []ListingKitRevisionRecord{{RevisionID: "rev-1"}},
			},
		},
	}
	svc := &service{repo: repo}

	_, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-3", "missing", nil)
	if !errors.Is(err, ErrRevisionHistoryRecordNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrRevisionHistoryRecordNotFound)
	}
}

func TestGetTaskRevisionHistoryDetailBuildsRestoreDraftFromEditorContext(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-4",
			Result: &ListingKitResult{
				TaskID: "task-history-detail-4",
				RevisionHistory: []ListingKitRevisionRecord{
					{
						RevisionID: "rev-ctx",
						Platform:   "shein",
						UpdatedAt:  now,
						EditorContext: &SheinEditorContext{
							Basics: &SheinEditorBasicsContext{
								SpuName:       "Fallback Bottle",
								ProductNameEn: "Fallback Bottle",
								BrandName:     "Fallback Brand",
							},
						},
					},
				},
			},
		},
	}
	svc := &service{repo: repo}

	detail, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-4", "rev-ctx", nil)
	if err != nil {
		t.Fatalf("get history detail: %v", err)
	}
	if detail.RestorePayload == nil || detail.RestorePayload.Core == nil || detail.RestorePayload.Core.Draft == nil || detail.RestorePayload.Core.Draft.Shein == nil {
		t.Fatalf("restore payload = %+v", detail.RestorePayload)
	}
	if detail.Navigation != nil {
		t.Fatalf("navigation = %+v", detail.Navigation)
	}
	if detail.RestorePayload.Core.Draft.Shein.SpuName == nil || *detail.RestorePayload.Core.Draft.Shein.SpuName != "Fallback Bottle" {
		t.Fatalf("restore draft = %+v", detail.RestorePayload.Core.Draft)
	}
}

func TestGetTaskRevisionHistoryDetailBuildsComparePreview(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	prevName := "Old Bottle"
	currentName := "New Bottle"
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-5",
			Result: &ListingKitResult{
				TaskID: "task-history-detail-5",
				Shein: &SheinPackage{
					SpuName:       currentName,
					ProductNameEn: currentName,
				},
				RevisionHistory: []ListingKitRevisionRecord{
					{
						RevisionID: "rev-1",
						Platform:   "shein",
						UpdatedAt:  now.Add(-time.Minute),
						EditorContext: &SheinEditorContext{
							RevisionSkeleton: &SheinEditorRevisionSkeleton{
								Platform: "shein",
								Shein: &SheinRevisionInput{
									SpuName: &prevName,
								},
							},
						},
					},
					{
						RevisionID: "rev-2",
						Platform:   "shein",
						UpdatedAt:  now,
						EditorContext: &SheinEditorContext{
							RevisionSkeleton: &SheinEditorRevisionSkeleton{
								Platform: "shein",
								Shein: &SheinRevisionInput{
									SpuName: &currentName,
								},
							},
						},
					},
				},
			},
		},
	}
	svc := &service{repo: repo}

	detail, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-5", "rev-2", &RevisionHistoryDetailQuery{CompareTo: "prev"})
	if err != nil {
		t.Fatalf("get history detail: %v", err)
	}
	if detail.RestorePayload == nil || detail.RestorePayload.Core == nil || detail.RestorePayload.Core.Compare == nil || detail.RestorePayload.Core.Compare.CompareRevisionID != "rev-1" {
		t.Fatalf("compare preview = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Core.Compare.DiffPreview == nil || detail.RestorePayload.Core.Compare.DiffPreview.ChangeCount == 0 {
		t.Fatalf("compare preview diff = %+v", detail.RestorePayload.Core.Compare)
	}
	if detail.RestorePayload.Core.Context == nil || detail.RestorePayload.Core.Context.TargetRevisionID != "rev-1" {
		t.Fatalf("restore context = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Presentation == nil || detail.RestorePayload.Presentation.SummaryCard == nil || detail.RestorePayload.Presentation.SummaryCard.Status != "ready_with_warnings" {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
	if len(detail.RestorePayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
}

func TestGetTaskRevisionHistoryDetailReturnsNotFoundForMissingCompareTarget(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-6",
			Result: &ListingKitResult{
				TaskID:          "task-history-detail-6",
				RevisionHistory: []ListingKitRevisionRecord{{RevisionID: "rev-1", Platform: "shein", UpdatedAt: now}},
			},
		},
	}
	svc := &service{repo: repo}

	_, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-6", "rev-1", &RevisionHistoryDetailQuery{CompareTo: "next"})
	if !errors.Is(err, ErrRevisionHistoryCompareTargetNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrRevisionHistoryCompareTargetNotFound)
	}
}

func TestGetTaskRevisionHistoryDetailBuildsComparePreviewAgainstCurrent(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	oldName := "Historic Bottle"
	currentName := "Current Bottle"
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-7",
			Result: &ListingKitResult{
				TaskID: "task-history-detail-7",
				Shein: &SheinPackage{
					SpuName:       currentName,
					ProductNameEn: currentName,
				},
				RevisionHistory: []ListingKitRevisionRecord{
					{
						RevisionID: "rev-1",
						Platform:   "shein",
						UpdatedAt:  now,
						EditorContext: &SheinEditorContext{
							RevisionSkeleton: &SheinEditorRevisionSkeleton{
								Platform: "shein",
								Shein: &SheinRevisionInput{
									SpuName:       &oldName,
									ProductNameEn: &oldName,
								},
							},
						},
					},
				},
			},
		},
	}
	svc := &service{repo: repo}

	detail, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-7", "rev-1", &RevisionHistoryDetailQuery{CompareTo: "current"})
	if err != nil {
		t.Fatalf("get history detail: %v", err)
	}
	if detail.RestorePayload == nil || detail.RestorePayload.Core == nil || detail.RestorePayload.Core.Compare == nil || detail.RestorePayload.Core.Compare.CompareRevisionID != "current" {
		t.Fatalf("compare preview = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Core.Compare.RelationLabel != "当前版本" {
		t.Fatalf("compare preview = %+v", detail.RestorePayload.Core.Compare)
	}
	if detail.RestorePayload.Core.Compare.DiffPreview == nil || detail.RestorePayload.Core.Compare.DiffPreview.ChangeCount == 0 {
		t.Fatalf("compare preview diff = %+v", detail.RestorePayload.Core.Compare)
	}
	if detail.RestorePayload.Core.Context == nil || detail.RestorePayload.Core.Context.TargetRevisionID != "current" {
		t.Fatalf("restore context = %+v", detail.RestorePayload)
	}
}

func TestGetTaskRevisionHistoryDetailBuildsRestoreSafetyWarnings(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	currentName := "Current Bottle"
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-8",
			Result: &ListingKitResult{
				TaskID: "task-history-detail-8",
				Shein: &SheinPackage{
					SpuName:       currentName,
					ProductNameEn: currentName,
					ReviewNotes:   []string{"confirm category again"},
				},
				RevisionHistory: []ListingKitRevisionRecord{
					{
						RevisionID:             "rev-restore",
						Platform:               "shein",
						ActionType:             RevisionActionTypeRestore,
						RestoredFromRevisionID: "rev-old",
						UpdatedAt:              now,
						EditorContext: &SheinEditorContext{
							RevisionSkeleton: &SheinEditorRevisionSkeleton{
								Platform: "shein",
								Shein:    &SheinRevisionInput{},
							},
						},
					},
				},
			},
		},
	}
	svc := &service{repo: repo}

	detail, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-8", "rev-restore", &RevisionHistoryDetailQuery{CompareTo: "current"})
	if err != nil {
		t.Fatalf("get history detail: %v", err)
	}
	if detail.RestorePayload == nil || detail.RestorePayload.Core == nil || detail.RestorePayload.Core.Safety == nil || !detail.RestorePayload.Core.Safety.CanRestore {
		t.Fatalf("restore safety = %+v", detail.RestorePayload)
	}
	if len(detail.RestorePayload.Core.Safety.RestoreWarnings) == 0 {
		t.Fatalf("restore warnings = %+v", detail.RestorePayload.Core.Safety)
	}
	if detail.RestorePayload.Presentation == nil || detail.RestorePayload.Presentation.SummaryCard == nil || detail.RestorePayload.Presentation.SummaryCard.Status != "ready_with_warnings" {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
	if len(detail.RestorePayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
}

func TestGetTaskRevisionHistoryDetailMarksRestoreUnsupportedWithoutCurrentShein(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-history-detail-9",
			Result: &ListingKitResult{
				TaskID: "task-history-detail-9",
				RevisionHistory: []ListingKitRevisionRecord{
					{
						RevisionID: "rev-1",
						Platform:   "shein",
						UpdatedAt:  now,
						EditorContext: &SheinEditorContext{
							RevisionSkeleton: &SheinEditorRevisionSkeleton{
								Platform: "shein",
								Shein: &SheinRevisionInput{
									SpuName: stringPointerOrNil("Fallback Bottle"),
								},
							},
						},
					},
				},
			},
		},
	}
	svc := &service{repo: repo}

	detail, err := svc.GetTaskRevisionHistoryDetail(context.Background(), "task-history-detail-9", "rev-1", nil)
	if err != nil {
		t.Fatalf("get history detail: %v", err)
	}
	if detail.RestorePayload == nil || detail.RestorePayload.Core == nil || detail.RestorePayload.Core.Safety == nil || detail.RestorePayload.Core.Safety.CanRestore {
		t.Fatalf("restore safety = %+v", detail.RestorePayload)
	}
	if len(detail.RestorePayload.Core.Safety.RestoreWarnings) == 0 {
		t.Fatalf("restore warnings = %+v", detail.RestorePayload.Core.Safety)
	}
	if detail.RestorePayload.Presentation == nil || detail.RestorePayload.Presentation.SummaryCard == nil || detail.RestorePayload.Presentation.SummaryCard.Status != "blocked" {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
	if len(detail.RestorePayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore overview = %+v", detail.RestorePayload)
	}
	if detail.RestorePayload.Presentation.Messages == nil || detail.RestorePayload.Presentation.Messages.WarningTitle == "" {
		t.Fatalf("restore messages = %+v", detail.RestorePayload)
	}
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}
