package sheinsync

import "time"

type SheinCostPriceSource string

const (
	SheinCostPriceSourceNone   SheinCostPriceSource = "none"
	SheinCostPriceSourceAuto   SheinCostPriceSource = "auto"
	SheinCostPriceSourceManual SheinCostPriceSource = "manual"
)

type SheinSyncTriggerMode string

const (
	SheinSyncTriggerModeManual   SheinSyncTriggerMode = "manual"
	SheinSyncTriggerModeSchedule SheinSyncTriggerMode = "schedule"
)

type SheinSyncJobStatus string

const (
	SheinSyncJobStatusPending            SheinSyncJobStatus = "pending"
	SheinSyncJobStatusRunning            SheinSyncJobStatus = "running"
	SheinSyncJobStatusSucceeded          SheinSyncJobStatus = "succeeded"
	SheinSyncJobStatusPartiallySucceeded SheinSyncJobStatus = "partially_succeeded"
	SheinSyncJobStatusFailed             SheinSyncJobStatus = "failed"
)

type SheinCandidateEligibilityStatus string

const (
	SheinCandidateEligibilityStatusEligible   SheinCandidateEligibilityStatus = "eligible"
	SheinCandidateEligibilityStatusIneligible SheinCandidateEligibilityStatus = "ineligible"
)

type SheinCandidateReviewStatus string

const (
	SheinCandidateReviewStatusPendingReview SheinCandidateReviewStatus = "pending_review"
	SheinCandidateReviewStatusApproved      SheinCandidateReviewStatus = "approved"
	SheinCandidateReviewStatusRejected      SheinCandidateReviewStatus = "rejected"
	SheinCandidateReviewStatusAutoQueued    SheinCandidateReviewStatus = "auto_queued"
	SheinCandidateReviewStatusEnrolled      SheinCandidateReviewStatus = "enrolled"
	SheinCandidateReviewStatusFailed        SheinCandidateReviewStatus = "failed"
)

type SheinEnrollmentRunTriggerMode string

const (
	SheinEnrollmentRunTriggerModeManualConfirmed SheinEnrollmentRunTriggerMode = "manual_confirmed"
	SheinEnrollmentRunTriggerModeAutoSchedule    SheinEnrollmentRunTriggerMode = "auto_schedule"
)

type SheinEnrollmentRunStatus string

const (
	SheinEnrollmentRunStatusPending            SheinEnrollmentRunStatus = "pending"
	SheinEnrollmentRunStatusRunning            SheinEnrollmentRunStatus = "running"
	SheinEnrollmentRunStatusSucceeded          SheinEnrollmentRunStatus = "succeeded"
	SheinEnrollmentRunStatusPartiallySucceeded SheinEnrollmentRunStatus = "partially_succeeded"
	SheinEnrollmentRunStatusFailed             SheinEnrollmentRunStatus = "failed"
	SheinEnrollmentRunStatusCancelled          SheinEnrollmentRunStatus = "cancelled"
)

type SheinEnrollmentItemStatus string

const (
	SheinEnrollmentItemStatusPending   SheinEnrollmentItemStatus = "pending"
	SheinEnrollmentItemStatusRunning   SheinEnrollmentItemStatus = "running"
	SheinEnrollmentItemStatusSucceeded SheinEnrollmentItemStatus = "succeeded"
	SheinEnrollmentItemStatusFailed    SheinEnrollmentItemStatus = "failed"
	SheinEnrollmentItemStatusCancelled SheinEnrollmentItemStatus = "cancelled"
)

