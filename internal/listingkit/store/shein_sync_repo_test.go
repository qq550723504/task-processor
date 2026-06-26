package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
)

type sheinSyncRepositoryHarness struct {
	name           string
	repo           listingkit.SheinSyncRepository
	listCandidates func(ctx context.Context, query *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error)
	listRuns       func(t *testing.T) []listingkit.SheinActivityEnrollmentRunRecord
	listItems      func(t *testing.T) []listingkit.SheinActivityEnrollmentItemRecord
}

func TestSheinSyncRepositoryUpsertSyncedProductsByStoreAndSKC(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			now := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)

			first := &listingkit.SheinSyncedProductRecord{
				TenantID:      1,
				StoreID:       101,
				SKCName:       "skc-1",
				BusinessModel: 7,
				ShelfStatus:   "ON_SHELF",
				IsActive:      true,
				LastSyncAt:    &now,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			second := &listingkit.SheinSyncedProductRecord{
				TenantID:      1,
				StoreID:       101,
				SKCName:       "skc-1",
				BusinessModel: 7,
				ShelfStatus:   "OFF_SHELF",
				AutoCostPrice: float64Ptr(12.5),
				IsActive:      false,
				LastSyncAt:    sheinTimePtr(now.Add(time.Minute)),
				CreatedAt:     now.Add(time.Minute),
				UpdatedAt:     now.Add(time.Minute),
			}

			if err := harness.repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{first}); err != nil {
				t.Fatalf("first upsert: %v", err)
			}
			if err := harness.repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{second}); err != nil {
				t.Fatalf("second upsert: %v", err)
			}

			items, total, err := harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{
				TenantID: 1,
				StoreID:  101,
			})
			if err != nil {
				t.Fatalf("list synced products: %v", err)
			}
			if total != 1 {
				t.Fatalf("total = %d, want 1", total)
			}
			if len(items) != 1 {
				t.Fatalf("len(items) = %d, want 1", len(items))
			}
			if items[0].ShelfStatus != "OFF_SHELF" {
				t.Fatalf("shelf status = %q, want OFF_SHELF", items[0].ShelfStatus)
			}
			if items[0].IsActive {
				t.Fatalf("is_active = true, want false")
			}
			if items[0].CostPriceSource != listingkit.SheinCostPriceSourceAuto {
				t.Fatalf("cost price source = %q, want auto", items[0].CostPriceSource)
			}
			if items[0].EffectiveCostPrice == nil || *items[0].EffectiveCostPrice != 12.5 {
				t.Fatalf("effective cost = %v, want 12.5", items[0].EffectiveCostPrice)
			}
			if items[0].BusinessModel != 7 {
				t.Fatalf("business model = %d, want 7", items[0].BusinessModel)
			}
		})
	}
}

func TestSheinSyncRepositoryListSyncedProductsSupportsFilteringAndPagination(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
			activeOnly := true

			records := []*listingkit.SheinSyncedProductRecord{
				{
					TenantID:    1,
					StoreID:     101,
					SKCName:     "sku-z",
					ShelfStatus: "ON_SHELF",
					IsActive:    true,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
				{
					TenantID:    1,
					StoreID:     101,
					SKCName:     "sku-a",
					ShelfStatus: "ON_SHELF",
					IsActive:    true,
					CreatedAt:   now.Add(time.Minute),
					UpdatedAt:   now.Add(time.Minute),
				},
				{
					TenantID:    1,
					StoreID:     101,
					SKCName:     "sku-b",
					ShelfStatus: "OFF_SHELF",
					IsActive:    false,
					CreatedAt:   now.Add(2 * time.Minute),
					UpdatedAt:   now.Add(2 * time.Minute),
				},
				{
					TenantID:    2,
					StoreID:     101,
					SKCName:     "sku-other-tenant",
					ShelfStatus: "ON_SHELF",
					IsActive:    true,
					CreatedAt:   now.Add(3 * time.Minute),
					UpdatedAt:   now.Add(3 * time.Minute),
				},
			}
			if err := harness.repo.UpsertSyncedProducts(ctx, records); err != nil {
				t.Fatalf("seed synced products: %v", err)
			}

			items, total, err := harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{
				TenantID: 1,
				StoreID:  101,
				IsActive: &activeOnly,
				Page:     1,
				PageSize: 1,
			})
			if err != nil {
				t.Fatalf("list synced products: %v", err)
			}
			if total != 2 {
				t.Fatalf("total = %d, want 2", total)
			}
			if len(items) != 1 {
				t.Fatalf("len(items) = %d, want 1", len(items))
			}
			if items[0].SKCName != "sku-a" {
				t.Fatalf("page 1 skc = %q, want sku-a", items[0].SKCName)
			}

			items, total, err = harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{
				TenantID: 1,
				StoreID:  101,
				IsActive: &activeOnly,
				Page:     2,
				PageSize: 1,
			})
			if err != nil {
				t.Fatalf("list synced products page 2: %v", err)
			}
			if total != 2 {
				t.Fatalf("page 2 total = %d, want 2", total)
			}
			if len(items) != 1 {
				t.Fatalf("page 2 len(items) = %d, want 1", len(items))
			}
			if items[0].SKCName != "sku-z" {
				t.Fatalf("page 2 skc = %q, want sku-z", items[0].SKCName)
			}
		})
	}
}

