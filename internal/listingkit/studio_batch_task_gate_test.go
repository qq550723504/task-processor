package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/catalog/canonical"
)

type stubStudioBatchBaselineReadinessChecker struct {
	entries map[string]*SDSBaselineCacheEntry
	err     error
	calls   int
}

func (s *stubStudioBatchBaselineReadinessChecker) CheckStudioBatchBaselineReadiness(_ context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineCacheEntry, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.entries == nil {
		return nil, nil
	}
	return s.entries[sdsBaselineKey(query.TenantID, query.BaselineOptions())], nil
}

type stubStudioBatchStoreValidator struct {
	results map[int64]studioBatchStoreValidationResult
	err     error
	calls   int
}

func (s *stubStudioBatchStoreValidator) ValidateStudioBatchStore(_ context.Context, _ string, storeID int64) (studioBatchStoreValidationResult, error) {
	s.calls++
	if s.err != nil {
		return studioBatchStoreValidationResult{}, s.err
	}
	return s.results[storeID], nil
}

func TestStudioBatchTaskGateEvaluateReasonCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mutate     func(*studioBatchTaskGateEvaluation)
		wantReason string
	}{
		{
			name: "design_not_found",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.DesignsByID = map[string]StudioMaterializedDesignRecord{}
			},
			wantReason: "design_not_found",
		},
		{
			name: "design_target_mismatch",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.Design.BatchID = "other-batch"
			},
			wantReason: "design_target_mismatch",
		},
		{
			name: "design_not_approved",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.Design.ReviewStatus = StudioMaterializedDesignReviewStatusRejected
				eval.DesignsByID["design-1"] = eval.Candidate.Design
			},
			wantReason: "design_not_approved",
		},
		{
			name: "design_image_missing",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.Design.ImageURL = ""
				eval.DesignsByID["design-1"] = eval.Candidate.Design
			},
			wantReason: "design_image_missing",
		},
		{
			name: "selection_not_in_batch",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.SelectionsByID = map[string]SheinStudioGroupedSelection{}
			},
			wantReason: "selection_not_in_batch",
		},
		{
			name: "selection_not_in_item",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.Item.SelectionIDs = SheinStudioStringList{"other-selection"}
			},
			wantReason: "selection_not_in_item",
		},
		{
			name: "selection_identity_incomplete",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.SelectionSnapshot.ParentProductID = 0
			},
			wantReason: "selection_identity_incomplete",
		},
		{
			name: "selection_variant_incompatible",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.SelectionSnapshot.Variants = []SheinStudioSelectionVariant{{
					VariantID:        3003,
					PrototypeGroupID: 9999,
					LayerID:          "layer-1",
					TemplateImageURL: "https://cdn.example.com/template.png",
					MaskImageURL:     "https://cdn.example.com/mask.png",
				}}
			},
			wantReason: "selection_variant_incompatible",
		},
		{
			name: "store_missing",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.SheinStoreID = 0
			},
			wantReason: "store_missing",
		},
		{
			name: "store_invalid",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.StoreValidator = &stubStudioBatchStoreValidator{results: map[int64]studioBatchStoreValidationResult{
					870: {Exists: false},
				}}
			},
			wantReason: "store_invalid",
		},
		{
			name: "store_not_available",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.StoreValidator = &stubStudioBatchStoreValidator{results: map[int64]studioBatchStoreValidationResult{
					870: {Exists: true, Valid: true, Available: false},
				}}
			},
			wantReason: "store_not_available",
		},
		{
			name: "baseline_missing",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.BaselineChecker = &stubStudioBatchBaselineReadinessChecker{}
			},
			wantReason: "baseline_missing",
		},
		{
			name: "baseline_not_ready",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.BaselineChecker = &stubStudioBatchBaselineReadinessChecker{entries: map[string]*SDSBaselineCacheEntry{
					baselineKeyForStudioBatchGateTest(eval): baselineEntryForStudioBatchGateTest(t, SDSBaselineValidationStatusBlocked, sdsBaselineSupportedVersion),
				}}
			},
			wantReason: "baseline_not_ready",
		},
		{
			name: "baseline_invalid",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				entry := baselineEntryForStudioBatchGateTest(t, SDSBaselineValidationStatusReady, sdsBaselineSupportedVersion)
				entry.CanonicalProductBase = nil
				eval.BaselineChecker = &stubStudioBatchBaselineReadinessChecker{entries: map[string]*SDSBaselineCacheEntry{
					baselineKeyForStudioBatchGateTest(eval): entry,
				}}
			},
			wantReason: "baseline_invalid",
		},
		{
			name: "baseline_stale",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.BaselineChecker = &stubStudioBatchBaselineReadinessChecker{entries: map[string]*SDSBaselineCacheEntry{
					baselineKeyForStudioBatchGateTest(eval): baselineEntryForStudioBatchGateTest(t, SDSBaselineValidationStatusReady, sdsBaselineSupportedVersion+1),
				}}
			},
			wantReason: "baseline_stale",
		},
		{
			name: "baseline_check_unavailable",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.BaselineChecker = &stubStudioBatchBaselineReadinessChecker{err: errors.New("baseline store down")}
			},
			wantReason: "baseline_check_unavailable",
		},
		{
			name: "compatibility_incomplete",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				eval.Candidate.SelectionSnapshot.TemplateImageURL = ""
			},
			wantReason: "compatibility_incomplete",
		},
		{
			name: "compatibility_mismatch",
			mutate: func(eval *studioBatchTaskGateEvaluation) {
				other := eval.Candidate.Selection
				other.SelectionID = "selection-2"
				other.Selection.MaskImageURL = "https://cdn.example.com/other-mask.png"
				eval.ItemSelections = []SheinStudioGroupedSelection{eval.Candidate.Selection, other}
			},
			wantReason: "compatibility_mismatch",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			eval := newEligibleStudioBatchGateEvaluation(t)
			tt.mutate(eval)
			gate := newStudioBatchTaskGate(eval.BaselineChecker, eval.StoreValidator)

			result, err := gate.Evaluate(context.Background(), eval)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result.Eligible {
				t.Fatalf("Evaluate() eligible = true, want rejection %s", tt.wantReason)
			}
			if result.ReasonCode != tt.wantReason {
				t.Fatalf("reason code = %q, want %q (message %q)", result.ReasonCode, tt.wantReason, result.Message)
			}
		})
	}
}

