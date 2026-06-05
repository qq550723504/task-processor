package listingkit

import "time"

type StudioBatchStatus string

const (
	StudioBatchStatusDraft                 StudioBatchStatus = "draft"
	StudioBatchStatusGenerating            StudioBatchStatus = "generating"
	StudioBatchStatusPartiallyMaterialized StudioBatchStatus = "partially_materialized"
	StudioBatchStatusReviewReady           StudioBatchStatus = "review_ready"
	StudioBatchStatusPartiallyFailed       StudioBatchStatus = "partially_failed"
	StudioBatchStatusFailed                StudioBatchStatus = "failed"
	StudioBatchStatusTasksCreated          StudioBatchStatus = "tasks_created"
)

type StudioBatchItemStatus string

const (
	StudioBatchItemStatusPending                 StudioBatchItemStatus = "pending"
	StudioBatchItemStatusGenerating              StudioBatchItemStatus = "generating"
	StudioBatchItemStatusAwaitingMaterialization StudioBatchItemStatus = "awaiting_materialization"
	StudioBatchItemStatusReviewReady             StudioBatchItemStatus = "review_ready"
	StudioBatchItemStatusFailed                  StudioBatchItemStatus = "failed"
)

type StudioGenerationAttemptStatus string

const (
	StudioGenerationAttemptStatusQueued       StudioGenerationAttemptStatus = "queued"
	StudioGenerationAttemptStatusRunning      StudioGenerationAttemptStatus = "running"
	StudioGenerationAttemptStatusSucceeded    StudioGenerationAttemptStatus = "succeeded"
	StudioGenerationAttemptStatusMaterialized StudioGenerationAttemptStatus = "materialized"
	StudioGenerationAttemptStatusFailed       StudioGenerationAttemptStatus = "failed"
	StudioGenerationAttemptStatusCancelled    StudioGenerationAttemptStatus = "cancelled"
)

type StudioMaterializedDesignReviewStatus string

const (
	StudioMaterializedDesignReviewStatusUnreviewed StudioMaterializedDesignReviewStatus = "unreviewed"
	StudioMaterializedDesignReviewStatusApproved   StudioMaterializedDesignReviewStatus = "approved"
	StudioMaterializedDesignReviewStatusRejected   StudioMaterializedDesignReviewStatus = "rejected"
)