func TestSheinSyncRepositoryListMethodsHandleNilQueryWithDefaultPagination(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			startedAt := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

			for i := 0; i < 25; i++ {
				record := &listingkit.SheinSyncedProductRecord{
					TenantID: 1,
					StoreID:  int64(100 + i),
					SKCName:  "skc-" + time.Duration(i).String(),
					IsActive: true,
				}
				if err := harness.repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{record}); err != nil {
					t.Fatalf("seed synced product %d: %v", i, err)
				}
				job := &listingkit.SheinSyncJobRecord{
					TenantID:    1,
					StoreID:     int64(100 + i),
					TriggerMode: listingkit.SheinSyncTriggerModeManual,
					Status:      listingkit.SheinSyncJobStatusSucceeded,
					StartedAt:   sheinTimePtr(startedAt.Add(time.Duration(i) * time.Minute)),
				}
				if err := harness.repo.SaveSyncJob(ctx, job); err != nil {
					t.Fatalf("seed sync job %d: %v", i, err)
				}
			}

			products, total, err := harness.repo.ListSyncedProducts(ctx, nil)
			if err != nil {
				t.Fatalf("ListSyncedProducts(nil): %v", err)
			}
			if total != 25 {
				t.Fatalf("ListSyncedProducts(nil) total = %d, want 25", total)
			}
			if len(products) != 20 {
				t.Fatalf("ListSyncedProducts(nil) len = %d, want 20 default page size", len(products))
			}

			jobs, total, err := harness.repo.ListSyncJobs(ctx, nil)
			if err != nil {
				t.Fatalf("ListSyncJobs(nil): %v", err)
			}
			if total != 25 {
				t.Fatalf("ListSyncJobs(nil) total = %d, want 25", total)
			}
			if len(jobs) != 20 {
				t.Fatalf("ListSyncJobs(nil) len = %d, want 20 default page size", len(jobs))
			}

			for i := 0; i < 25; i++ {
				run := &listingkit.SheinActivityEnrollmentRunRecord{
					TenantID:     1,
					StoreID:      int64(100 + i),
					ActivityType: "PROMOTION",
					ActivityKey:  "PROMOTION:1:101",
					TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
					Status:       listingkit.SheinEnrollmentRunStatusSucceeded,
					StartedAt:    sheinTimePtr(startedAt.Add(time.Duration(i) * time.Minute)),
				}
				if err := harness.repo.CreateEnrollmentRun(ctx, run); err != nil {
					t.Fatalf("seed enrollment run %d: %v", i, err)
				}
			}

			runs, total, err := harness.repo.ListEnrollmentRuns(ctx, nil)
			if err != nil {
				t.Fatalf("ListEnrollmentRuns(nil): %v", err)
			}
			if total != 25 {
				t.Fatalf("ListEnrollmentRuns(nil) total = %d, want 25", total)
			}
			if len(runs) != 20 {
				t.Fatalf("ListEnrollmentRuns(nil) len = %d, want 20 default page size", len(runs))
			}
		})
	}
}

func TestSheinSyncRepositoryUpsertPreservesCreatedAtWhenUpdateOmitsIt(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			createdAt := time.Date(2026, 6, 4, 13, 0, 0, 0, time.UTC)

			first := &listingkit.SheinSyncedProductRecord{
				TenantID:    1,
				StoreID:     101,
				SKCName:     "skc-created-at",
				ShelfStatus: "ON_SHELF",
				IsActive:    true,
				CreatedAt:   createdAt,
				UpdatedAt:   createdAt,
			}
			if err := harness.repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{first}); err != nil {
				t.Fatalf("first upsert: %v", err)
			}

			second := &listingkit.SheinSyncedProductRecord{
				TenantID:    1,
				StoreID:     101,
				SKCName:     "skc-created-at",
				ShelfStatus: "OFF_SHELF",
				IsActive:    false,
			}
			if err := harness.repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{second}); err != nil {
				t.Fatalf("second upsert: %v", err)
			}

			items, total, err := harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{
				TenantID: 1,
				StoreID:  101,
				SKCName:  "skc-created-at",
			})
			if err != nil {
				t.Fatalf("list synced products: %v", err)
			}
			if total != 1 || len(items) != 1 {
				t.Fatalf("created_at preservation count = total %d len %d, want 1", total, len(items))
			}
			if !items[0].CreatedAt.Equal(createdAt) {
				t.Fatalf("created_at = %s, want %s", items[0].CreatedAt.Format(time.RFC3339Nano), createdAt.Format(time.RFC3339Nano))
			}
		})
	}
}

func TestSheinSyncRepositoryUpdateManualCostPriceRecomputesEffectiveCost(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			record := &listingkit.SheinSyncedProductRecord{
				TenantID:      1,
				StoreID:       101,
				SKCName:       "skc-cost",
				AutoCostPrice: float64Ptr(10.5),
				IsActive:      true,
			}
			if err := harness.repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{record}); err != nil {
				t.Fatalf("seed synced product: %v", err)
			}

			items, total, err := harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{TenantID: 1, StoreID: 101})
			if err != nil {
				t.Fatalf("list before update: %v", err)
			}
			if total != 1 || len(items) != 1 {
				t.Fatalf("before update count = total %d len %d, want 1", total, len(items))
			}

			if err := harness.repo.UpdateManualCostPrice(ctx, items[0].ID, float64Ptr(18.8)); err != nil {
				t.Fatalf("set manual cost: %v", err)
			}

			items, total, err = harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{TenantID: 1, StoreID: 101})
			if err != nil {
				t.Fatalf("list after manual set: %v", err)
			}
			if total != 1 || len(items) != 1 {
				t.Fatalf("after manual set count = total %d len %d, want 1", total, len(items))
			}
			if items[0].ManualCostPrice == nil || *items[0].ManualCostPrice != 18.8 {
				t.Fatalf("manual cost = %v, want 18.8", items[0].ManualCostPrice)
			}
			if items[0].EffectiveCostPrice == nil || *items[0].EffectiveCostPrice != 18.8 {
				t.Fatalf("effective cost = %v, want 18.8", items[0].EffectiveCostPrice)
			}
			if items[0].CostPriceSource != listingkit.SheinCostPriceSourceManual {
				t.Fatalf("cost price source = %q, want manual", items[0].CostPriceSource)
			}

			if err := harness.repo.UpdateManualCostPrice(ctx, items[0].ID, nil); err != nil {
				t.Fatalf("clear manual cost: %v", err)
			}

			items, total, err = harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{TenantID: 1, StoreID: 101})
			if err != nil {
				t.Fatalf("list after manual clear: %v", err)
			}
			if total != 1 || len(items) != 1 {
				t.Fatalf("after manual clear count = total %d len %d, want 1", total, len(items))
			}
			if items[0].ManualCostPrice != nil {
				t.Fatalf("manual cost = %v, want nil", items[0].ManualCostPrice)
			}
			if items[0].EffectiveCostPrice == nil || *items[0].EffectiveCostPrice != 10.5 {
				t.Fatalf("effective cost = %v, want 10.5", items[0].EffectiveCostPrice)
			}
			if items[0].CostPriceSource != listingkit.SheinCostPriceSourceAuto {
				t.Fatalf("cost price source = %q, want auto", items[0].CostPriceSource)
			}
		})
	}
}

