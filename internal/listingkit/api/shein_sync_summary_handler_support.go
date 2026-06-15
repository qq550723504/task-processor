package api

import (
	"context"
	"strings"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

func resolveSheinSummaryActivityType(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultSheinSummaryActivityType
	}
	return trimmed
}

func (h *handler) listSheinStores(ctx context.Context, tenantID int64) ([]listingadmin.Store, error) {
	items := make([]listingadmin.Store, 0)
	page := 1
	for {
		storePage, err := h.storeRepository.ListStores(ctx, listingadmin.StoreQuery{
			TenantID: tenantID,
			Platform: "SHEIN",
			Page:     page,
			PageSize: 200,
		})
		if err != nil {
			return nil, err
		}
		if storePage == nil || len(storePage.Items) == 0 {
			break
		}
		items = append(items, storePage.Items...)
		if int64(page*storePage.PageSize) >= storePage.Total || len(storePage.Items) < storePage.PageSize {
			break
		}
		page++
	}
	return items, nil
}

func (h *handler) buildSheinEnrollmentStoreSummary(
	ctx context.Context,
	tenantID int64,
	store *listingadmin.Store,
	activityType string,
) (*listingkit.SheinEnrollmentStoreSummary, error) {
	if store == nil {
		return &listingkit.SheinEnrollmentStoreSummary{ActivityType: activityType}, nil
	}

	products, err := h.listActiveSheinSyncedProducts(ctx, tenantID, store.ID)
	if err != nil {
		return nil, err
	}
	candidates, err := h.listSheinActivityCandidatesForSummary(ctx, tenantID, store.ID, activityType)
	if err != nil {
		return nil, err
	}
	lastSyncJob, err := h.getLatestSheinSyncJob(ctx, tenantID, store.ID)
	if err != nil {
		return nil, err
	}
	lastRun, err := h.getLatestSheinEnrollmentRun(ctx, tenantID, store.ID, activityType)
	if err != nil {
		return nil, err
	}

	summary := &listingkit.SheinEnrollmentStoreSummary{
		StoreID:            store.ID,
		StoreName:          strings.TrimSpace(store.Name),
		StoreUsername:      strings.TrimSpace(store.Username),
		Platform:           strings.TrimSpace(store.Platform),
		Region:             strings.TrimSpace(store.Region),
		EnableAutoListing:  store.EnableAutoListing,
		ActivityType:       activityType,
		SyncedProductCount: len(products),
		MissingCostCount:   countSheinProductsMissingCost(products),
	}
	pendingReviewCount, readyToEnrollCount := summarizeLatestSheinCandidates(candidates)
	summary.PendingReviewCount = pendingReviewCount
	summary.ReadyToEnrollCount = readyToEnrollCount
	if lastSyncJob != nil {
		summary.LastSyncJob = lastSyncJob
		summary.LastSyncStatus = lastSyncJob.Status
		summary.LastSyncAt = preferSheinTime(lastSyncJob.FinishedAt, lastSyncJob.StartedAt)
	}
	if lastRun != nil {
		summary.LastEnrollmentRun = lastRun
		summary.LastEnrollmentAt = preferSheinTime(lastRun.FinishedAt, lastRun.StartedAt)
	}
	return summary, nil
}

func (h *handler) listActiveSheinSyncedProducts(ctx context.Context, tenantID, storeID int64) ([]listingkit.SheinSyncedProductRecord, error) {
	active := true
	items := make([]listingkit.SheinSyncedProductRecord, 0)
	page := 1
	for {
		rows, total, err := h.sheinSyncRepository.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{
			TenantID: tenantID,
			StoreID:  storeID,
			IsActive: &active,
			Page:     page,
			PageSize: sheinSummaryPageSize,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, rows...)
		if len(rows) == 0 || int64(page*sheinSummaryPageSize) >= total {
			break
		}
		page++
	}
	return items, nil
}

func (h *handler) listSheinActivityCandidatesForSummary(ctx context.Context, tenantID, storeID int64, activityType string) ([]listingkit.SheinActivityCandidateRecord, error) {
	items := make([]listingkit.SheinActivityCandidateRecord, 0)
	page := 1
	for {
		rows, total, err := h.sheinSyncRepository.ListCandidates(ctx, &listingkit.SheinActivityCandidateQuery{
			TenantID:     tenantID,
			StoreID:      storeID,
			ActivityType: activityType,
			Page:         page,
			PageSize:     sheinSummaryPageSize,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, rows...)
		if len(rows) == 0 || int64(page*sheinSummaryPageSize) >= total {
			break
		}
		page++
	}
	return items, nil
}

func (h *handler) getLatestSheinSyncJob(ctx context.Context, tenantID, storeID int64) (*listingkit.SheinSyncJobRecord, error) {
	rows, _, err := h.sheinSyncRepository.ListSyncJobs(ctx, &listingkit.SheinSyncJobQuery{
		TenantID: tenantID,
		StoreID:  storeID,
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	return &row, nil
}

func (h *handler) getLatestSheinEnrollmentRun(ctx context.Context, tenantID, storeID int64, activityType string) (*listingkit.SheinActivityEnrollmentRunRecord, error) {
	rows, _, err := h.sheinSyncRepository.ListEnrollmentRuns(ctx, &listingkit.SheinEnrollmentRunQuery{
		TenantID:     tenantID,
		StoreID:      storeID,
		ActivityType: activityType,
		Page:         1,
		PageSize:     1,
	})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	return &row, nil
}

func countSheinProductsMissingCost(items []listingkit.SheinSyncedProductRecord) int {
	count := 0
	for _, item := range items {
		if item.EffectiveCostPrice == nil {
			count++
		}
	}
	return count
}

func summarizeLatestSheinCandidates(items []listingkit.SheinActivityCandidateRecord) (pendingReviewCount int, readyToEnrollCount int) {
	latestBySKC := make(map[string]listingkit.SheinActivityCandidateRecord, len(items))
	for _, item := range items {
		current, ok := latestBySKC[item.SKCName]
		if !ok || compareSheinCandidateFreshness(item, current) > 0 {
			latestBySKC[item.SKCName] = item
		}
	}
	for _, item := range latestBySKC {
		if item.ReviewStatus == listingkit.SheinCandidateReviewStatusPendingReview {
			pendingReviewCount++
		}
		if item.EligibilityStatus == listingkit.SheinCandidateEligibilityStatusEligible &&
			(item.ReviewStatus == listingkit.SheinCandidateReviewStatusApproved || item.ReviewStatus == listingkit.SheinCandidateReviewStatusAutoQueued) {
			readyToEnrollCount++
		}
	}
	return pendingReviewCount, readyToEnrollCount
}

func compareSheinCandidateFreshness(left, right listingkit.SheinActivityCandidateRecord) int {
	switch {
	case left.UpdatedAt.After(right.UpdatedAt):
		return 1
	case left.UpdatedAt.Before(right.UpdatedAt):
		return -1
	case left.CreatedAt.After(right.CreatedAt):
		return 1
	case left.CreatedAt.Before(right.CreatedAt):
		return -1
	case left.ID > right.ID:
		return 1
	case left.ID < right.ID:
		return -1
	default:
		return 0
	}
}

func preferSheinTime(primary, fallback *time.Time) *time.Time {
	if primary != nil {
		return primary
	}
	return fallback
}
