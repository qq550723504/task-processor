package sheinsync

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"task-processor/internal/shein/api/marketing"

	"github.com/stretchr/testify/require"
)

func TestExecuteSheinActivityEnrollmentExecutesApprovedCandidatesAndUpdatesRunOutcome(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                 1,
			TenantID:           11,
			StoreID:            22,
			SyncedProductID:    101,
			ActivityType:       "PROMOTION",
			ActivityKey:        "PROMOTION:11:22",
			SKCName:            "skc-approved",
			CandidateVersion:   "v1",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(12.5),
			EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
			ReviewStatus:       SheinCandidateReviewStatusApproved,
		},
		{
			ID:                 2,
			TenantID:           11,
			StoreID:            22,
			SyncedProductID:    102,
			ActivityType:       "PROMOTION",
			ActivityKey:        "PROMOTION:11:22",
			SKCName:            "skc-auto",
			CandidateVersion:   "v1",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(13.5),
			EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
			ReviewStatus:       SheinCandidateReviewStatusAutoQueued,
		},
		{
			ID:                 3,
			TenantID:           11,
			StoreID:            22,
			SyncedProductID:    103,
			ActivityType:       "PROMOTION",
			ActivityKey:        "PROMOTION:11:22",
			SKCName:            "skc-rejected",
			CandidateVersion:   "v1",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(14.5),
			EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
			ReviewStatus:       SheinCandidateReviewStatusRejected,
		},
	})
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{
			{CandidateID: 1, Success: true},
			{CandidateID: 2, Success: false, ErrorMessage: "promotion rejected by SHEIN"},
		},
	}

	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1, 2, 3,
	)
	require.NoError(t, err)
	require.NotNil(t, run)
	require.Equal(t, int64(1), run.ID)
	require.Equal(t, SheinEnrollmentRunStatusPartiallySucceeded, run.Status)
	require.Equal(t, 3, run.CandidateCount)
	require.Equal(t, 2, run.SubmittedCount)
	require.Equal(t, 1, run.SucceededCount)
	require.Equal(t, 2, run.FailedCount)

	require.Len(t, repo.createdRuns, 1)
	require.Equal(t, SheinEnrollmentRunStatusRunning, repo.createdRuns[0].Status)
	require.Equal(t, "PROMOTION:11:22", repo.createdRuns[0].ActivityKey)
	require.Equal(t, 3, repo.createdRuns[0].CandidateCount)
	require.Zero(t, repo.createdRuns[0].SubmittedCount)
	require.Zero(t, repo.createdRuns[0].SucceededCount)
	require.Zero(t, repo.createdRuns[0].FailedCount)

	require.Len(t, repo.updatedRuns, 1)
	require.Equal(t, SheinEnrollmentRunStatusPartiallySucceeded, repo.updatedRuns[0].Status)
	require.Equal(t, "PROMOTION:11:22", repo.updatedRuns[0].ActivityKey)
	require.Equal(t, 2, repo.updatedRuns[0].SubmittedCount)
	require.Equal(t, 1, repo.updatedRuns[0].SucceededCount)
	require.Equal(t, 2, repo.updatedRuns[0].FailedCount)

	require.Len(t, repo.listCandidateQueries, 1)
	require.Equal(t, "PROMOTION:11:22", repo.listCandidateQueries[0].ActivityKey)
	require.Equal(t, []int64{1, 2, 3}, repo.listCandidateQueries[0].CandidateIDs)

	require.Len(t, adapter.calls, 1)
	require.Equal(t, int64(22), adapter.calls[0].StoreID)
	require.Equal(t, "PROMOTION", adapter.calls[0].ActivityType)
	require.Equal(t, "PROMOTION:11:22", adapter.calls[0].ActivityKey)
	require.Equal(t, []int64{1, 2}, sheinEnrollmentCandidateIDs(adapter.calls[0].Candidates))

	require.Len(t, repo.savedItems, 3)
	require.Equal(t, SheinEnrollmentItemStatusSucceeded, repo.savedItems[0].Status)
	require.Equal(t, int64(1), repo.savedItems[0].CandidateID)
	require.Equal(t, SheinEnrollmentItemStatusFailed, repo.savedItems[1].Status)
	require.Equal(t, int64(2), repo.savedItems[1].CandidateID)
	require.Equal(t, SheinEnrollmentItemStatusFailed, repo.savedItems[2].Status)
	require.Equal(t, int64(3), repo.savedItems[2].CandidateID)
	require.Contains(t, repo.savedItems[2].ErrorMessage, "review status rejected is not executable")

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 3)
	require.Equal(t, SheinCandidateReviewStatusEnrolled, candidates[0].ReviewStatus)
	require.Equal(t, SheinCandidateReviewStatusFailed, candidates[1].ReviewStatus)
	require.Equal(t, SheinCandidateReviewStatusRejected, candidates[2].ReviewStatus)
}

func TestExecuteSheinActivityEnrollmentReturnsErrorWhenCandidateIDsMissing(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:11:22",
			SKCName:           "skc-approved",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusApproved,
		},
	})
	service := NewSheinEnrollmentService(repo, &sheinEnrollmentAdapterStub{})

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1, 99,
	)
	require.Nil(t, run)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing SHEIN enrollment candidates")
	require.Empty(t, repo.createdRuns)
}

func TestExecuteSheinActivityEnrollmentBestEffortPersistsTerminalRunWhenFollowupPersistenceFails(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:11:22",
			SKCName:           "skc-approved",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusApproved,
		},
	})
	repo.saveItemsErr = errors.New("save items failed")
	repo.saveCandidatesErr = errors.New("save candidates failed")
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 1, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1,
	)
	require.NotNil(t, run)
	require.Error(t, err)
	require.ErrorContains(t, err, "save items failed")
	require.ErrorContains(t, err, "save candidates failed")
	require.Len(t, repo.updatedRuns, 2)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, repo.updatedRuns[0].Status)
	require.Equal(t, SheinEnrollmentRunStatusFailed, repo.updatedRuns[1].Status)
	require.Contains(t, repo.updatedRuns[1].ErrorSummary, "save items failed")
	require.Contains(t, repo.updatedRuns[1].ErrorSummary, "save candidates failed")
}

