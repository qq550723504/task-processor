package listingkit

import "time"

type studioBatchProductImageUsageRoute string

const (
	studioBatchProductImageUsageRoutePending studioBatchProductImageUsageRoute = "pending"
	studioBatchProductImageUsageRouteLegacy  studioBatchProductImageUsageRoute = "legacy"
	studioBatchProductImageUsageRouteLedger  studioBatchProductImageUsageRoute = "ledger"
)

type StudioBatchTaskLinkRecord struct {
	ID                                        string                            `json:"id" gorm:"primaryKey;type:varchar(96)"`
	TenantID                                  string                            `json:"-" gorm:"type:varchar(64);uniqueIndex:idx_listingkit_studio_batch_task_links_candidate,priority:1;uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:1;index"`
	UserID                                    string                            `json:"-" gorm:"type:varchar(128);index"`
	BatchID                                   string                            `json:"batch_id" gorm:"type:varchar(64);uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:2;index"`
	ItemID                                    string                            `json:"item_id" gorm:"type:varchar(96);uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:3"`
	DesignID                                  string                            `json:"design_id" gorm:"type:varchar(96);uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:4"`
	SelectionID                               string                            `json:"selection_id" gorm:"type:varchar(128);uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:5"`
	CompatibilityFingerprint                  string                            `json:"compatibility_fingerprint,omitempty" gorm:"type:varchar(128);uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:6"`
	ImageStrategy                             string                            `json:"image_strategy,omitempty" gorm:"type:varchar(32);uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:7"`
	SheinStoreID                              int64                             `json:"shein_store_id,omitempty" gorm:"uniqueIndex:idx_listingkit_studio_batch_task_links_tuple,priority:8;index"`
	ListingKitTaskID                          string                            `json:"listingkit_task_id,omitempty" gorm:"column:listingkit_task_id;type:varchar(96);index"`
	CandidateKey                              string                            `json:"candidate_key" gorm:"type:varchar(128);uniqueIndex:idx_listingkit_studio_batch_task_links_candidate,priority:2"`
	ClaimToken                                string                            `json:"-" gorm:"type:varchar(96);index"`
	Status                                    string                            `json:"status,omitempty" gorm:"type:varchar(32);index"`
	Source                                    string                            `json:"source,omitempty" gorm:"type:varchar(64);index"`
	ReasonCode                                string                            `json:"reason_code,omitempty" gorm:"type:varchar(96)"`
	Message                                   string                            `json:"message,omitempty" gorm:"type:text"`
	ProductImageUsageRoute                    studioBatchProductImageUsageRoute `json:"-" gorm:"type:varchar(16);index"`
	ProductImageUsageSettled                  bool                              `json:"-" gorm:"default:false;index"`
	PendingProductImageUsageReleaseClaimToken string                            `json:"-" gorm:"column:pending_product_image_usage_release_claim_token;type:varchar(96);index"`
	CreatedAt                                 time.Time                         `json:"created_at"`
	UpdatedAt                                 time.Time                         `json:"updated_at"`
}

func (StudioBatchTaskLinkRecord) TableName() string {
	return "listingkit_studio_batch_task_links"
}