func TestSheinSyncRepositoryMarkMissingSyncedProductsInactive(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if err := harness.repo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{
				{TenantID: 1, StoreID: 101, SKCName: "keep-active", IsActive: true, ShelfStatus: "ON_SHELF"},
				{TenantID: 1, StoreID: 101, SKCName: "go-inactive", IsActive: true, ShelfStatus: "ON_SHELF"},
				{TenantID: 1, StoreID: 202, SKCName: "other-store", IsActive: true, ShelfStatus: "ON_SHELF"},
			}); err != nil {
				t.Fatalf("seed synced products: %v", err)
			}

			if err := harness.repo.MarkMissingSyncedProductsInactive(ctx, 1, 101, []string{"keep-active"}); err != nil {
				t.Fatalf("mark missing inactive: %v", err)
			}

			items, total, err := harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{TenantID: 1, StoreID: 101})
			if err != nil {
				t.Fatalf("list target store: %v", err)
			}
			if total != 2 || len(items) != 2 {
				t.Fatalf("target store count = total %d len %d, want 2", total, len(items))
			}

			statusBySKC := map[string]bool{}
			for _, item := range items {
				statusBySKC[item.SKCName] = item.IsActive
			}
			if !statusBySKC["keep-active"] {
				t.Fatalf("keep-active marked inactive unexpectedly")
			}
			if statusBySKC["go-inactive"] {
				t.Fatalf("go-inactive remained active unexpectedly")
			}

			otherStore, total, err := harness.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{TenantID: 1, StoreID: 202})
			if err != nil {
				t.Fatalf("list other store: %v", err)
			}
			if total != 1 || len(otherStore) != 1 || !otherStore[0].IsActive {
				t.Fatalf("other store items = %+v total=%d, want untouched active row", otherStore, total)
			}
		})
	}
}

func TestSheinSyncRepositorySaveSyncJobAndListHistory(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			startedAt := time.Date(2026, 6, 4, 11, 0, 0, 0, time.UTC)
			finishedAt := startedAt.Add(2 * time.Minute)

			job := &listingkit.SheinSyncJobRecord{
				TenantID:     1,
				StoreID:      101,
				TriggerMode:  listingkit.SheinSyncTriggerModeManual,
				Status:       listingkit.SheinSyncJobStatusRunning,
				StartedAt:    &startedAt,
				FetchedCount: 3,
			}
			if err := harness.repo.SaveSyncJob(ctx, job); err != nil {
				t.Fatalf("save new sync job: %v", err)
			}
			if job.ID <= 0 {
				t.Fatalf("job id = %d, want > 0", job.ID)
			}

			job.Status = listingkit.SheinSyncJobStatusSucceeded
			job.FinishedAt = &finishedAt
			job.InsertedCount = 2
			job.UpdatedCount = 1
			if err := harness.repo.SaveSyncJob(ctx, job); err != nil {
				t.Fatalf("update sync job: %v", err)
			}

			if err := harness.repo.SaveSyncJob(ctx, &listingkit.SheinSyncJobRecord{
				TenantID:    1,
				StoreID:     202,
				TriggerMode: listingkit.SheinSyncTriggerModeSchedule,
				Status:      listingkit.SheinSyncJobStatusFailed,
				StartedAt:   sheinTimePtr(startedAt.Add(time.Hour)),
			}); err != nil {
				t.Fatalf("save other store job: %v", err)
			}

			items, total, err := harness.repo.ListSyncJobs(ctx, &listingkit.SheinSyncJobQuery{
				TenantID: 1,
				StoreID:  101,
			})
			if err != nil {
				t.Fatalf("list sync jobs: %v", err)
			}
			if total != 1 || len(items) != 1 {
				t.Fatalf("jobs = %+v total=%d, want one scoped job", items, total)
			}
			if items[0].Status != listingkit.SheinSyncJobStatusSucceeded {
				t.Fatalf("job status = %q, want succeeded", items[0].Status)
			}
			if items[0].InsertedCount != 2 || items[0].UpdatedCount != 1 {
				t.Fatalf("job counters = %+v, want inserted=2 updated=1", items[0])
			}
		})
	}
}