func TestExecuteSheinActivityEnrollmentDeduplicatesExecutableCandidatesBySKC(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:11:22",
			SKCName:           "shared-skc",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusApproved,
		},
		{
			ID:                2,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:11:22",
			SKCName:           "shared-skc",
			CandidateVersion:  "v2",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusApproved,
		},
	})
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 1, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1, 2,
	)
	require.NoError(t, err)
	require.NotNil(t, run)
	require.Equal(t, 2, run.CandidateCount)
	require.Equal(t, 1, run.SubmittedCount)
	require.Equal(t, 1, run.SucceededCount)
	require.Equal(t, 1, run.FailedCount)
	require.Len(t, adapter.calls, 1)
	require.Equal(t, []int64{1}, sheinEnrollmentCandidateIDs(adapter.calls[0].Candidates))

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 2)
	require.Equal(t, SheinCandidateReviewStatusEnrolled, candidates[0].ReviewStatus)
	require.Equal(t, SheinCandidateReviewStatusFailed, candidates[1].ReviewStatus)
	require.Len(t, repo.savedItems, 2)
	require.ErrorContains(t, errors.New(repo.savedItems[1].ErrorMessage), "duplicate executable candidate")
}

func TestExecuteSheinActivityEnrollmentManualConfirmedDoesNotRetryFailedCandidates(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:11:22",
			SKCName:           "skc-failed",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusFailed,
		},
	})
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 1, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, run.Status)
	require.Zero(t, run.CandidateCount)
	require.Zero(t, run.SubmittedCount)
	require.Zero(t, run.FailedCount)
	require.Len(t, repo.listCandidateQueries, 1)
	require.True(t, repo.listCandidateQueries[0].ExecutableOnly)
	require.Empty(t, adapter.calls)
	require.Empty(t, repo.savedItems)
	require.Equal(t, SheinCandidateReviewStatusFailed, repo.savedCandidates()[0].ReviewStatus)
}

func TestExecuteSheinActivityEnrollmentByPageOnlyLoadsExecutableCandidates(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "TIME_LIMITED",
			ActivityKey:       "TIME_LIMITED:11:22",
			SKCName:           "skc-failed",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusFailed,
		},
		{
			ID:                2,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "TIME_LIMITED",
			ActivityKey:       "TIME_LIMITED:11:22",
			SKCName:           "skc-pending",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusPendingReview,
		},
	})
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 2, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"TIME_LIMITED",
		"TIME_LIMITED:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, run.Status)
	require.Equal(t, 1, run.CandidateCount)
	require.Equal(t, 1, run.SubmittedCount)
	require.Equal(t, 1, run.SucceededCount)
	require.Zero(t, run.FailedCount)
	require.Len(t, repo.listCandidateQueries, 1)
	require.True(t, repo.listCandidateQueries[0].ExecutableOnly)
	require.Len(t, adapter.calls, 1)
	require.Equal(t, []int64{2}, sheinEnrollmentCandidateIDs(adapter.calls[0].Candidates))
	require.Len(t, repo.savedItems, 1)
	require.Equal(t, int64(2), repo.savedItems[0].CandidateID)

	candidates := repo.savedCandidates()
	require.Equal(t, SheinCandidateReviewStatusFailed, candidates[0].ReviewStatus)
	require.Equal(t, SheinCandidateReviewStatusEnrolled, candidates[1].ReviewStatus)
}

func TestExecuteSheinActivityEnrollmentManualConfirmedSubmitsPendingReviewCandidates(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:11:22",
			SKCName:           "skc-pending",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusPendingReview,
		},
	})
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 1, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, run.Status)
	require.Equal(t, 1, run.CandidateCount)
	require.Equal(t, 1, run.SubmittedCount)
	require.Len(t, adapter.calls, 1)
	require.Equal(t, []int64{1}, sheinEnrollmentCandidateIDs(adapter.calls[0].Candidates))
	require.Equal(t, SheinCandidateReviewStatusEnrolled, repo.savedCandidates()[0].ReviewStatus)
}

func TestSheinEnrollmentRepositoryRequiresSyncedProductLookup(t *testing.T) {
	t.Parallel()

	var repo SheinEnrollmentRepository = newSheinEnrollmentRepoStub(nil)
	_, _, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{
		TenantID: 11,
		StoreID:  22,
		SKCName:  "sg260618173076361709498",
		Page:     1,
		PageSize: 1,
	})

	require.NoError(t, err)
}

func TestExecuteSheinActivityEnrollmentUsesLatestSyncedProductCostBeforeEnrollment(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                 1,
			TenantID:           11,
			StoreID:            22,
			SyncedProductID:    101,
			ActivityType:       "PROMOTION",
			ActivityKey:        "PROMOTION:11:22",
			SKCName:            "sg260618173076361709498",
			CandidateVersion:   "v1",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(19.99),
			PriceSnapshot:      `{"sale_price":40,"currency":"USD"}`,
			InventorySnapshot:  `{"available":999,"total":999}`,
			EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
			ReviewStatus:       SheinCandidateReviewStatusPendingReview,
		},
	})
	repo.syncedProducts = []SheinSyncedProductRecord{
		{
			ID:                 101,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "sg260618173076361709498",
			SupplierCode:       "JJ0529207001-5CC441F3",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(34.77),
			PriceSnapshot:      `{"sale_price":40,"currency":"USD"}`,
			InventorySnapshot:  `{"available":999,"total":999}`,
			IsActive:           true,
		},
	}
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 1, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, run.Status)
	require.Len(t, repo.listSyncedProductQueries, 1)
	require.Equal(t, "sg260618173076361709498", repo.listSyncedProductQueries[0].SKCName)
	require.Len(t, adapter.calls, 1)
	require.Len(t, adapter.calls[0].Candidates, 1)
	require.NotNil(t, adapter.calls[0].Candidates[0].EffectiveCostPrice)
	require.Equal(t, 34.77, *adapter.calls[0].Candidates[0].EffectiveCostPrice)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 1)
	require.Equal(t, SheinCandidateReviewStatusEnrolled, candidates[0].ReviewStatus)
	require.NotNil(t, candidates[0].EffectiveCostPrice)
	require.Equal(t, 34.77, *candidates[0].EffectiveCostPrice)
}

