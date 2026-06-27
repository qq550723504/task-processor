package sheinsync

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

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
	require.Equal(t, 1, run.FailedCount)

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
	require.Equal(t, 1, repo.updatedRuns[0].FailedCount)

	require.Len(t, repo.listCandidateQueries, 1)
	require.Equal(t, "PROMOTION:11:22", repo.listCandidateQueries[0].ActivityKey)
	require.Equal(t, []int64{1, 2, 3}, repo.listCandidateQueries[0].CandidateIDs)

	require.Len(t, adapter.calls, 1)
	require.Equal(t, int64(22), adapter.calls[0].StoreID)
	require.Equal(t, "PROMOTION", adapter.calls[0].ActivityType)
	require.Equal(t, "PROMOTION:11:22", adapter.calls[0].ActivityKey)
	require.Equal(t, []int64{1, 2}, sheinEnrollmentCandidateIDs(adapter.calls[0].Candidates))

	require.Len(t, repo.savedItems, 2)
	require.Equal(t, SheinEnrollmentItemStatusSucceeded, repo.savedItems[0].Status)
	require.Equal(t, int64(1), repo.savedItems[0].CandidateID)
	require.Equal(t, SheinEnrollmentItemStatusFailed, repo.savedItems[1].Status)
	require.Equal(t, int64(2), repo.savedItems[1].CandidateID)

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
					{Skc: "skc-approved", ActStock: 5, DropRate: 20, ReservedActStock: 10},
				},
			},
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
	require.Equal(t, "PROMOTION:11:22", bridge.calls[0].ActivityKey)
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
	require.ErrorContains(t, errors.New(results[1].ErrorMessage), "filtered")
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
	mu                   sync.Mutex
	nextRunID            int64
	candidates           map[int64]SheinActivityCandidateRecord
	createdRuns          []SheinActivityEnrollmentRunRecord
	updatedRuns          []SheinActivityEnrollmentRunRecord
	savedItems           []SheinActivityEnrollmentItemRecord
	listCandidateQueries []SheinActivityCandidateQuery

	saveItemsErr      error
	saveCandidatesErr error
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
		}
		items = append(items, cloneSheinEnrollmentCandidate(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, int64(len(items)), nil
}

func (r *sheinEnrollmentRepoStub) SaveCandidates(_ context.Context, records []*SheinActivityCandidateRecord) error {
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

func (r *sheinEnrollmentRepoStub) UpdateEnrollmentRun(_ context.Context, run *SheinActivityEnrollmentRunRecord) error {
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

func (r *sheinEnrollmentRepoStub) SaveEnrollmentItems(_ context.Context, items []*SheinActivityEnrollmentItemRecord) error {
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
}

type sheinEnrollmentAdapterCall struct {
	StoreID      int64
	ActivityType string
	ActivityKey  string
	Candidates   []SheinActivityEnrollmentCandidate
}

func (s *sheinEnrollmentAdapterStub) EnrollCandidates(
	_ context.Context,
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

func cloneSheinEnrollmentCandidate(row SheinActivityCandidateRecord) SheinActivityCandidateRecord {
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	row.CalculatedProfitRate = cloneServiceTestFloat64(row.CalculatedProfitRate)
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

func sheinPromotionBridgeSKCs(products []marketing.SkcInfo) []string {
	values := make([]string, 0, len(products))
	for _, product := range products {
		values = append(values, product.Skc)
	}
	return values
}