func TestSheinSyncRepositorySaveCandidatesAndCreateEnrollmentRun(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			costA := 12.3
			costB := 18.8
			if err := harness.repo.SaveCandidates(ctx, []*listingkit.SheinActivityCandidateRecord{
				{
					TenantID:           1,
					StoreID:            101,
					SyncedProductID:    11,
					ActivityType:       "PROMOTION",
					ActivityKey:        "PROMO-1",
					SKCName:            "skc-1",
					CandidateVersion:   "v1",
					EffectiveCostPrice: &costA,
					EligibilityStatus:  listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:       listingkit.SheinCandidateReviewStatusPendingReview,
				},
				{
					TenantID:           1,
					StoreID:            101,
					SyncedProductID:    12,
					ActivityType:       "PROMOTION",
					ActivityKey:        "PROMO-1",
					SKCName:            "skc-2",
					CandidateVersion:   "v1",
					EffectiveCostPrice: &costB,
					EligibilityStatus:  listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:       listingkit.SheinCandidateReviewStatusApproved,
				},
			}); err != nil {
				t.Fatalf("save candidates: %v", err)
			}

			if err := harness.repo.SaveCandidates(ctx, []*listingkit.SheinActivityCandidateRecord{
				{
					TenantID:           1,
					StoreID:            101,
					SyncedProductID:    11,
					ActivityType:       "PROMOTION",
					ActivityKey:        "PROMO-1",
					SKCName:            "skc-1",
					CandidateVersion:   "v1",
					EffectiveCostPrice: &costA,
					EligibilityStatus:  listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:       listingkit.SheinCandidateReviewStatusApproved,
				},
			}); err != nil {
				t.Fatalf("upsert candidate: %v", err)
			}

			candidates, total, err := harness.listCandidates(ctx, &listingkit.SheinActivityCandidateQuery{
				TenantID:     1,
				StoreID:      101,
				ActivityType: "PROMOTION",
				Page:         1,
				PageSize:     10,
			})
			if err != nil {
				t.Fatalf("list candidates: %v", err)
			}
			if len(candidates) != 2 || total != 2 {
				t.Fatalf("len(candidates) = %d total=%d, want 2", len(candidates), total)
			}
			approvedCount := 0
			for _, candidate := range candidates {
				if candidate.TenantID != 1 || candidate.StoreID != 101 {
					t.Fatalf("candidate scope = (%d,%d), want (1,101)", candidate.TenantID, candidate.StoreID)
				}
				if candidate.SKCName == "skc-1" && candidate.ReviewStatus == listingkit.SheinCandidateReviewStatusApproved {
					approvedCount++
				}
			}
			if approvedCount != 1 {
				t.Fatalf("approved count for skc-1 = %d, want 1 upserted record", approvedCount)
			}

			run := &listingkit.SheinActivityEnrollmentRunRecord{
				TenantID:       1,
				StoreID:        101,
				ActivityType:   "PROMOTION",
				ActivityKey:    "PROMO-1",
				TriggerMode:    listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
				Status:         listingkit.SheinEnrollmentRunStatusRunning,
				CandidateCount: 2,
			}
			if err := harness.repo.CreateEnrollmentRun(ctx, run); err != nil {
				t.Fatalf("create enrollment run: %v", err)
			}
			if run.ID <= 0 {
				t.Fatalf("run id = %d, want > 0", run.ID)
			}

			runs := harness.listRuns(t)
			if len(runs) != 1 {
				t.Fatalf("len(runs) = %d, want 1", len(runs))
			}
			if runs[0].TenantID != 1 || runs[0].StoreID != 101 {
				t.Fatalf("run scope = (%d,%d), want (1,101)", runs[0].TenantID, runs[0].StoreID)
			}
			if runs[0].CandidateCount != 2 {
				t.Fatalf("candidate count = %d, want 2", runs[0].CandidateCount)
			}
		})
	}
}

func TestSheinSyncRepositorySaveEnrollmentItemsPreservesRunHistory(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			firstRun := &listingkit.SheinActivityEnrollmentRunRecord{
				TenantID:       1,
				StoreID:        101,
				ActivityType:   "PROMOTION",
				ActivityKey:    "PROMO-1",
				TriggerMode:    listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
				Status:         listingkit.SheinEnrollmentRunStatusSucceeded,
				CandidateCount: 1,
			}
			secondRun := &listingkit.SheinActivityEnrollmentRunRecord{
				TenantID:       1,
				StoreID:        101,
				ActivityType:   "PROMOTION",
				ActivityKey:    "PROMO-1",
				TriggerMode:    listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
				Status:         listingkit.SheinEnrollmentRunStatusFailed,
				CandidateCount: 1,
			}
			if err := harness.repo.CreateEnrollmentRun(ctx, firstRun); err != nil {
				t.Fatalf("create first run: %v", err)
			}
			if err := harness.repo.CreateEnrollmentRun(ctx, secondRun); err != nil {
				t.Fatalf("create second run: %v", err)
			}

			saveItems := harness.repo.(interface {
				SaveEnrollmentItems(ctx context.Context, items []*listingkit.SheinActivityEnrollmentItemRecord) error
			})
			if err := saveItems.SaveEnrollmentItems(ctx, []*listingkit.SheinActivityEnrollmentItemRecord{
				{
					RunID:            firstRun.ID,
					CandidateID:      1,
					StoreID:          101,
					ActivityKey:      "PROMO-1",
					CandidateVersion: "v1",
					SyncedProductID:  11,
					SKCName:          "skc-1",
					Status:           listingkit.SheinEnrollmentItemStatusSucceeded,
				},
				{
					RunID:            secondRun.ID,
					CandidateID:      1,
					StoreID:          101,
					ActivityKey:      "PROMO-1",
					CandidateVersion: "v1",
					SyncedProductID:  11,
					SKCName:          "skc-1",
					Status:           listingkit.SheinEnrollmentItemStatusFailed,
				},
			}); err != nil {
				t.Fatalf("save enrollment history items: %v", err)
			}

			savedItems := harness.listItems(t)
			if len(savedItems) != 2 {
				t.Fatalf("len(saved items) = %d, want 2 run-scoped history rows", len(savedItems))
			}
			if savedItems[0].RunID == savedItems[1].RunID {
				t.Fatalf("saved items share run id unexpectedly: %+v", savedItems)
			}
		})
	}
}