func TestExecuteSheinActivityEnrollmentUsesLatestSDSCostGroupOverride(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                 1,
			TenantID:           11,
			StoreID:            22,
			SyncedProductID:    101,
			ActivityType:       "PROMOTION",
			ActivityKey:        "PROMOTION:11:22",
			SKCName:            "sg260618174087119533319",
			CandidateVersion:   "v1",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(56.42),
			PriceSnapshot:      `{"sale_price":64.9,"currency":"USD"}`,
			InventorySnapshot:  `{"available":999,"total":999}`,
			EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
			ReviewStatus:       SheinCandidateReviewStatusPendingReview,
		},
	})
	repo.syncedProducts = []SheinSyncedProductRecord{
		{
			ID:                 101,
			TenantID:           11,
			StoreID:            22,
			SKCName:            "sg260618174087119533319",
			SupplierCode:       "XB0613000001-B1C5FD77",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(56.42),
			PriceSnapshot:      `{"sale_price":64.9,"currency":"USD"}`,
			InventorySnapshot:  `{"available":999,"total":999}`,
			IsActive:           true,
		},
	}
	repo.sdsGroups = map[string]SheinSDSCostGroupRecord{
		"source:XB0613000001": {
			TenantID:        11,
			StoreID:         22,
			GroupKey:        "source:XB0613000001",
			GroupLabel:      "XB0613000001",
			ManualCostPrice: sheinEnrollmentFloat64Ptr(29.99),
		},
	}
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 1, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, run.Status)
	require.Len(t, adapter.calls, 1)
	require.Len(t, adapter.calls[0].Candidates, 1)
	require.NotNil(t, adapter.calls[0].Candidates[0].EffectiveCostPrice)
	require.Equal(t, 29.99, *adapter.calls[0].Candidates[0].EffectiveCostPrice)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 1)
	require.Equal(t, SheinCandidateReviewStatusEnrolled, candidates[0].ReviewStatus)
	require.NotNil(t, candidates[0].EffectiveCostPrice)
	require.Equal(t, 29.99, *candidates[0].EffectiveCostPrice)
}

func TestExecuteSheinActivityEnrollmentUsesAutoCostAsOriginalPriceWithManualSDSCost(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                 1,
			TenantID:           227,
			StoreID:            870,
			SyncedProductID:    456,
			ActivityType:       "PROMOTION",
			ActivityKey:        "PROMOTION:227:870",
			SKCName:            "sg260618173076361709498",
			CandidateVersion:   "v1",
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(19.99),
			PriceSnapshot:      `{"sale_price":40,"currency":"USD","sub_site":"shein-us"}`,
			InventorySnapshot:  `{"available":999,"total":999}`,
			EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
			ReviewStatus:       SheinCandidateReviewStatusPendingReview,
		},
	})
	repo.syncedProducts = []SheinSyncedProductRecord{
		{
			ID:                 456,
			TenantID:           227,
			StoreID:            870,
			SKCName:            "sg260618173076361709498",
			SupplierCode:       "JJ0529207001-5CC441F3",
			AutoCostPrice:      sheinEnrollmentFloat64Ptr(34.77),
			EffectiveCostPrice: sheinEnrollmentFloat64Ptr(34.77),
			Currency:           "USD",
			PriceSnapshot:      `{"sale_price":40,"currency":"USD","sub_site":"shein-us"}`,
			InventorySnapshot:  `{"available":999,"total":999}`,
			IsActive:           true,
		},
	}
	repo.sdsGroups = map[string]SheinSDSCostGroupRecord{
		"source:JJ0529207001": {
			TenantID:        227,
			StoreID:         870,
			GroupKey:        "source:JJ0529207001",
			GroupLabel:      "JJ0529207001",
			ManualCostPrice: sheinEnrollmentFloat64Ptr(19.99),
		},
	}
	adapter := &sheinEnrollmentAdapterStub{
		results: []SheinActivityEnrollmentResult{{CandidateID: 1, Success: true}},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		227,
		870,
		"PROMOTION",
		"PROMOTION:227:870",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, run.Status)
	require.Len(t, adapter.calls, 1)
	require.Len(t, adapter.calls[0].Candidates, 1)
	require.NotNil(t, adapter.calls[0].Candidates[0].EffectiveCostPrice)
	require.Equal(t, 19.99, *adapter.calls[0].Candidates[0].EffectiveCostPrice)
	price, currency := parsePromotionPriceSnapshot(adapter.calls[0].Candidates[0].PriceSnapshot)
	require.Equal(t, 34.77, price)
	require.Equal(t, "USD", currency)

	candidates := repo.savedCandidates()
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].EffectiveCostPrice)
	require.Equal(t, 19.99, *candidates[0].EffectiveCostPrice)
	price, currency = parsePromotionPriceSnapshot(candidates[0].PriceSnapshot)
	require.Equal(t, 34.77, price)
	require.Equal(t, "USD", currency)
}

