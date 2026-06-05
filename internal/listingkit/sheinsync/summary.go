package sheinsync

import "time"

type SheinEnrollmentStoreSummary struct {
	StoreID            int64                             `json:"store_id"`
	StoreName          string                            `json:"store_name,omitempty"`
	StoreUsername      string                            `json:"store_username,omitempty"`
	Platform           string                            `json:"platform,omitempty"`
	Region             string                            `json:"region,omitempty"`
	EnableAutoListing  *bool                             `json:"enable_auto_listing,omitempty"`
	ActivityType       string                            `json:"activity_type,omitempty"`
	SyncedProductCount int                               `json:"synced_product_count"`
	MissingCostCount   int                               `json:"missing_cost_count"`
	PendingReviewCount int                               `json:"pending_review_count"`
	ReadyToEnrollCount int                               `json:"ready_to_enroll_count"`
	LastSyncAt         *time.Time                        `json:"last_sync_at,omitempty"`
	LastSyncStatus     SheinSyncJobStatus                `json:"last_sync_status,omitempty"`
	LastSyncJob        *SheinSyncJobRecord               `json:"last_sync_job,omitempty"`
	LastEnrollmentAt   *time.Time                        `json:"last_enrollment_at,omitempty"`
	LastEnrollmentRun  *SheinActivityEnrollmentRunRecord `json:"last_enrollment_run,omitempty"`
}
