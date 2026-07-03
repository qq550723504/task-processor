package listingkit

import "time"

type SheinStudioSessionStatus string

const (
	SheinStudioSessionStatusSelecting     SheinStudioSessionStatus = "selecting"
	SheinStudioSessionStatusGenerating    SheinStudioSessionStatus = "generating"
	SheinStudioSessionStatusGenerated     SheinStudioSessionStatus = "generated"
	SheinStudioSessionStatusReviewing     SheinStudioSessionStatus = "reviewing"
	SheinStudioSessionStatusFailed        SheinStudioSessionStatus = "failed"
	SheinStudioSessionStatusTasksCreating SheinStudioSessionStatus = "tasks_creating"
	SheinStudioSessionStatusTasksCreated  SheinStudioSessionStatus = "tasks_created"
)

type SheinStudioCreatedTask struct {
	ID                       string `json:"id,omitempty"`
	Title                    string `json:"title,omitempty"`
	DesignID                 string `json:"design_id,omitempty"`
	ItemID                   string `json:"item_id,omitempty"`
	SelectionID              string `json:"selection_id,omitempty"`
	CompatibilityFingerprint string `json:"compatibility_fingerprint,omitempty"`
	Status                   string `json:"status,omitempty"`
	SubmissionState          string `json:"submission_state,omitempty"`
	LastSubmissionAction     string `json:"last_submission_action,omitempty"`
	Source                   string `json:"source,omitempty"`
	ReasonCode               string `json:"reason_code,omitempty"`
	Message                  string `json:"message,omitempty"`
}

type SheinStudioGenerationJob struct {
	JobID            string               `json:"job_id,omitempty"`
	TargetGroupKey   string               `json:"target_group_key,omitempty"`
	TargetGroupLabel string               `json:"target_group_label,omitempty"`
	Status           StudioAsyncJobStatus `json:"status,omitempty"`
}