func TestStudioBatchTaskGateEvaluateEligibleAndCachesSharedChecks(t *testing.T) {
	t.Parallel()

	eval := newEligibleStudioBatchGateEvaluation(t)
	baseline := eval.BaselineChecker.(*stubStudioBatchBaselineReadinessChecker)
	store := eval.StoreValidator.(*stubStudioBatchStoreValidator)
	gate := newStudioBatchTaskGate(eval.BaselineChecker, eval.StoreValidator)

	for i := 0; i < 2; i++ {
		result, err := gate.Evaluate(context.Background(), eval)
		if err != nil {
			t.Fatalf("Evaluate(%d) error = %v", i, err)
		}
		if !result.Eligible {
			t.Fatalf("Evaluate(%d) = %+v, want eligible", i, result)
		}
	}
	if baseline.calls != 1 {
		t.Fatalf("baseline calls = %d, want cached single call", baseline.calls)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want cached single call", store.calls)
	}
}

func TestStudioBatchTaskGateAllowsReadyCacheWithUnknownValidation(t *testing.T) {
	eval := newEligibleStudioBatchGateEvaluation(t)
	baseline := eval.BaselineChecker.(*stubStudioBatchBaselineReadinessChecker)
	entry := baseline.entries[baselineKeyForStudioBatchGateTest(eval)]
	entry.Status = SDSBaselineStatusReady
	entry.ValidationStatus = SDSBaselineValidationStatusUnknown

	result, err := newStudioBatchTaskGate(eval.BaselineChecker, eval.StoreValidator).Evaluate(context.Background(), eval)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !result.Eligible {
		t.Fatalf("Evaluate() = %+v, want eligible", result)
	}
}

func TestStudioBatchTaskGateRejectsPolicyBeforeExternalChecks(t *testing.T) {
	t.Parallel()

	eval := newEligibleStudioBatchGateEvaluation(t)
	eval.Candidate.Design.ReviewStatus = StudioMaterializedDesignReviewStatusRejected
	eval.DesignsByID[eval.Candidate.Design.ID] = eval.Candidate.Design
	baseline := eval.BaselineChecker.(*stubStudioBatchBaselineReadinessChecker)
	store := eval.StoreValidator.(*stubStudioBatchStoreValidator)

	result, err := newStudioBatchTaskGate(eval.BaselineChecker, eval.StoreValidator).Evaluate(context.Background(), eval)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.Eligible || result.ReasonCode != "design_not_approved" {
		t.Fatalf("Evaluate() = %+v, want design_not_approved", result)
	}
	if store.calls != 0 || baseline.calls != 0 {
		t.Fatalf("external calls = store:%d baseline:%d, want both zero", store.calls, baseline.calls)
	}
}