func TestExecuteSheinActivityEnrollmentMarksRunFailedWhenCandidatesExistButNoneExecutable(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:11:22",
			SKCName:           "skc-rejected",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusRejected,
		},
	})
	adapter := &sheinEnrollmentAdapterStub{}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
		1,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusFailed, run.Status)
	require.Equal(t, 1, run.CandidateCount)
	require.Zero(t, run.SubmittedCount)
	require.Zero(t, run.SucceededCount)
	require.Equal(t, 1, run.FailedCount)
	require.Contains(t, run.ErrorSummary, "no executable SHEIN enrollment candidates")
	require.Empty(t, adapter.calls)
	require.Len(t, repo.savedItems, 1)
	require.Equal(t, int64(1), repo.savedItems[0].CandidateID)
	require.Equal(t, SheinEnrollmentItemStatusFailed, repo.savedItems[0].Status)
	require.Contains(t, repo.savedItems[0].ErrorMessage, "review status rejected is not executable")
	require.Equal(t, SheinCandidateReviewStatusRejected, repo.savedCandidates()[0].ReviewStatus)
}

func TestExecuteSheinActivityEnrollmentPersistsTimeLimitedBatchFallbackResultsFromAdapter(t *testing.T) {
	t.Parallel()

	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "TIME_LIMITED",
			ActivityKey:       "TIME_LIMITED:11:22",
			SKCName:           "skc-ok-1",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusPendingReview,
		},
		{
			ID:                2,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "TIME_LIMITED",
			ActivityKey:       "TIME_LIMITED:11:22",
			SKCName:           "skc-bad",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusPendingReview,
		},
		{
			ID:                3,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "TIME_LIMITED",
			ActivityKey:       "TIME_LIMITED:11:22",
			SKCName:           "skc-ok-2",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusPendingReview,
		},
	})
	adapter := &sheinEnrollmentAdapterStub{
		enroll: func(_ context.Context, _ int64, _ string, _ string, candidates []SheinActivityEnrollmentCandidate) ([]SheinActivityEnrollmentResult, error) {
			return []SheinActivityEnrollmentResult{
				{
					CandidateID: candidates[0].CandidateID,
					Success:     true,
				},
				{
					CandidateID:  candidates[1].CandidateID,
					Success:      false,
					ErrorMessage: "创建限时折扣活动失败: 参数无效",
				},
				{
					CandidateID: candidates[2].CandidateID,
					Success:     true,
				},
			}, nil
		},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		context.Background(),
		11,
		22,
		"TIME_LIMITED",
		"TIME_LIMITED:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusPartiallySucceeded, run.Status)
	require.Equal(t, 3, run.CandidateCount)
	require.Equal(t, 3, run.SubmittedCount)
	require.Equal(t, 2, run.SucceededCount)
	require.Equal(t, 1, run.FailedCount)
	require.Len(t, adapter.calls, 1)
	require.Equal(t, []int64{1, 2, 3}, sheinEnrollmentCandidateIDs(adapter.calls[0].Candidates))

	require.Len(t, repo.savedItems, 3)
	require.Equal(t, SheinEnrollmentItemStatusSucceeded, repo.savedItems[0].Status)
	require.Equal(t, SheinEnrollmentItemStatusFailed, repo.savedItems[1].Status)
	require.Contains(t, repo.savedItems[1].ErrorMessage, "参数无效")
	require.Equal(t, SheinEnrollmentItemStatusSucceeded, repo.savedItems[2].Status)

	candidates := repo.savedCandidates()
	require.Equal(t, SheinCandidateReviewStatusEnrolled, candidates[0].ReviewStatus)
	require.Equal(t, SheinCandidateReviewStatusFailed, candidates[1].ReviewStatus)
	require.Equal(t, SheinCandidateReviewStatusEnrolled, candidates[2].ReviewStatus)
}

func TestExecuteSheinActivityEnrollmentPersistsOutcomeAfterRequestContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "TIME_LIMITED",
			ActivityKey:       "TIME_LIMITED:11:22",
			SKCName:           "skc-ok",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusPendingReview,
		},
	})
	repo.respectContextCancellation = true
	adapter := &sheinEnrollmentAdapterStub{
		enroll: func(_ context.Context, _ int64, _ string, _ string, candidates []SheinActivityEnrollmentCandidate) ([]SheinActivityEnrollmentResult, error) {
			cancel()
			return []SheinActivityEnrollmentResult{{
				CandidateID: candidates[0].CandidateID,
				Success:     true,
			}}, nil
		},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.ExecuteSheinActivityEnrollment(
		ctx,
		11,
		22,
		"TIME_LIMITED",
		"TIME_LIMITED:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, run.Status)
	require.Len(t, repo.updatedRuns, 1)
	require.Equal(t, SheinEnrollmentRunStatusSucceeded, repo.updatedRuns[0].Status)
	require.Len(t, repo.savedItems, 1)
	require.Equal(t, SheinEnrollmentItemStatusSucceeded, repo.savedItems[0].Status)
	require.Equal(t, SheinCandidateReviewStatusEnrolled, repo.savedCandidates()[0].ReviewStatus)
}

func TestStartSheinActivityEnrollmentRunsInBackgroundAfterRequestContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	repo := newSheinEnrollmentRepoStub([]SheinActivityCandidateRecord{
		{
			ID:                1,
			TenantID:          11,
			StoreID:           22,
			ActivityType:      "TIME_LIMITED",
			ActivityKey:       "TIME_LIMITED:11:22",
			SKCName:           "skc-ok",
			CandidateVersion:  "v1",
			EligibilityStatus: SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      SheinCandidateReviewStatusPendingReview,
		},
	})
	repo.respectContextCancellation = true
	adapterCalled := make(chan struct{})
	adapter := &sheinEnrollmentAdapterStub{
		enroll: func(ctx context.Context, _ int64, _ string, _ string, candidates []SheinActivityEnrollmentCandidate) ([]SheinActivityEnrollmentResult, error) {
			cancel()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			close(adapterCalled)
			return []SheinActivityEnrollmentResult{{
				CandidateID: candidates[0].CandidateID,
				Success:     true,
			}}, nil
		},
	}
	service := NewSheinEnrollmentService(repo, adapter)

	run, err := service.StartSheinActivityEnrollment(
		ctx,
		11,
		22,
		"TIME_LIMITED",
		"TIME_LIMITED:11:22",
		SheinEnrollmentRunTriggerModeManualConfirmed,
	)

	require.NoError(t, err)
	require.Equal(t, SheinEnrollmentRunStatusRunning, run.Status)
	select {
	case <-adapterCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("adapter was not called")
	}
	require.Eventually(t, func() bool {
		candidates := repo.savedCandidates()
		return len(repo.savedItems) == 1 &&
			repo.savedItems[0].Status == SheinEnrollmentItemStatusSucceeded &&
			len(candidates) == 1 &&
			candidates[0].ReviewStatus == SheinCandidateReviewStatusEnrolled
	}, 2*time.Second, 10*time.Millisecond)
}