type StudioBatchRecord struct {
	ID                    string                          `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TenantID              string                          `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID                string                          `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	Status                StudioBatchStatus               `json:"status" gorm:"type:varchar(32);index;not null"`
	Prompt                string                          `json:"prompt,omitempty" gorm:"type:text"`
	GroupedImageMode      string                          `json:"grouped_image_mode,omitempty" gorm:"type:varchar(32)"`
	Selection             SheinStudioSelectionSnapshot    `json:"selection,omitempty" gorm:"type:text"`
	GroupedSelections     SheinStudioGroupedSelectionList `json:"grouped_selections,omitempty" gorm:"type:text"`
	StyleCount            string                          `json:"style_count,omitempty" gorm:"type:varchar(32)"`
	VariationIntensity    string                          `json:"variation_intensity,omitempty" gorm:"type:varchar(16)"`
	ArtworkModel          string                          `json:"artwork_model,omitempty" gorm:"type:varchar(32)"`
	SelectedSDSImages     SheinStudioSelectedSDSImageList `json:"selected_sds_images,omitempty" gorm:"type:text"`
	TransparentBackground bool                            `json:"transparent_background"`
	SheinStoreID          int64                           `json:"shein_store_id,omitempty" gorm:"index"`
	DraftUpdatedAt        *time.Time                      `json:"draft_updated_at,omitempty" gorm:"-"`
	CreatedAt             time.Time                       `json:"created_at"`
	UpdatedAt             time.Time                       `json:"updated_at"`
}

func (StudioBatchRecord) TableName() string {
	return "listingkit_studio_batches"
}

type StudioBatchItemRecord struct {
	ID               string                `json:"id" gorm:"primaryKey;type:varchar(96)"`
	BatchID          string                `json:"batch_id" gorm:"type:varchar(64);index:idx_listingkit_studio_batch_items_batch_group,priority:1"`
	TenantID         string                `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID           string                `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	TargetGroupKey   string                `json:"target_group_key,omitempty" gorm:"type:varchar(255);index:idx_listingkit_studio_batch_items_batch_group,priority:2"`
	TargetGroupLabel string                `json:"target_group_label,omitempty" gorm:"type:varchar(255)"`
	SelectionIDs     SheinStudioStringList `json:"selection_ids,omitempty" gorm:"type:text"`
	GroupMode        string                `json:"group_mode,omitempty" gorm:"type:varchar(32)"`
	Status           StudioBatchItemStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	SelectionCount   int                   `json:"selection_count" gorm:"not null;default:0"`
	LastError        string                `json:"last_error,omitempty" gorm:"type:text"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

func (StudioBatchItemRecord) TableName() string {
	return "listingkit_studio_batch_items"
}

type StudioGenerationAttemptRecord struct {
	ID             string                        `json:"id" gorm:"primaryKey;type:varchar(96)"`
	ItemID         string                        `json:"item_id" gorm:"type:varchar(96);index:idx_listingkit_studio_generation_attempts_item_attempt,priority:1"`
	BatchID        string                        `json:"batch_id,omitempty" gorm:"type:varchar(64);index"`
	TenantID       string                        `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID         string                        `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	AttemptNo      int                           `json:"attempt_no" gorm:"index:idx_listingkit_studio_generation_attempts_item_attempt,priority:2;not null"`
	Status         StudioGenerationAttemptStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	UpstreamJobID  string                        `json:"upstream_job_id,omitempty" gorm:"type:varchar(64);index"`
	RequestPayload string                        `json:"request_payload,omitempty" gorm:"type:text"`
	ResultPayload  string                        `json:"result_payload,omitempty" gorm:"type:text"`
	ErrorMessage   string                        `json:"error_message,omitempty" gorm:"type:text"`
	StartedAt      *time.Time                    `json:"started_at,omitempty"`
	FinishedAt     *time.Time                    `json:"finished_at,omitempty"`
	CreatedAt      time.Time                     `json:"created_at"`
	UpdatedAt      time.Time                     `json:"updated_at"`
}

func (StudioGenerationAttemptRecord) TableName() string {
	return "listingkit_studio_generation_attempts"
}

type StudioMaterializedDesignRecord struct {
	ID               string                               `json:"id" gorm:"primaryKey;type:varchar(96)"`
	BatchID          string                               `json:"batch_id" gorm:"type:varchar(64);index"`
	ItemID           string                               `json:"item_id" gorm:"type:varchar(96);index:idx_listingkit_studio_materialized_designs_item_sort,priority:1"`
	TenantID         string                               `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID           string                               `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	SourceAttemptID  string                               `json:"source_attempt_id,omitempty" gorm:"type:varchar(96);index"`
	TargetGroupKey   string                               `json:"target_group_key,omitempty" gorm:"type:varchar(255);index"`
	TargetGroupLabel string                               `json:"target_group_label,omitempty" gorm:"type:varchar(255)"`
	ImageURL         string                               `json:"image_url,omitempty" gorm:"type:text"`
	ReviewStatus     StudioMaterializedDesignReviewStatus `json:"review_status" gorm:"type:varchar(32);index;not null;default:'approved'"`
	SortOrder        int                                  `json:"sort_order" gorm:"index:idx_listingkit_studio_materialized_designs_item_sort,priority:2;not null;default:0"`
	ReviewNote       string                               `json:"review_note,omitempty" gorm:"type:text"`
	CreatedAt        time.Time                            `json:"created_at"`
	UpdatedAt        time.Time                            `json:"updated_at"`
}

func (StudioMaterializedDesignRecord) TableName() string {
	return "listingkit_studio_materialized_designs"
}

type StudioBatchDetailGraph struct {
	Batch          *StudioBatchRecord                          `json:"batch,omitempty"`
	Items          []StudioBatchItemRecord                     `json:"items,omitempty"`
	AttemptsByItem map[string][]StudioGenerationAttemptRecord  `json:"attempts_by_item,omitempty"`
	DesignsByItem  map[string][]StudioMaterializedDesignRecord `json:"designs_by_item,omitempty"`
}