func TestSheinSyncRepositoryListEnrollmentRunsSupportsFilteringAndPagination(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			startedAt := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
			failedStatus := listingkit.SheinEnrollmentRunStatusFailed

			seedRuns := []*listingkit.SheinActivityEnrollmentRunRecord{
				{
					TenantID:     1,
					StoreID:      101,
					ActivityType: "PROMOTION",
					ActivityKey:  "PROMOTION:1:101",
					TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
					Status:       listingkit.SheinEnrollmentRunStatusSucceeded,
					StartedAt:    sheinTimePtr(startedAt),
				},
				{
					TenantID:     1,
					StoreID:      101,
					ActivityType: "PROMOTION",
					ActivityKey:  "PROMOTION:1:101",
					TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
					Status:       listingkit.SheinEnrollmentRunStatusFailed,
					StartedAt:    sheinTimePtr(startedAt.Add(time.Minute)),
				},
				{
					TenantID:     1,
					StoreID:      101,
					ActivityType: "TIME_LIMITED",
					ActivityKey:  "TIME_LIMITED:1:101",
					TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeAutoSchedule,
					Status:       listingkit.SheinEnrollmentRunStatusSucceeded,
					StartedAt:    sheinTimePtr(startedAt.Add(2 * time.Minute)),
				},
				{
					TenantID:     2,
					StoreID:      101,
					ActivityType: "PROMOTION",
					ActivityKey:  "PROMOTION:2:101",
					TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
					Status:       listingkit.SheinEnrollmentRunStatusSucceeded,
					StartedAt:    sheinTimePtr(startedAt.Add(3 * time.Minute)),
				},
			}
			for _, run := range seedRuns {
				if err := harness.repo.CreateEnrollmentRun(ctx, run); err != nil {
					t.Fatalf("seed enrollment run: %v", err)
				}
			}

			rows, total, err := harness.repo.ListEnrollmentRuns(ctx, &listingkit.SheinEnrollmentRunQuery{
				TenantID:     1,
				StoreID:      101,
				ActivityType: "PROMOTION",
				Status:       &failedStatus,
				Page:         1,
				PageSize:     10,
			})
			if err != nil {
				t.Fatalf("ListEnrollmentRuns(filter): %v", err)
			}
			if total != 1 || len(rows) != 1 {
				t.Fatalf("ListEnrollmentRuns(filter) total=%d len=%d, want 1", total, len(rows))
			}
			if rows[0].Status != listingkit.SheinEnrollmentRunStatusFailed {
				t.Fatalf("filtered run status = %q, want failed", rows[0].Status)
			}

			rows, total, err = harness.repo.ListEnrollmentRuns(ctx, &listingkit.SheinEnrollmentRunQuery{
				TenantID: 1,
				StoreID:  101,
				Page:     1,
				PageSize: 2,
			})
			if err != nil {
				t.Fatalf("ListEnrollmentRuns(page1): %v", err)
			}
			if total != 3 || len(rows) != 2 {
				t.Fatalf("ListEnrollmentRuns(page1) total=%d len=%d, want total 3 len 2", total, len(rows))
			}
			if rows[0].ActivityType != "TIME_LIMITED" {
				t.Fatalf("page1 first activity_type = %q, want TIME_LIMITED", rows[0].ActivityType)
			}

			rows, total, err = harness.repo.ListEnrollmentRuns(ctx, &listingkit.SheinEnrollmentRunQuery{
				TenantID: 1,
				StoreID:  101,
				Page:     2,
				PageSize: 2,
			})
			if err != nil {
				t.Fatalf("ListEnrollmentRuns(page2): %v", err)
			}
			if total != 3 || len(rows) != 1 {
				t.Fatalf("ListEnrollmentRuns(page2) total=%d len=%d, want total 3 len 1", total, len(rows))
			}
			if rows[0].StartedAt == nil || !rows[0].StartedAt.Equal(startedAt) {
				t.Fatalf("page2 run started_at = %v, want %v", rows[0].StartedAt, startedAt)
			}
		})
	}
}