func TestSheinActivityAdapterUsesListingKitCandidatesAsOnlyPromotionSource(t *testing.T) {
	t.Parallel()

	strategyProvider := &sheinPromotionStrategyProviderStub{
		strategy: NewSheinPromotionStrategy(SheinPromotionStrategyInput{
			StoreID:              22,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		}),
	}
	bridge := &sheinPromotionBridgeStub{
		result: &SheinPromotionRegistrationResult{
			Request: &marketing.SaveConfigRequest{
				ConfigList: []marketing.ActivityConfig{
					{Skc: "skc-approved", ActStock: 5, ReservedActStock: 10, DropRate: 20},
				},
			},
			Response: &marketing.SaveConfigResponse{Code: "0", Msg: "ok"},
			FilterReasons: map[string]string{
				"skc-filtered": "商品 skc-filtered 库存不足(8 < 15)",
			},
		},
	}
	adapter := newSheinActivityAdapter(strategyProvider, bridge)

	results, err := adapter.EnrollCandidates(
		context.Background(),
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		[]SheinActivityEnrollmentCandidate{
			{
				CandidateID:        1,
				ActivityKey:        "PROMOTION:11:22",
				CandidateVersion:   "v1",
				SKCName:            "skc-approved",
				EffectiveCostPrice: sheinEnrollmentFloat64Ptr(12.5),
				PriceSnapshot:      `{"sale_price":29.9,"currency":"USD"}`,
				InventorySnapshot:  `{"available":10}`,
			},
			{
				CandidateID:        2,
				ActivityKey:        "PROMOTION:11:22",
				CandidateVersion:   "v1",
				SKCName:            "skc-filtered",
				EffectiveCostPrice: sheinEnrollmentFloat64Ptr(13.5),
				PriceSnapshot:      `{"sale_price":39.9,"currency":"USD"}`,
				InventorySnapshot:  `{"available":8}`,
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, bridge.calls, 1)
	require.Empty(t, bridge.calls[0].ActivityKey)
	require.Equal(t, int64(22), bridge.calls[0].Strategy.StoreID)
	require.Len(t, bridge.calls[0].Products, 2)
	require.Equal(t, []string{"skc-approved", "skc-filtered"}, sheinPromotionBridgeSKCs(bridge.calls[0].Products))
	require.Equal(t, 12.5, bridge.calls[0].Products[0].SupplyPrice)
	require.Equal(t, 13.5, bridge.calls[0].Products[1].SupplyPrice)
	require.Len(t, results, 2)
	require.True(t, results[0].Success)
	require.Equal(t, int64(1), results[0].CandidateID)
	require.False(t, results[1].Success)
	require.Equal(t, int64(2), results[1].CandidateID)
	require.Equal(t, "商品 skc-filtered 库存不足(8 < 15)", results[1].ErrorMessage)
}

func TestSheinActivityAdapterSupportsTimeLimitedEnrollment(t *testing.T) {
	t.Parallel()

	strategyProvider := &sheinPromotionStrategyProviderStub{
		strategy: NewSheinPromotionStrategy(SheinPromotionStrategyInput{
			StoreID:              22,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		}),
	}
	bridge := &sheinPromotionBridgeStub{
		result: &SheinPromotionRegistrationResult{
			ActivityRequest: &marketing.CreateActivityRequest{
				AddCostAndStockInfoList: []marketing.CostAndStockInfo{
					{Skc: "skc-time-limited", AttendNum: 5, StockNum: 5},
				},
			},
			ActivityResponse: &marketing.CreateActivityResponse{
				Code: "0",
				Msg:  "ok",
				Info: &marketing.ActivityCreateInfo{ActivityID: 123},
			},
		},
	}
	adapter := newSheinActivityAdapter(strategyProvider, bridge)

	results, err := adapter.EnrollCandidates(
		context.Background(),
		22,
		"TIME_LIMITED",
		"TIME_LIMITED:11:22",
		[]SheinActivityEnrollmentCandidate{
			{
				CandidateID:        1,
				ActivityKey:        "TIME_LIMITED:11:22",
				CandidateVersion:   "v1",
				SKCName:            "skc-time-limited",
				EffectiveCostPrice: sheinEnrollmentFloat64Ptr(12.5),
				PriceSnapshot:      `{"sale_price":29.9,"currency":"USD"}`,
				InventorySnapshot:  `{"available":10}`,
			},
		},
	)

	require.NoError(t, err)
	require.Len(t, bridge.calls, 1)
	require.Equal(t, "TIME_LIMITED:11:22:1", bridge.calls[0].ActivityKey)
	require.Len(t, results, 1)
	require.True(t, results[0].Success)
}

func TestSheinActivityAdapterTimeLimitedBatchFallbackReusesPromotionSession(t *testing.T) {
	t.Parallel()

	strategyProvider := &sheinPromotionStrategyProviderStub{
		strategy: NewSheinPromotionStrategy(SheinPromotionStrategyInput{
			StoreID:              22,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		}),
	}
	bridge := &sheinPromotionSessionBridgeStub{}
	adapter := newSheinActivityAdapter(strategyProvider, bridge)

	results, err := adapter.EnrollCandidates(
		context.Background(),
		22,
		"TIME_LIMITED",
		"TIME_LIMITED:11:22",
		[]SheinActivityEnrollmentCandidate{
			{
				CandidateID:       1,
				SKCName:           "skc-one",
				PriceSnapshot:     `{"sale_price":29.9,"currency":"USD"}`,
				InventorySnapshot: `{"available":10}`,
			},
			{
				CandidateID:       2,
				SKCName:           "skc-two",
				PriceSnapshot:     `{"sale_price":39.9,"currency":"USD"}`,
				InventorySnapshot: `{"available":10}`,
			},
		},
	)

	require.NoError(t, err)
	require.Len(t, results, 2)
	require.True(t, results[0].Success)
	require.True(t, results[1].Success)
	require.Equal(t, 1, bridge.sessionStarts)
	require.Zero(t, bridge.directCalls)
	require.Equal(t, [][]string{
		{"skc-one", "skc-two"},
		{"skc-one"},
		{"skc-two"},
	}, bridge.session.productSKCs)
}

func TestSheinActivityAdapterPromotionUsesSingleDirectBridgeCallWhenSessionIsAvailable(t *testing.T) {
	t.Parallel()

	strategyProvider := &sheinPromotionStrategyProviderStub{
		strategy: NewSheinPromotionStrategy(SheinPromotionStrategyInput{
			StoreID:              22,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		}),
	}
	bridge := &sheinPromotionSessionCapableBridgeStub{}
	adapter := newSheinActivityAdapter(strategyProvider, bridge)

	results, err := adapter.EnrollCandidates(
		context.Background(),
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		[]SheinActivityEnrollmentCandidate{
			{
				CandidateID:       1,
				SKCName:           "skc-one",
				PriceSnapshot:     `{"sale_price":29.9,"currency":"USD"}`,
				InventorySnapshot: `{"available":10}`,
			},
			{
				CandidateID:       2,
				SKCName:           "skc-two",
				PriceSnapshot:     `{"sale_price":39.9,"currency":"USD"}`,
				InventorySnapshot: `{"available":10}`,
			},
		},
	)

	require.NoError(t, err)
	require.Len(t, results, 2)
	require.True(t, results[0].Success)
	require.True(t, results[1].Success)
	require.Zero(t, bridge.sessionStarts)
	require.Equal(t, 1, bridge.directCalls)
	require.Equal(t, []string{"skc-one", "skc-two"}, bridge.directProductSKCs)
}

func TestSheinActivityAdapterRejectsInvalidPromotionDiscountStrategy(t *testing.T) {
	t.Parallel()

	strategyProvider := &sheinPromotionStrategyProviderStub{
		strategy: NewSheinPromotionStrategy(SheinPromotionStrategyInput{
			StoreID:              22,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0,
			ActivityStockRatio:   0.5,
		}),
	}
	bridge := &sheinPromotionBridgeStub{}
	adapter := newSheinActivityAdapter(strategyProvider, bridge)

	results, err := adapter.EnrollCandidates(
		context.Background(),
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		[]SheinActivityEnrollmentCandidate{
			{
				CandidateID:       1,
				SKCName:           "skc-approved",
				PriceSnapshot:     `{"sale_price":29.9,"currency":"USD"}`,
				InventorySnapshot: `{"available":10}`,
			},
		},
	)

	require.ErrorContains(t, err, "activity discount rate must be between 0 and 1")
	require.Empty(t, results)
	require.Empty(t, bridge.calls)
}

func TestSheinActivityAdapterAllowsZeroPromotionMinProfitRate(t *testing.T) {
	t.Parallel()

	strategyProvider := &sheinPromotionStrategyProviderStub{
		strategy: NewSheinPromotionStrategy(SheinPromotionStrategyInput{
			StoreID:               22,
			ActivityPriceMode:     "PROFIT",
			ActivityStockRatio:    0.5,
			ActivityMinProfitRate: 0,
		}),
	}
	bridge := &sheinPromotionBridgeStub{
		result: &SheinPromotionRegistrationResult{
			Response: &marketing.SaveConfigResponse{Code: "0", Msg: "ok"},
		},
	}
	adapter := newSheinActivityAdapter(strategyProvider, bridge)

	results, err := adapter.EnrollCandidates(
		context.Background(),
		22,
		"PROMOTION",
		"PROMOTION:11:22",
		[]SheinActivityEnrollmentCandidate{
			{
				CandidateID:       1,
				SKCName:           "skc-approved",
				PriceSnapshot:     `{"sale_price":29.9,"currency":"USD"}`,
				InventorySnapshot: `{"available":10}`,
			},
		},
	)

	require.NoError(t, err)
	require.Len(t, bridge.calls, 1)
	require.Len(t, results, 1)
}

type sheinEnrollmentRepoStub struct {
	mu                       sync.Mutex
	nextRunID                int64
	candidates               map[int64]SheinActivityCandidateRecord
	syncedProducts           []SheinSyncedProductRecord
	sdsGroups                map[string]SheinSDSCostGroupRecord
	createdRuns              []SheinActivityEnrollmentRunRecord
	updatedRuns              []SheinActivityEnrollmentRunRecord
	savedItems               []SheinActivityEnrollmentItemRecord
	listCandidateQueries     []SheinActivityCandidateQuery
	listSyncedProductQueries []SheinSyncedProductQuery

	saveItemsErr               error
	saveCandidatesErr          error
	respectContextCancellation bool
}

func newSheinEnrollmentRepoStub(seed []SheinActivityCandidateRecord) *sheinEnrollmentRepoStub {
	candidates := make(map[int64]SheinActivityCandidateRecord, len(seed))
	for _, row := range seed {
		candidates[row.ID] = cloneSheinEnrollmentCandidate(row)
	}
	return &sheinEnrollmentRepoStub{
		nextRunID:  1,
		candidates: candidates,
	}
}

func (r *sheinEnrollmentRepoStub) ListCandidates(_ context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if query != nil {
		copied := *query
		copied.CandidateIDs = append([]int64(nil), query.CandidateIDs...)
		r.listCandidateQueries = append(r.listCandidateQueries, copied)
	}

	items := make([]SheinActivityCandidateRecord, 0, len(r.candidates))
	for _, row := range r.candidates {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.ActivityType != "" && row.ActivityType != query.ActivityType {
				continue
			}
			if query.ActivityKey != "" && row.ActivityKey != query.ActivityKey {
				continue
			}
			if len(query.CandidateIDs) > 0 && !containsSheinEnrollmentID(query.CandidateIDs, row.ID) {
				continue
			}
			if query.ExecutableOnly && !isExecutableSheinEnrollmentQueryCandidate(row) {
				continue
			}
		}
		items = append(items, cloneSheinEnrollmentCandidate(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, int64(len(items)), nil
}

func (r *sheinEnrollmentRepoStub) SaveCandidates(ctx context.Context, records []*SheinActivityCandidateRecord) error {
	if r.respectContextCancellation && ctx.Err() != nil {
		return ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, record := range records {
		if record == nil {
			continue
		}
		r.candidates[record.ID] = cloneSheinEnrollmentCandidate(*record)
	}
	return r.saveCandidatesErr
}

func (r *sheinEnrollmentRepoStub) ListSyncedProducts(_ context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if query != nil {
		copied := *query
		r.listSyncedProductQueries = append(r.listSyncedProductQueries, copied)
	}

	items := make([]SheinSyncedProductRecord, 0, len(r.syncedProducts))
	for _, row := range r.syncedProducts {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.SKCName != "" && row.SKCName != query.SKCName {
				continue
			}
			if query.IsActive != nil && row.IsActive != *query.IsActive {
				continue
			}
		}
		items = append(items, cloneSheinEnrollmentSyncedProduct(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, int64(len(items)), nil
}

func (r *sheinEnrollmentRepoStub) ListSDSCostGroups(_ context.Context, query *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]SheinSDSCostGroupRecord, 0, len(r.sdsGroups))
	for _, row := range r.sdsGroups {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if len(query.GroupKeys) > 0 && !containsSheinEnrollmentString(query.GroupKeys, row.GroupKey) {
				continue
			}
		}
		items = append(items, cloneSheinEnrollmentSDSCostGroup(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].GroupKey < items[j].GroupKey
	})
	return items, int64(len(items)), nil
}

func (r *sheinEnrollmentRepoStub) CreateEnrollmentRun(_ context.Context, run *SheinActivityEnrollmentRunRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	row := *run
	row.ID = r.nextRunID
	r.nextRunID++
	r.createdRuns = append(r.createdRuns, row)
	run.ID = row.ID
	return nil
}

func (r *sheinEnrollmentRepoStub) UpdateEnrollmentRun(ctx context.Context, run *SheinActivityEnrollmentRunRecord) error {
	if r.respectContextCancellation && ctx.Err() != nil {
		return ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.updatedRuns = append(r.updatedRuns, *run)
	return nil
}

func (r *sheinEnrollmentRepoStub) ListEnrollmentRuns(_ context.Context, _ *SheinEnrollmentRunQuery) ([]SheinActivityEnrollmentRunRecord, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := append([]SheinActivityEnrollmentRunRecord(nil), r.createdRuns...)
	items = append(items, r.updatedRuns...)
	return items, int64(len(items)), nil
}

func (r *sheinEnrollmentRepoStub) SaveEnrollmentItems(ctx context.Context, items []*SheinActivityEnrollmentItemRecord) error {
	if r.respectContextCancellation && ctx.Err() != nil {
		return ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.savedItems = r.savedItems[:0]
	for _, item := range items {
		if item == nil {
			continue
		}
		r.savedItems = append(r.savedItems, *item)
	}
	sort.Slice(r.savedItems, func(i, j int) bool {
		return r.savedItems[i].CandidateID < r.savedItems[j].CandidateID
	})
	return r.saveItemsErr
}

func (r *sheinEnrollmentRepoStub) ListEnrollmentItems(_ context.Context, _ *SheinEnrollmentItemQuery) ([]SheinActivityEnrollmentItemRecord, int64, error) {
	return nil, 0, nil
}

func (r *sheinEnrollmentRepoStub) savedCandidates() []SheinActivityCandidateRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]SheinActivityCandidateRecord, 0, len(r.candidates))
	for _, row := range r.candidates {
		items = append(items, cloneSheinEnrollmentCandidate(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

type sheinEnrollmentAdapterStub struct {
	calls   []sheinEnrollmentAdapterCall
	results []SheinActivityEnrollmentResult
	err     error
	enroll  func(context.Context, int64, string, string, []SheinActivityEnrollmentCandidate) ([]SheinActivityEnrollmentResult, error)
}

type sheinEnrollmentAdapterCall struct {
	StoreID      int64
	ActivityType string
	ActivityKey  string
	Candidates   []SheinActivityEnrollmentCandidate
}

func (s *sheinEnrollmentAdapterStub) EnrollCandidates(
	ctx context.Context,
	storeID int64,
	activityType string,
	activityKey string,
	candidates []SheinActivityEnrollmentCandidate,
) ([]SheinActivityEnrollmentResult, error) {
	s.calls = append(s.calls, sheinEnrollmentAdapterCall{
		StoreID:      storeID,
		ActivityType: activityType,
		ActivityKey:  activityKey,
		Candidates:   append([]SheinActivityEnrollmentCandidate(nil), candidates...),
	})
	if s.enroll != nil {
		return s.enroll(ctx, storeID, activityType, activityKey, candidates)
	}
	return append([]SheinActivityEnrollmentResult(nil), s.results...), s.err
}

type sheinPromotionStrategyProviderStub struct {
	strategy *SheinPromotionStrategy
	err      error
}

func (s *sheinPromotionStrategyProviderStub) GetPromotionStrategy(_ context.Context, storeID int64, activityKey string) (*SheinPromotionStrategy, error) {
	return s.strategy, s.err
}

type sheinPromotionBridgeStub struct {
	calls  []sheinPromotionBridgeCall
	result *SheinPromotionRegistrationResult
	err    error
}

type sheinPromotionSessionBridgeStub struct {
	sessionStarts int
	directCalls   int
	session       *sheinPromotionRegistrationSessionStub
}

func (s *sheinPromotionSessionBridgeStub) RegisterPromotionProducts(
	_ context.Context,
	_ *SheinPromotionStrategy,
	_ string,
	_ []marketing.SkcInfo,
) (*SheinPromotionRegistrationResult, error) {
	s.directCalls++
	return nil, errors.New("direct bridge should not be used when session is available")
}

func (s *sheinPromotionSessionBridgeStub) StartPromotionRegistrationSession(
	_ context.Context,
	_ *SheinPromotionStrategy,
	_ string,
) (SheinPromotionRegistrationSession, error) {
	s.sessionStarts++
	s.session = &sheinPromotionRegistrationSessionStub{}
	return s.session, nil
}

type sheinPromotionSessionCapableBridgeStub struct {
	sessionStarts     int
	directCalls       int
	directProductSKCs []string
}

func (s *sheinPromotionSessionCapableBridgeStub) RegisterPromotionProducts(
	_ context.Context,
	_ *SheinPromotionStrategy,
	_ string,
	products []marketing.SkcInfo,
) (*SheinPromotionRegistrationResult, error) {
	s.directCalls++
	s.directProductSKCs = sheinPromotionBridgeSKCs(products)
	request := &marketing.CreateActivityRequest{
		AddCostAndStockInfoList: make([]marketing.CostAndStockInfo, 0, len(products)),
	}
	for _, product := range products {
		request.AddCostAndStockInfoList = append(request.AddCostAndStockInfoList, marketing.CostAndStockInfo{Skc: product.Skc})
	}
	return &SheinPromotionRegistrationResult{ActivityRequest: request}, nil
}

func (s *sheinPromotionSessionCapableBridgeStub) StartPromotionRegistrationSession(
	_ context.Context,
	_ *SheinPromotionStrategy,
	_ string,
) (SheinPromotionRegistrationSession, error) {
	s.sessionStarts++
	return &sheinPromotionRegistrationSessionStub{}, nil
}

type sheinPromotionRegistrationSessionStub struct {
	productSKCs [][]string
}

func (s *sheinPromotionRegistrationSessionStub) RegisterPromotionProducts(
	_ context.Context,
	_ string,
	products []marketing.SkcInfo,
) (*SheinPromotionRegistrationResult, error) {
	s.productSKCs = append(s.productSKCs, sheinPromotionBridgeSKCs(products))
	request := &marketing.CreateActivityRequest{
		AddCostAndStockInfoList: make([]marketing.CostAndStockInfo, 0, len(products)),
	}
	for _, product := range products {
		request.AddCostAndStockInfoList = append(request.AddCostAndStockInfoList, marketing.CostAndStockInfo{Skc: product.Skc})
	}
	result := &SheinPromotionRegistrationResult{ActivityRequest: request}
	if len(products) > 1 {
		return result, errors.New("batch rejected")
	}
	return result, nil
}

type sheinPromotionBridgeCall struct {
	Strategy    *SheinPromotionStrategy
	ActivityKey string
	Products    []marketing.SkcInfo
}

func (s *sheinPromotionBridgeStub) RegisterPromotionProducts(
	_ context.Context,
	strategy *SheinPromotionStrategy,
	activityKey string,
	products []marketing.SkcInfo,
) (*SheinPromotionRegistrationResult, error) {
	copiedProducts := append([]marketing.SkcInfo(nil), products...)
	s.calls = append(s.calls, sheinPromotionBridgeCall{
		Strategy:    strategy,
		ActivityKey: activityKey,
		Products:    copiedProducts,
	})
	return s.result, s.err
}

func sheinEnrollmentCandidateIDs(candidates []SheinActivityEnrollmentCandidate) []int64 {
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.CandidateID)
	}
	return ids
}

func sheinEnrollmentAdapterCallCandidateIDSets(calls []sheinEnrollmentAdapterCall) [][]int64 {
	sets := make([][]int64, 0, len(calls))
	for _, call := range calls {
		sets = append(sets, sheinEnrollmentCandidateIDs(call.Candidates))
	}
	return sets
}

func isExecutableSheinEnrollmentQueryCandidate(row SheinActivityCandidateRecord) bool {
	if row.EligibilityStatus != SheinCandidateEligibilityStatusEligible {
		return false
	}
	switch row.ReviewStatus {
	case SheinCandidateReviewStatusPendingReview,
		SheinCandidateReviewStatusApproved,
		SheinCandidateReviewStatusAutoQueued:
		return true
	default:
		return false
	}
}

func cloneSheinEnrollmentCandidate(row SheinActivityCandidateRecord) SheinActivityCandidateRecord {
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	row.CalculatedProfitRate = cloneServiceTestFloat64(row.CalculatedProfitRate)
	return row
}

func cloneSheinEnrollmentSyncedProduct(row SheinSyncedProductRecord) SheinSyncedProductRecord {
	row.AutoCostPrice = cloneServiceTestFloat64(row.AutoCostPrice)
	row.ManualCostPrice = cloneServiceTestFloat64(row.ManualCostPrice)
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	return row
}

func cloneSheinEnrollmentSDSCostGroup(row SheinSDSCostGroupRecord) SheinSDSCostGroupRecord {
	row.ManualCostPrice = cloneServiceTestFloat64(row.ManualCostPrice)
	return row
}

func sheinEnrollmentFloat64Ptr(v float64) *float64 {
	return &v
}

func containsSheinEnrollmentID(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func containsSheinEnrollmentString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sheinPromotionBridgeSKCs(products []marketing.SkcInfo) []string {
	values := make([]string, 0, len(products))
	for _, product := range products {
		values = append(values, product.Skc)
	}
	return values
}