type SheinSyncedProductRecord struct {
	ID                 int64                `json:"id" gorm:"primaryKey"`
	TenantID           int64                `json:"tenant_id" gorm:"index:idx_listingkit_shein_synced_products_scope,priority:1;uniqueIndex:uk_listingkit_shein_synced_products_store_skc,priority:1"`
	StoreID            int64                `json:"store_id" gorm:"index:idx_listingkit_shein_synced_products_scope,priority:2;uniqueIndex:uk_listingkit_shein_synced_products_store_skc,priority:2"`
	SPUName            string               `json:"spu_name,omitempty" gorm:"type:varchar(255)"`
	SPUCode            string               `json:"spu_code,omitempty" gorm:"type:varchar(128);index"`
	SKCName            string               `json:"skc_name,omitempty" gorm:"type:varchar(128);uniqueIndex:uk_listingkit_shein_synced_products_store_skc,priority:3"`
	SKCCode            string               `json:"skc_code,omitempty" gorm:"type:varchar(128);index"`
	SupplierCode       string               `json:"supplier_code,omitempty" gorm:"type:varchar(128);index"`
	CategoryID         int64                `json:"category_id,omitempty" gorm:"index"`
	BrandName          string               `json:"brand_name,omitempty" gorm:"type:varchar(255)"`
	ProductNameMulti   string               `json:"product_name_multi,omitempty" gorm:"type:text"`
	MainImageURL       string               `json:"main_image_url,omitempty" gorm:"type:text"`
	SaleName           string               `json:"sale_name,omitempty" gorm:"type:varchar(255)"`
	ShelfStatus        string               `json:"shelf_status,omitempty" gorm:"type:varchar(64);index"`
	PublishTime        *time.Time           `json:"publish_time,omitempty"`
	FirstShelfTime     *time.Time           `json:"first_shelf_time,omitempty"`
	Currency           string               `json:"currency,omitempty" gorm:"type:varchar(16)"`
	PriceSnapshot      string               `json:"price_snapshot,omitempty" gorm:"type:text"`
	InventorySnapshot  string               `json:"inventory_snapshot,omitempty" gorm:"type:text"`
	SiteSnapshot       string               `json:"site_snapshot,omitempty" gorm:"type:text"`
	AutoCostPrice      *float64             `json:"auto_cost_price,omitempty"`
	ManualCostPrice    *float64             `json:"manual_cost_price,omitempty"`
	EffectiveCostPrice *float64             `json:"effective_cost_price,omitempty"`
	CostPriceSource    SheinCostPriceSource `json:"cost_price_source" gorm:"type:varchar(32);not null;default:'none'"`
	SyncVersion        string               `json:"sync_version,omitempty" gorm:"type:varchar(64);index"`
	LastSyncAt         *time.Time           `json:"last_sync_at,omitempty"`
	IsActive           bool                 `json:"is_active" gorm:"index;not null;default:true"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

func (SheinSyncedProductRecord) TableName() string {
	return "listingkit_shein_synced_products"
}

type SheinSyncJobRecord struct {
	ID               int64                `json:"id" gorm:"primaryKey"`
	TenantID         int64                `json:"tenant_id" gorm:"index:idx_listingkit_shein_sync_jobs_scope,priority:1"`
	StoreID          int64                `json:"store_id" gorm:"index:idx_listingkit_shein_sync_jobs_scope,priority:2"`
	TriggerMode      SheinSyncTriggerMode `json:"trigger_mode" gorm:"type:varchar(32);index;not null"`
	Status           SheinSyncJobStatus   `json:"status" gorm:"type:varchar(32);index;not null"`
	StartedAt        *time.Time           `json:"started_at,omitempty"`
	FinishedAt       *time.Time           `json:"finished_at,omitempty"`
	FetchedCount     int                  `json:"fetched_count"`
	InsertedCount    int                  `json:"inserted_count"`
	UpdatedCount     int                  `json:"updated_count"`
	DeactivatedCount int                  `json:"deactivated_count"`
	SkippedCount     int                  `json:"skipped_count"`
	ErrorSummary     string               `json:"error_summary,omitempty" gorm:"type:text"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

func (SheinSyncJobRecord) TableName() string {
	return "listingkit_shein_sync_jobs"
}

type SheinActivityCandidateRecord struct {
	ID                   int64                           `json:"id" gorm:"primaryKey"`
	TenantID             int64                           `json:"tenant_id" gorm:"index:idx_listingkit_shein_activity_candidates_scope,priority:1;uniqueIndex:uk_listingkit_shein_activity_candidates_activity_skc_version,priority:1"`
	StoreID              int64                           `json:"store_id" gorm:"index:idx_listingkit_shein_activity_candidates_scope,priority:2;uniqueIndex:uk_listingkit_shein_activity_candidates_activity_skc_version,priority:2"`
	SyncedProductID      int64                           `json:"synced_product_id" gorm:"index"`
	ActivityType         string                          `json:"activity_type" gorm:"type:varchar(64);index;uniqueIndex:uk_listingkit_shein_activity_candidates_activity_skc_version,priority:3"`
	ActivityKey          string                          `json:"activity_key" gorm:"type:varchar(128);index;uniqueIndex:uk_listingkit_shein_activity_candidates_activity_skc_version,priority:4"`
	SKCName              string                          `json:"skc_name,omitempty" gorm:"type:varchar(128);index;uniqueIndex:uk_listingkit_shein_activity_candidates_activity_skc_version,priority:5"`
	CandidateVersion     string                          `json:"candidate_version" gorm:"type:varchar(64);uniqueIndex:uk_listingkit_shein_activity_candidates_activity_skc_version,priority:6"`
	EffectiveCostPrice   *float64                        `json:"effective_cost_price,omitempty"`
	PriceSnapshot        string                          `json:"price_snapshot,omitempty" gorm:"type:text"`
	InventorySnapshot    string                          `json:"inventory_snapshot,omitempty" gorm:"type:text"`
	CalculatedProfitRate *float64                        `json:"calculated_profit_rate,omitempty"`
	EligibilityStatus    SheinCandidateEligibilityStatus `json:"eligibility_status" gorm:"type:varchar(32);index;not null"`
	EligibilityReason    string                          `json:"eligibility_reason,omitempty" gorm:"type:text"`
	ReviewStatus         SheinCandidateReviewStatus      `json:"review_status" gorm:"type:varchar(32);index;not null"`
	AutoModeEligible     bool                            `json:"auto_mode_eligible" gorm:"index;not null;default:false"`
	SelectedForRun       bool                            `json:"selected_for_run" gorm:"index;not null;default:false"`
	CreatedAt            time.Time                       `json:"created_at"`
	UpdatedAt            time.Time                       `json:"updated_at"`
}

func (SheinActivityCandidateRecord) TableName() string {
	return "listingkit_shein_activity_candidates"
}

type SheinActivityEnrollmentRunRecord struct {
	ID             int64                         `json:"id" gorm:"primaryKey"`
	TenantID       int64                         `json:"tenant_id" gorm:"index:idx_listingkit_shein_enrollment_runs_scope,priority:1"`
	StoreID        int64                         `json:"store_id" gorm:"index:idx_listingkit_shein_enrollment_runs_scope,priority:2"`
	ActivityType   string                        `json:"activity_type" gorm:"type:varchar(64);index"`
	ActivityKey    string                        `json:"activity_key" gorm:"type:varchar(128);index"`
	TriggerMode    SheinEnrollmentRunTriggerMode `json:"trigger_mode" gorm:"type:varchar(32);index;not null"`
	Status         SheinEnrollmentRunStatus      `json:"status" gorm:"type:varchar(32);index;not null"`
	CandidateCount int                           `json:"candidate_count"`
	SubmittedCount int                           `json:"submitted_count"`
	SucceededCount int                           `json:"succeeded_count"`
	FailedCount    int                           `json:"failed_count"`
	StartedAt      *time.Time                    `json:"started_at,omitempty"`
	FinishedAt     *time.Time                    `json:"finished_at,omitempty"`
	ErrorSummary   string                        `json:"error_summary,omitempty" gorm:"type:text"`
	CreatedAt      time.Time                     `json:"created_at"`
	UpdatedAt      time.Time                     `json:"updated_at"`
}

func (SheinActivityEnrollmentRunRecord) TableName() string {
	return "listingkit_shein_activity_enrollment_runs"
}

type SheinActivityEnrollmentItemRecord struct {
	ID               int64                     `json:"id" gorm:"primaryKey"`
	RunID            int64                     `json:"run_id" gorm:"index:idx_listingkit_shein_enrollment_items_run_candidate,priority:1;uniqueIndex:uk_listingkit_shein_enrollment_items_run_candidate,priority:1"`
	CandidateID      int64                     `json:"candidate_id" gorm:"index:idx_listingkit_shein_enrollment_items_run_candidate,priority:2;uniqueIndex:uk_listingkit_shein_enrollment_items_run_candidate,priority:2"`
	StoreID          int64                     `json:"store_id" gorm:"index"`
	ActivityKey      string                    `json:"activity_key,omitempty" gorm:"type:varchar(128);index"`
	CandidateVersion string                    `json:"candidate_version,omitempty" gorm:"type:varchar(64);index"`
	SyncedProductID  int64                     `json:"synced_product_id" gorm:"index"`
	SKCName          string                    `json:"skc_name,omitempty" gorm:"type:varchar(128);index"`
	Status           SheinEnrollmentItemStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	RequestPayload   string                    `json:"request_payload,omitempty" gorm:"type:text"`
	ResponsePayload  string                    `json:"response_payload,omitempty" gorm:"type:text"`
	ErrorMessage     string                    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

func (SheinActivityEnrollmentItemRecord) TableName() string {
	return "listingkit_shein_activity_enrollment_items"
}

func ApplyEffectiveCostPrice(record *SheinSyncedProductRecord) {
	if record == nil {
		return
	}

	switch {
	case record.ManualCostPrice != nil:
		record.EffectiveCostPrice = sheinFloat64Ptr(*record.ManualCostPrice)
		record.CostPriceSource = SheinCostPriceSourceManual
	case record.AutoCostPrice != nil:
		record.EffectiveCostPrice = sheinFloat64Ptr(*record.AutoCostPrice)
		record.CostPriceSource = SheinCostPriceSourceAuto
	default:
		record.EffectiveCostPrice = nil
		record.CostPriceSource = SheinCostPriceSourceNone
	}
}

func sheinFloat64Ptr(v float64) *float64 {
	return &v
}