func TestSheinSyncRepositoryListCandidatesByIDsAndPersistEnrollmentOutcome(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if err := harness.repo.SaveCandidates(ctx, []*listingkit.SheinActivityCandidateRecord{
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   11,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMO-1",
					SKCName:           "skc-1",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusApproved,
				},
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   12,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMO-2",
					SKCName:           "skc-2",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusApproved,
				},
			}); err != nil {
				t.Fatalf("seed candidates: %v", err)
			}

			items, total, err := harness.listCandidates(ctx, &listingkit.SheinActivityCandidateQuery{
				TenantID:     1,
				StoreID:      101,
				ActivityType: "PROMOTION",
				ActivityKey:  "PROMO-1",
				CandidateIDs: []int64{1, 999},
				Page:         1,
				PageSize:     10,
			})
			if err != nil {
				t.Fatalf("list candidates by ids: %v", err)
			}
			if total != 1 || len(items) != 1 {
				t.Fatalf("candidate by ids count = total %d len %d, want 1", total, len(items))
			}
			if items[0].ActivityKey != "PROMO-1" || items[0].ID != 1 {
				t.Fatalf("candidate returned = %+v, want candidate 1 scoped to PROMO-1", items[0])
			}

			run := &listingkit.SheinActivityEnrollmentRunRecord{
				TenantID:       1,
				StoreID:        101,
				ActivityType:   "PROMOTION",
				ActivityKey:    "PROMO-1",
				TriggerMode:    listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
				Status:         listingkit.SheinEnrollmentRunStatusRunning,
				CandidateCount: 1,
			}
			if err := harness.repo.CreateEnrollmentRun(ctx, run); err != nil {
				t.Fatalf("create enrollment run: %v", err)
			}

			if err := harness.repo.(interface {
				UpdateEnrollmentRun(ctx context.Context, run *listingkit.SheinActivityEnrollmentRunRecord) error
			}).UpdateEnrollmentRun(ctx, &listingkit.SheinActivityEnrollmentRunRecord{
				ID:             run.ID,
				TenantID:       1,
				StoreID:        101,
				ActivityType:   "PROMOTION",
				ActivityKey:    "PROMO-1",
				TriggerMode:    listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
				Status:         listingkit.SheinEnrollmentRunStatusSucceeded,
				CandidateCount: 1,
				SubmittedCount: 1,
				SucceededCount: 1,
				ErrorSummary:   "finished",
				CreatedAt:      run.CreatedAt,
			}); err != nil {
				t.Fatalf("update enrollment run: %v", err)
			}

			if err := harness.repo.(interface {
				SaveEnrollmentItems(ctx context.Context, items []*listingkit.SheinActivityEnrollmentItemRecord) error
			}).SaveEnrollmentItems(ctx, []*listingkit.SheinActivityEnrollmentItemRecord{
				{
					RunID:            run.ID,
					CandidateID:      1,
					StoreID:          101,
					ActivityKey:      "PROMO-1",
					CandidateVersion: "v1",
					SyncedProductID:  11,
					SKCName:          "skc-1",
					Status:           listingkit.SheinEnrollmentItemStatusSucceeded,
					ResponsePayload:  `{"code":"0"}`,
				},
			}); err != nil {
				t.Fatalf("save enrollment items: %v", err)
			}

			runs := harness.listRuns(t)
			if len(runs) != 1 {
				t.Fatalf("len(runs) = %d, want 1", len(runs))
			}
			if runs[0].Status != listingkit.SheinEnrollmentRunStatusSucceeded {
				t.Fatalf("run status = %q, want succeeded", runs[0].Status)
			}
			if runs[0].ActivityKey != "PROMO-1" {
				t.Fatalf("run activity key = %q, want PROMO-1", runs[0].ActivityKey)
			}

			savedItems := harness.listItems(t)
			if len(savedItems) != 1 {
				t.Fatalf("len(saved items) = %d, want 1", len(savedItems))
			}
			if savedItems[0].ActivityKey != "PROMO-1" || savedItems[0].CandidateID != 1 {
				t.Fatalf("saved item = %+v, want candidate 1 / PROMO-1", savedItems[0])
			}
		})
	}
}

func TestSheinSyncRepositoryListCandidatesSupportsFilteringAndSameVersionUpsert(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if err := harness.repo.SaveCandidates(ctx, []*listingkit.SheinActivityCandidateRecord{
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   1,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMOTION:1:101",
					SKCName:           "skc-a",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusPendingReview,
				},
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   2,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMOTION:1:101",
					SKCName:           "skc-b",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusPendingReview,
				},
				{
					TenantID:          1,
					StoreID:           202,
					SyncedProductID:   3,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMOTION:1:202",
					SKCName:           "other-store",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusPendingReview,
				},
			}); err != nil {
				t.Fatalf("seed candidates: %v", err)
			}

			if err := harness.repo.SaveCandidates(ctx, []*listingkit.SheinActivityCandidateRecord{
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   1,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMOTION:1:101",
					SKCName:           "skc-a",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusApproved,
					AutoModeEligible:  true,
					SelectedForRun:    true,
				},
			}); err != nil {
				t.Fatalf("same-version upsert candidate: %v", err)
			}

			items, total, err := harness.listCandidates(ctx, &listingkit.SheinActivityCandidateQuery{
				TenantID:     1,
				StoreID:      101,
				ActivityType: "PROMOTION",
				Page:         1,
				PageSize:     10,
			})
			if err != nil {
				t.Fatalf("list candidates: %v", err)
			}
			if total != 2 || len(items) != 2 {
				t.Fatalf("candidate count = total %d len %d, want 2", total, len(items))
			}

			for _, item := range items {
				if item.SKCName != "skc-a" {
					continue
				}
				if item.ReviewStatus != listingkit.SheinCandidateReviewStatusApproved {
					t.Fatalf("same-version review status = %q, want approved", item.ReviewStatus)
				}
				if !item.AutoModeEligible || !item.SelectedForRun {
					t.Fatalf("same-version workflow flags = auto:%v selected:%v, want true/true", item.AutoModeEligible, item.SelectedForRun)
				}
			}
		})
	}
}