type SheinStudioSession struct {
	ID                         string                            `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TenantID                   string                            `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID                     string                            `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	SelectionKey               string                            `json:"selection_key" gorm:"type:varchar(255);index"`
	Status                     SheinStudioSessionStatus          `json:"status" gorm:"type:varchar(32);index"`
	ProductID                  int64                             `json:"product_id,omitempty" gorm:"index"`
	ParentProductID            int64                             `json:"parent_product_id,omitempty"`
	VariantID                  int64                             `json:"variant_id,omitempty" gorm:"index"`
	PrototypeGroupID           int64                             `json:"prototype_group_id,omitempty"`
	LayerID                    string                            `json:"layer_id,omitempty" gorm:"type:varchar(128)"`
	PrintableWidth             int                               `json:"printable_width,omitempty"`
	PrintableHeight            int                               `json:"printable_height,omitempty"`
	SelectedVariantIDs         SheinStudioInt64List              `json:"selected_variant_ids,omitempty" gorm:"type:text"`
	Selection                  SheinStudioSelectionSnapshot      `json:"selection,omitempty" gorm:"type:text"`
	Prompt                     string                            `json:"prompt,omitempty" gorm:"type:text"`
	PromptMode                 string                            `json:"prompt_mode,omitempty" gorm:"type:varchar(16)"`
	StyleCount                 string                            `json:"style_count,omitempty" gorm:"type:varchar(32)"`
	VariationIntensity         string                            `json:"variation_intensity,omitempty" gorm:"type:varchar(16)"`
	ProductImageCount          string                            `json:"product_image_count,omitempty" gorm:"type:varchar(32)"`
	ProductImagePrompt         string                            `json:"product_image_prompt,omitempty" gorm:"type:text"`
	ProductImagePrompts        SheinStudioProductImagePromptList `json:"product_image_prompts,omitempty" gorm:"type:text"`
	ArtworkModel               string                            `json:"artwork_model,omitempty" gorm:"type:varchar(32)"`
	ImageStrategy              string                            `json:"image_strategy,omitempty" gorm:"type:varchar(32)"`
	GroupedImageMode           string                            `json:"grouped_image_mode,omitempty" gorm:"type:varchar(32)"`
	SelectedSDSImages          SheinStudioSelectedSDSImageList   `json:"selected_sds_images,omitempty" gorm:"type:text"`
	GroupedSelections          SheinStudioGroupedSelectionList   `json:"grouped_selections,omitempty" gorm:"type:text"`
	TransparentBackground      bool                              `json:"transparent_background"`
	RenderSizeImagesWithSDS    bool                              `json:"render_size_images_with_sds"`
	HotStyleReferenceImageURLs SheinStudioStringList             `json:"hot_style_reference_image_urls" gorm:"type:text"`
	HotStyleReferenceBrief     string                            `json:"hot_style_reference_brief" gorm:"type:text"`
	HotStyleReferencePrompt    string                            `json:"hot_style_reference_prompt" gorm:"type:text"`
	SheinStoreID               string                            `json:"shein_store_id,omitempty" gorm:"type:varchar(64)"`
	GenerationJobID            string                            `json:"generation_job_id,omitempty" gorm:"type:varchar(64);index"`
	GenerationJobs             SheinStudioGenerationJobList      `json:"generation_jobs,omitempty" gorm:"type:text"`
	GenerationError            string                            `json:"generation_error,omitempty" gorm:"type:text"`
	ApprovedDesignIDs          SheinStudioStringList             `json:"approved_design_ids,omitempty" gorm:"type:text"`
	PendingTaskDesignIDs       SheinStudioStringList             `json:"pending_task_design_ids,omitempty" gorm:"type:text"`
	CreatedTaskIDs             SheinStudioStringList             `json:"created_task_ids,omitempty" gorm:"type:text"`
	CreatedTasks               SheinStudioCreatedTaskList        `json:"created_tasks,omitempty" gorm:"type:text"`
	FailedTasks                SheinStudioFailedTaskList         `json:"failed_tasks,omitempty" gorm:"type:text"`
	SavedAsBatch               bool                              `json:"saved_as_batch,omitempty" gorm:"index"`
	BatchName                  string                            `json:"batch_name,omitempty" gorm:"type:varchar(255)"`
	CreatedAt                  time.Time                         `json:"created_at"`
	UpdatedAt                  time.Time                         `json:"updated_at"`
}

type SheinStudioDesign struct {
	ID                    string                `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TenantID              string                `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	SessionID             string                `json:"session_id" gorm:"type:varchar(64);index:idx_shein_studio_design_session_sort,priority:1"`
	ImageURL              string                `json:"image_url" gorm:"type:text"`
	ProductImageURLs      SheinStudioStringList `json:"product_image_urls,omitempty" gorm:"type:text"`
	Prompt                string                `json:"prompt,omitempty" gorm:"type:text"`
	RevisedPrompt         string                `json:"revised_prompt,omitempty" gorm:"type:text"`
	ImageModel            string                `json:"image_model,omitempty" gorm:"type:varchar(64)"`
	TransparentBackground bool                  `json:"transparent_background,omitempty"`
	VariationIntensity    string                `json:"variation_intensity,omitempty" gorm:"type:varchar(16)"`
	Role                  string                `json:"role,omitempty" gorm:"type:varchar(64)"`
	RoleLabel             string                `json:"role_label,omitempty" gorm:"type:varchar(128)"`
	ReviewNote            string                `json:"review_note,omitempty" gorm:"type:text"`
	SortOrder             int                   `json:"sort_order" gorm:"index:idx_shein_studio_design_session_sort,priority:2"`
	Approved              bool                  `json:"approved"`
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
}

type SheinStudioSessionDetail struct {
	Session *SheinStudioSession `json:"session,omitempty"`
	Designs []SheinStudioDesign `json:"designs,omitempty"`
}
