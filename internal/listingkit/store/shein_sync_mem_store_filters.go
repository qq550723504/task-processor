package store

import "task-processor/internal/listingkit"

func matchesSheinSyncedProductQuery(row listingkit.SheinSyncedProductRecord, query *listingkit.SheinSyncedProductQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.SKCName != "" && row.SKCName != query.SKCName {
		return false
	}
	if query.IsActive != nil && row.IsActive != *query.IsActive {
		return false
	}
	return true
}

func matchesSheinSyncJobQuery(row listingkit.SheinSyncJobRecord, query *listingkit.SheinSyncJobQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.TriggerMode != nil && row.TriggerMode != *query.TriggerMode {
		return false
	}
	if query.Status != nil && row.Status != *query.Status {
		return false
	}
	if query.StartedFrom != nil && (row.StartedAt == nil || row.StartedAt.Before(*query.StartedFrom)) {
		return false
	}
	if query.StartedTo != nil && (row.StartedAt == nil || row.StartedAt.After(*query.StartedTo)) {
		return false
	}
	return true
}

func matchesSheinActivityCandidateQuery(row listingkit.SheinActivityCandidateRecord, query *listingkit.SheinActivityCandidateQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.ActivityType != "" && row.ActivityType != query.ActivityType {
		return false
	}
	if query.ActivityKey != "" && row.ActivityKey != query.ActivityKey {
		return false
	}
	if query.SKCName != "" && row.SKCName != query.SKCName {
		return false
	}
	if query.CandidateVersion != "" && row.CandidateVersion != query.CandidateVersion {
		return false
	}
	if len(query.CandidateIDs) > 0 {
		found := false
		for _, id := range query.CandidateIDs {
			if row.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if query.ExecutableOnly && !isExecutableSheinActivityCandidate(row) {
		return false
	}
	return true
}

func isExecutableSheinActivityCandidate(row listingkit.SheinActivityCandidateRecord) bool {
	if row.EligibilityStatus != listingkit.SheinCandidateEligibilityStatusEligible {
		return false
	}
	switch row.ReviewStatus {
	case listingkit.SheinCandidateReviewStatusPendingReview,
		listingkit.SheinCandidateReviewStatusApproved,
		listingkit.SheinCandidateReviewStatusAutoQueued:
		return true
	default:
		return false
	}
}

func matchesSheinEnrollmentRunQuery(row listingkit.SheinActivityEnrollmentRunRecord, query *listingkit.SheinEnrollmentRunQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.ActivityType != "" && row.ActivityType != query.ActivityType {
		return false
	}
	if query.ActivityKey != "" && row.ActivityKey != query.ActivityKey {
		return false
	}
	if query.Status != nil && row.Status != *query.Status {
		return false
	}
	return true
}

func matchesSheinEnrollmentItemQuery(row listingkit.SheinActivityEnrollmentItemRecord, run listingkit.SheinActivityEnrollmentRunRecord, query *listingkit.SheinEnrollmentItemQuery) bool {
	if query == nil {
		return true
	}
	if query.RunID > 0 && row.RunID != query.RunID {
		return false
	}
	if query.Status != nil && row.Status != *query.Status {
		return false
	}
	if query.TenantID > 0 && run.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && run.StoreID != query.StoreID {
		return false
	}
	return true
}

func cloneSheinSyncedProductRecord(row listingkit.SheinSyncedProductRecord) listingkit.SheinSyncedProductRecord {
	row.PublishTime = cloneTimePtr(row.PublishTime)
	row.FirstShelfTime = cloneTimePtr(row.FirstShelfTime)
	row.LastSyncAt = cloneTimePtr(row.LastSyncAt)
	row.SupplyPrice = cloneFloat64Ptr(row.SupplyPrice)
	row.AutoCostPrice = cloneFloat64Ptr(row.AutoCostPrice)
	row.ManualCostPrice = cloneFloat64Ptr(row.ManualCostPrice)
	row.EffectiveCostPrice = cloneFloat64Ptr(row.EffectiveCostPrice)
	row.SKUCostPriceInfoList = append([]listingkit.SheinSKUCostPrice(nil), row.SKUCostPriceInfoList...)
	return row
}

func cloneSheinSyncJobRecord(row listingkit.SheinSyncJobRecord) listingkit.SheinSyncJobRecord {
	row.StartedAt = cloneTimePtr(row.StartedAt)
	row.FinishedAt = cloneTimePtr(row.FinishedAt)
	return row
}

func cloneSheinCandidateRecord(row listingkit.SheinActivityCandidateRecord) listingkit.SheinActivityCandidateRecord {
	row.EffectiveCostPrice = cloneFloat64Ptr(row.EffectiveCostPrice)
	row.CalculatedProfitRate = cloneFloat64Ptr(row.CalculatedProfitRate)
	row.SKUCostPriceInfoList = append([]listingkit.SheinSKUCostPrice(nil), row.SKUCostPriceInfoList...)
	return row
}

func cloneSheinEnrollmentRunRecord(row listingkit.SheinActivityEnrollmentRunRecord) listingkit.SheinActivityEnrollmentRunRecord {
	row.StartedAt = cloneTimePtr(row.StartedAt)
	row.FinishedAt = cloneTimePtr(row.FinishedAt)
	return row
}

func cloneSheinEnrollmentItemRecord(row listingkit.SheinActivityEnrollmentItemRecord) listingkit.SheinActivityEnrollmentItemRecord {
	return row
}
