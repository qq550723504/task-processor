package sheinsync

import "time"

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
