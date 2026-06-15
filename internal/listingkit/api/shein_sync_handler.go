package api

import "task-processor/internal/listingkit"

type triggerSheinStoreSyncRequest struct {
	TriggerMode listingkit.SheinSyncTriggerMode `json:"trigger_mode"`
}

type listSheinSyncedProductsQuery struct {
	SKCName  string `form:"skc_name"`
	IsActive string `form:"is_active"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type updateSheinSyncedProductCostRequest struct {
	ManualCostPrice *float64 `json:"manual_cost_price"`
}

type refreshSheinActivityCandidatesRequest struct {
	ActivityType string `json:"activity_type"`
}

type listSheinActivityCandidatesQuery struct {
	ActivityType     string `form:"activity_type"`
	ActivityKey      string `form:"activity_key"`
	SKCName          string `form:"skc_name"`
	CandidateVersion string `form:"candidate_version"`
	Page             int    `form:"page"`
	PageSize         int    `form:"page_size"`
}

type reviewSheinActivityCandidateRequest struct {
	StoreID          int64                                 `json:"store_id"`
	ReviewStatus     listingkit.SheinCandidateReviewStatus `json:"review_status"`
	AutoModeEligible *bool                                 `json:"auto_mode_eligible"`
	SelectedForRun   *bool                                 `json:"selected_for_run"`
}

type executeSheinActivityEnrollmentRequest struct {
	ActivityType string                                   `json:"activity_type"`
	ActivityKey  string                                   `json:"activity_key"`
	TriggerMode  listingkit.SheinEnrollmentRunTriggerMode `json:"trigger_mode"`
	CandidateIDs []int64                                  `json:"candidate_ids"`
}