func TestSheinSyncRepositorySaveCandidatesAllowsSupersedingOlderVersions(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		harness := harness
		t.Run(harness.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if err := harness.repo.SaveCandidates(ctx, []*listingkit.SheinActivityCandidateRecord{
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   1,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMOTION:1:101",
					SKCName:           "skc-a",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusApproved,
					AutoModeEligible:  true,
					SelectedForRun:    true,
				},
			}); err != nil {
				t.Fatalf("seed old candidate: %v", err)
			}

			if err := harness.repo.SaveCandidates(ctx, []*listingkit.SheinActivityCandidateRecord{
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   1,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMOTION:1:101",
					SKCName:           "skc-a",
					CandidateVersion:  "v1",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusIneligible,
					EligibilityReason: "superseded by newer candidate version",
					ReviewStatus:      listingkit.SheinCandidateReviewStatusRejected,
					AutoModeEligible:  false,
					SelectedForRun:    false,
				},
				{
					TenantID:          1,
					StoreID:           101,
					SyncedProductID:   1,
					ActivityType:      "PROMOTION",
					ActivityKey:       "PROMOTION:1:101",
					SKCName:           "skc-a",
					CandidateVersion:  "v2",
					EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
					ReviewStatus:      listingkit.SheinCandidateReviewStatusPendingReview,
					AutoModeEligible:  false,
					SelectedForRun:    false,
				},
			}); err != nil {
				t.Fatalf("save versioned candidates: %v", err)
			}

			items, total, err := harness.listCandidates(ctx, &listingkit.SheinActivityCandidateQuery{
				TenantID:     1,
				StoreID:      101,
				ActivityType: "PROMOTION",
				SKCName:      "skc-a",
				Page:         1,
				PageSize:     10,
			})
			if err != nil {
				t.Fatalf("list versioned candidates: %v", err)
			}
			if total != 2 || len(items) != 2 {
				t.Fatalf("versioned candidate count = total %d len %d, want 2", total, len(items))
			}

			var oldVersion, newVersion *listingkit.SheinActivityCandidateRecord
			for i := range items {
				if items[i].CandidateVersion == "v1" {
					oldVersion = &items[i]
				}
				if items[i].CandidateVersion == "v2" {
					newVersion = &items[i]
				}
			}
			if oldVersion == nil || newVersion == nil {
				t.Fatalf("missing expected versions: %+v", items)
			}
			if oldVersion.ReviewStatus != listingkit.SheinCandidateReviewStatusRejected || oldVersion.AutoModeEligible || oldVersion.SelectedForRun {
				t.Fatalf("old version not superseded correctly: %+v", *oldVersion)
			}
			if newVersion.ReviewStatus != listingkit.SheinCandidateReviewStatusPendingReview || newVersion.AutoModeEligible || newVersion.SelectedForRun {
				t.Fatalf("new version state incorrect: %+v", *newVersion)
			}
		})
	}
}

func TestSheinSyncRepositoryUpsertsAndListsSDSCostGroups(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		t.Run(harness.name, func(t *testing.T) {
			ctx := context.Background()
			repo := harness.repo
			cost := 46.8
			updated := 50.0

			groupRepo, ok := repo.(interface {
				UpsertSDSCostGroup(ctx context.Context, record *listingkit.SheinSDSCostGroupRecord) error
				ListSDSCostGroups(ctx context.Context, query *listingkit.SheinSDSCostGroupQuery) ([]listingkit.SheinSDSCostGroupRecord, int64, error)
			})
			if !ok {
				t.Fatalf("%s repo does not implement SDS cost groups", harness.name)
			}

			if err := groupRepo.UpsertSDSCostGroup(ctx, &listingkit.SheinSDSCostGroupRecord{
				TenantID:        11,
				StoreID:         22,
				GroupKey:        "style:B3195DA6",
				GroupLabel:      "B3195DA6",
				ManualCostPrice: &cost,
			}); err != nil {
				t.Fatalf("upsert SDS cost group: %v", err)
			}
			if err := groupRepo.UpsertSDSCostGroup(ctx, &listingkit.SheinSDSCostGroupRecord{
				TenantID:        11,
				StoreID:         22,
				GroupKey:        "style:B3195DA6",
				GroupLabel:      "B3195DA6",
				ManualCostPrice: &updated,
			}); err != nil {
				t.Fatalf("update SDS cost group: %v", err)
			}

			rows, total, err := groupRepo.ListSDSCostGroups(ctx, &listingkit.SheinSDSCostGroupQuery{
				TenantID:  11,
				StoreID:   22,
				GroupKeys: []string{"style:B3195DA6"},
			})
			if err != nil {
				t.Fatalf("list SDS cost groups: %v", err)
			}
			if total != 1 || len(rows) != 1 {
				t.Fatalf("groups total=%d len=%d, want 1", total, len(rows))
			}
			if rows[0].ManualCostPrice == nil || *rows[0].ManualCostPrice != updated {
				t.Fatalf("manual cost = %+v, want %.1f", rows[0].ManualCostPrice, updated)
			}
		})
	}
}