func TestStudioBatchStoreProfileValidatorUsesResolvedNumericTenant(t *testing.T) {
	t.Parallel()

	repo := newInMemoryStoreProfileRepository()
	ctx := WithRequestIdentity(context.Background(), RequestIdentity{TenantID: "101"})
	if _, err := repo.Upsert(ctx, &ListingKitStoreProfile{TenantID: 101, StoreID: 870, Enabled: false, Site: "US"}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if _, err := repo.Upsert(ctx, &ListingKitStoreProfile{TenantID: 202, StoreID: 870, Enabled: true, Site: "US"}); err != nil {
		t.Fatalf("Upsert(other tenant) error = %v", err)
	}

	result, err := (studioBatchStoreProfileValidator{repo: repo}).ValidateStudioBatchStore(ctx, "101", 870)
	if err != nil {
		t.Fatalf("ValidateStudioBatchStore() error = %v", err)
	}
	if !result.Exists || !result.Valid || result.Available {
		t.Fatalf("ValidateStudioBatchStore() = %+v, want disabled store from tenant 101", result)
	}
}

func TestStudioBatchStoreProfileValidatorSkipsWhenTenantCannotResolve(t *testing.T) {
	t.Parallel()

	result, err := (studioBatchStoreProfileValidator{repo: newInMemoryStoreProfileRepository()}).ValidateStudioBatchStore(context.Background(), "tenant-a", 870)
	if err != nil {
		t.Fatalf("ValidateStudioBatchStore() error = %v", err)
	}
	if !result.Exists || !result.Valid || !result.Available {
		t.Fatalf("ValidateStudioBatchStore() = %+v, want non-blocking result for unresolved tenant", result)
	}
}

func TestServiceCreateStudioBatchTasks_ContinuesAfterGateRejectsCandidate(t *testing.T) {
	t.Parallel()

	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()
	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.TenantID = "tenant-a"
	batch.GroupedImageMode = "shared_by_size"
	selection1 := studioBatchFanOutSelection("selection-1", 3001, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png")
	selection2 := studioBatchFanOutSelection("selection-2", 3002, "Blue", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png")
	selection1.Selection.SelectedVariantIDs = []int64{3001}
	selection2.Selection.SelectedVariantIDs = []int64{3002}
	batch.GroupedSelections = SheinStudioGroupedSelectionList{selection1, selection2}
	items := []StudioBatchItemRecord{{
		ID:               "item-1",
		BatchID:          "batch-1",
		SelectionIDs:     SheinStudioStringList{"selection-1", "selection-2"},
		GroupMode:        "shared_by_size",
		Status:           StudioBatchItemStatusReviewReady,
		SelectionCount:   2,
		TargetGroupKey:   "size:1200x1200",
		TargetGroupLabel: "1200 x 1200",
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      "batch-1",
		ItemID:       "item-1",
		ImageURL:     "https://cdn.example.com/design-1.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repo.CreateStudioBatchGraph(ctx, batch, items, nil, []StudioMaterializedDesignRecord{design}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	entry := baselineEntryForStudioBatchGateTest(t, SDSBaselineValidationStatusReady, sdsBaselineSupportedVersion)
	entry.BaselineKey = baselineKeyForSelection("tenant-a", selection1.Selection)
	created := 0
	svc := newTaskStudioBatchService(taskStudioBatchServiceConfig{
		repo: repo,
		baselineChecker: &stubStudioBatchBaselineReadinessChecker{entries: map[string]*SDSBaselineCacheEntry{
			entry.BaselineKey: entry,
		}},
		storeValidator: &stubStudioBatchStoreValidator{results: map[int64]studioBatchStoreValidationResult{
			870: {Exists: true, Valid: true, Available: true},
		}},
		createGenerateTask: func(_ context.Context, _ *GenerateRequest) (*Task, error) {
			created++
			return &Task{ID: "task-created"}, nil
		},
	})

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{DesignIDs: []string{"design-1"}})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if created != 1 || len(result.CreatedTasks) != 1 {
		t.Fatalf("created = %d result = %+v, want exactly one created task", created, result.CreatedTasks)
	}
	if len(result.RejectedTasks) != 1 {
		t.Fatalf("rejected tasks = %+v, want one gate rejection", result.RejectedTasks)
	}
	if got := result.RejectedTasks[0].ReasonCode; got != "baseline_missing" {
		t.Fatalf("rejection reason = %q, want baseline_missing", got)
	}
	if result.RejectedTasks[0].SelectionID != "selection-2" {
		t.Fatalf("rejected selection = %q, want selection-2", result.RejectedTasks[0].SelectionID)
	}
}

func newEligibleStudioBatchGateEvaluation(t *testing.T) *studioBatchTaskGateEvaluation {
	t.Helper()

	now := time.Now().UTC()
	batch := &StudioBatchRecord{
		ID:               "batch-1",
		TenantID:         "tenant-a",
		GroupedImageMode: "shared_by_size",
		Status:           StudioBatchStatusReviewReady,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	selection := studioBatchFanOutSelection("selection-1", 3003, "Red", "870", "https://cdn.example.com/template.png", "https://cdn.example.com/mask.png")
	selection.Selection.SelectedVariantIDs = []int64{3003}
	selection.Selection.Variants = []SheinStudioSelectionVariant{{
		VariantID:        3003,
		PrototypeGroupID: selection.Selection.PrototypeGroupID,
		LayerID:          selection.Selection.LayerID,
		TemplateImageURL: selection.Selection.TemplateImageURL,
		MaskImageURL:     selection.Selection.MaskImageURL,
	}}
	batch.GroupedSelections = SheinStudioGroupedSelectionList{selection}
	item := StudioBatchItemRecord{
		ID:           "item-1",
		BatchID:      batch.ID,
		SelectionIDs: SheinStudioStringList{selection.SelectionID},
		GroupMode:    "shared_by_size",
		Status:       StudioBatchItemStatusReviewReady,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	design := StudioMaterializedDesignRecord{
		ID:           "design-1",
		BatchID:      batch.ID,
		ItemID:       item.ID,
		ImageURL:     "https://cdn.example.com/design-1.png",
		ReviewStatus: StudioMaterializedDesignReviewStatusApproved,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	candidate := studioBatchTaskCandidate{
		Design:                   design,
		Item:                     item,
		Selection:                selection,
		SelectionSnapshot:        selection.Selection,
		SelectionID:              selection.SelectionID,
		CompatibilityFingerprint: buildStudioBatchCompatibilityFingerprint(selection.Selection),
		SheinStoreID:             870,
		Title:                    "Red",
	}
	entry := baselineEntryForStudioBatchGateTest(t, SDSBaselineValidationStatusReady, sdsBaselineSupportedVersion)
	key := baselineKeyForSelection("tenant-a", selection.Selection)
	entry.BaselineKey = key
	return &studioBatchTaskGateEvaluation{
		Batch:          batch,
		Candidate:      candidate,
		DesignsByID:    map[string]StudioMaterializedDesignRecord{design.ID: design},
		SelectionsByID: map[string]SheinStudioGroupedSelection{selection.SelectionID: selection},
		ItemSelections: []SheinStudioGroupedSelection{selection},
		BaselineChecker: &stubStudioBatchBaselineReadinessChecker{entries: map[string]*SDSBaselineCacheEntry{
			key: entry,
		}},
		StoreValidator: &stubStudioBatchStoreValidator{results: map[int64]studioBatchStoreValidationResult{
			870: {Exists: true, Valid: true, Available: true},
		}},
	}
}

func baselineEntryForStudioBatchGateTest(t *testing.T, validationStatus string, version int) *SDSBaselineCacheEntry {
	t.Helper()

	payload, err := newCanonicalProductCachePayload(&canonical.Product{Title: "Baseline Product"})
	if err != nil {
		t.Fatalf("newCanonicalProductCachePayload: %v", err)
	}
	return &SDSBaselineCacheEntry{
		Status:               SDSBaselineStatusBaselineCached,
		Version:              version,
		CanonicalProductBase: payload,
		ValidationStatus:     validationStatus,
	}
}

func baselineKeyForStudioBatchGateTest(eval *studioBatchTaskGateEvaluation) string {
	return baselineKeyForSelection("tenant-a", eval.Candidate.SelectionSnapshot)
}

func baselineKeyForSelection(tenantID string, selection SheinStudioSelection) string {
	return sdsBaselineKey(tenantID, (&SDSBaselineReadinessQuery{
		TenantID:           tenantID,
		ParentProductID:    selection.ParentProductID,
		PrototypeGroupID:   selection.PrototypeGroupID,
		VariantID:          selection.VariantID,
		SelectedVariantIDs: selection.SelectedVariantIDs,
	}).BaselineOptions())
}
