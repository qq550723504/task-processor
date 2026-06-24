package listingkit

import "time"

type SDSRetirementRunStatus string

const (
	SDSRetirementRunStatusDraft              SDSRetirementRunStatus = "draft"
	SDSRetirementRunStatusReady              SDSRetirementRunStatus = "ready"
	SDSRetirementRunStatusRunning            SDSRetirementRunStatus = "running"
	SDSRetirementRunStatusSucceeded          SDSRetirementRunStatus = "succeeded"
	SDSRetirementRunStatusPartiallySucceeded SDSRetirementRunStatus = "partially_succeeded"
	SDSRetirementRunStatusFailed             SDSRetirementRunStatus = "failed"
	SDSRetirementRunStatusCancelled          SDSRetirementRunStatus = "cancelled"
)

type SDSRetirementItemStatus string

const (
	SDSRetirementItemStatusPending                  SDSRetirementItemStatus = "pending"
	SDSRetirementItemStatusSelected                 SDSRetirementItemStatus = "selected"
	SDSRetirementItemStatusRunning                  SDSRetirementItemStatus = "running"
	SDSRetirementItemStatusSucceeded                SDSRetirementItemStatus = "succeeded"
	SDSRetirementItemStatusSucceededAlreadyOffShelf SDSRetirementItemStatus = "succeeded_already_off_shelf"
	SDSRetirementItemStatusFailed                   SDSRetirementItemStatus = "failed"
	SDSRetirementItemStatusSkipped                  SDSRetirementItemStatus = "skipped"
)

type SDSRetirementRunRecord struct {
	ID                 string                 `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID           string                 `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	Platform           string                 `json:"platform" gorm:"type:varchar(32);index;not null"`
	StoreID            int64                  `json:"store_id" gorm:"index"`
	ParentProductID    int64                  `json:"parent_product_id" gorm:"index"`
	PrototypeGroupID   int64                  `json:"prototype_group_id" gorm:"index"`
	VariantID          int64                  `json:"variant_id" gorm:"index"`
	SelectedVariantIDs string                 `json:"selected_variant_ids,omitempty" gorm:"type:text"`
	BaselineKey        string                 `json:"baseline_key,omitempty" gorm:"type:varchar(192);index"`
	ValidationStatus   string                 `json:"validation_status,omitempty" gorm:"type:varchar(32);index"`
	ReasonCode         string                 `json:"reason_code,omitempty" gorm:"type:varchar(64);index"`
	Reason             string                 `json:"reason,omitempty" gorm:"type:text"`
	Status             SDSRetirementRunStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	CreatedBy          string                 `json:"created_by,omitempty" gorm:"type:varchar(128)"`
	ConfirmedBy        string                 `json:"confirmed_by,omitempty" gorm:"type:varchar(128)"`
	ConfirmedAt        *time.Time             `json:"confirmed_at,omitempty"`
	StartedAt          *time.Time             `json:"started_at,omitempty"`
	FinishedAt         *time.Time             `json:"finished_at,omitempty"`
	CreatedAt          time.Time              `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time              `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SDSRetirementRunRecord) TableName() string {
	return "listingkit_sds_retirement_runs"
}

type SDSRetirementItemRecord struct {
	ID                string                  `json:"id" gorm:"primaryKey;type:varchar(36)"`
	RunID             string                  `json:"run_id" gorm:"type:varchar(36);index;not null"`
	TenantID          string                  `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	Platform          string                  `json:"platform" gorm:"type:varchar(32);index;not null"`
	StoreID           int64                   `json:"store_id" gorm:"index"`
	TaskID            string                  `json:"task_id,omitempty" gorm:"type:varchar(36);index"`
	SyncedProductID   int64                   `json:"synced_product_id,omitempty" gorm:"index"`
	SPUName           string                  `json:"spu_name,omitempty" gorm:"type:varchar(255);index"`
	SKCName           string                  `json:"skc_name,omitempty" gorm:"type:varchar(128);index"`
	SKCCode           string                  `json:"skc_code,omitempty" gorm:"type:varchar(128)"`
	SupplierCode      string                  `json:"supplier_code,omitempty" gorm:"type:varchar(128);index"`
	BusinessModel     int                     `json:"business_model,omitempty"`
	ShelfStatusBefore string                  `json:"shelf_status_before,omitempty" gorm:"type:varchar(64)"`
	Selected          bool                    `json:"selected" gorm:"index;not null;default:false"`
	SiteSelection     string                  `json:"site_selection,omitempty" gorm:"type:text"`
	RequestSnapshot   string                  `json:"request_snapshot,omitempty" gorm:"type:text"`
	ResponseSnapshot  string                  `json:"response_snapshot,omitempty" gorm:"type:text"`
	Status            SDSRetirementItemStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	Error             string                  `json:"error,omitempty" gorm:"type:text"`
	StartedAt         *time.Time              `json:"started_at,omitempty"`
	FinishedAt        *time.Time              `json:"finished_at,omitempty"`
	CreatedAt         time.Time               `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time               `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SDSRetirementItemRecord) TableName() string {
	return "listingkit_sds_retirement_items"
}

type SDSRetirementItemSelectionUpdate struct {
	ItemID        string `json:"item_id"`
	Selected      bool   `json:"selected"`
	SiteSelection string `json:"site_selection,omitempty"`
}