func TestSheinSyncRepositoryListsSourceSDSCostGroupsFromSyncedProducts(t *testing.T) {
	t.Parallel()

	for _, harness := range sheinSyncRepositoryHarnesses(t) {
		t.Run(harness.name, func(t *testing.T) {
			ctx := context.Background()
			repo := harness.repo
			legacyCost := 43.3

			sourceRepo, ok := repo.(interface {
				UpsertSyncedProducts(ctx context.Context, records []*listingkit.SheinSyncedProductRecord) error
				UpsertSDSCostGroup(ctx context.Context, record *listingkit.SheinSDSCostGroupRecord) error
				ListSourceSDSCostGroups(ctx context.Context, query *listingkit.SheinSourceSDSCostGroupQuery) ([]listingkit.SheinSourceSDSCostGroupRecord, int64, error)
			})
			if !ok {
				t.Fatalf("%s repo does not implement source SDS cost groups", harness.name)
			}

			if err := sourceRepo.UpsertSyncedProducts(ctx, []*listingkit.SheinSyncedProductRecord{
				{
					TenantID:     11,
					StoreID:      22,
					SKCName:      "sg260604223794143925005",
					SupplierCode: "XB0608021001-DA578653",
					IsActive:     true,
				},
				{
					TenantID:     11,
					StoreID:      22,
					SKCName:      "sg260603162031320517713",
					SupplierCode: "XB0608021001-DE93508C",
					IsActive:     true,
				},
				{
					TenantID:     11,
					StoreID:      22,
					SKCName:      "sg-other",
					SupplierCode: "XB0608021002-0199A07E",
					IsActive:     true,
				},
			}); err != nil {
				t.Fatalf("seed synced products: %v", err)
			}
			if err := sourceRepo.UpsertSDSCostGroup(ctx, &listingkit.SheinSDSCostGroupRecord{
				TenantID:        11,
				StoreID:         22,
				GroupKey:        "style:DA578653",
				GroupLabel:      "DA578653",
				ManualCostPrice: &legacyCost,
			}); err != nil {
				t.Fatalf("seed legacy SDS cost group: %v", err)
			}

			rows, total, err := sourceRepo.ListSourceSDSCostGroups(ctx, &listingkit.SheinSourceSDSCostGroupQuery{
				TenantID: 11,
				StoreID:  22,
				Page:     1,
				PageSize: 10,
			})
			if err != nil {
				t.Fatalf("list source SDS cost groups: %v", err)
			}
			if total != 2 || len(rows) != 2 {
				t.Fatalf("groups total=%d len=%d, want 2", total, len(rows))
			}
			if rows[0].GroupKey != "source:XB0608021001" {
				t.Fatalf("first group key = %q, want source:XB0608021001", rows[0].GroupKey)
			}
			if rows[0].ProductCount != 2 {
				t.Fatalf("first product count = %d, want 2", rows[0].ProductCount)
			}
			if rows[0].ManualCostPrice == nil || *rows[0].ManualCostPrice != legacyCost {
				t.Fatalf("first manual cost = %+v, want %.1f", rows[0].ManualCostPrice, legacyCost)
			}
			if len(rows[0].Products) != 2 {
				t.Fatalf("first sample products len = %d, want 2", len(rows[0].Products))
			}
		})
	}
}

func sheinSyncRepositoryHarnesses(t *testing.T) []sheinSyncRepositoryHarness {
	t.Helper()

	return []sheinSyncRepositoryHarness{
		newGormSheinSyncRepositoryHarness(t),
		newMemSheinSyncRepositoryHarness(),
	}
}

func newGormSheinSyncRepositoryHarness(t *testing.T) sheinSyncRepositoryHarness {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateSheinSyncRepository(db); err != nil {
		t.Fatalf("auto migrate shein sync repository: %v", err)
	}

	return sheinSyncRepositoryHarness{
		name: "gorm",
		repo: NewSheinSyncRepository(db),
		listCandidates: func(ctx context.Context, query *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error) {
			return NewSheinSyncRepository(db).(interface {
				ListCandidates(ctx context.Context, query *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error)
			}).ListCandidates(ctx, query)
		},
		listRuns: func(t *testing.T) []listingkit.SheinActivityEnrollmentRunRecord {
			t.Helper()

			var rows []listingkit.SheinActivityEnrollmentRunRecord
			if err := db.Order("id ASC").Find(&rows).Error; err != nil {
				t.Fatalf("list enrollment runs: %v", err)
			}
			return rows
		},
		listItems: func(t *testing.T) []listingkit.SheinActivityEnrollmentItemRecord {
			t.Helper()

			var rows []listingkit.SheinActivityEnrollmentItemRecord
			if err := db.Order("id ASC").Find(&rows).Error; err != nil {
				t.Fatalf("list enrollment items: %v", err)
			}
			return rows
		},
	}
}

func newMemSheinSyncRepositoryHarness() sheinSyncRepositoryHarness {
	repo := NewMemSheinSyncRepository()
	memRepo := repo.(*MemSheinSyncRepository)

	return sheinSyncRepositoryHarness{
		name: "mem",
		repo: repo,
		listCandidates: func(ctx context.Context, query *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error) {
			return memRepo.ListCandidates(ctx, query)
		},
		listRuns: func(t *testing.T) []listingkit.SheinActivityEnrollmentRunRecord {
			t.Helper()

			memRepo.mu.RLock()
			defer memRepo.mu.RUnlock()

			rows := make([]listingkit.SheinActivityEnrollmentRunRecord, 0, len(memRepo.enrollmentRuns))
			for _, row := range memRepo.enrollmentRuns {
				rows = append(rows, row)
			}
			sortSheinEnrollmentRuns(rows)
			return rows
		},
		listItems: func(t *testing.T) []listingkit.SheinActivityEnrollmentItemRecord {
			t.Helper()

			memRepo.mu.RLock()
			defer memRepo.mu.RUnlock()

			rows := make([]listingkit.SheinActivityEnrollmentItemRecord, 0, len(memRepo.enrollmentItems))
			for _, row := range memRepo.enrollmentItems {
				rows = append(rows, row)
			}
			sortSheinEnrollmentItems(rows)
			return rows
		},
	}
}

func sheinTimePtr(v time.Time) *time.Time {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func sortSheinCandidates(rows []listingkit.SheinActivityCandidateRecord) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID != rows[j].ID {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].SKCName < rows[j].SKCName
	})
}

func sortSheinEnrollmentRuns(rows []listingkit.SheinActivityEnrollmentRunRecord) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID != rows[j].ID {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].ActivityType < rows[j].ActivityType
	})
}

func sortSheinEnrollmentItems(rows []listingkit.SheinActivityEnrollmentItemRecord) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID != rows[j].ID {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CandidateID < rows[j].CandidateID
	})
}
