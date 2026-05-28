package listingkit

import (
	"database/sql/driver"
	"time"
)

type SheinStudioSessionStatus string

const (
	SheinStudioSessionStatusSelecting    SheinStudioSessionStatus = "selecting"
	SheinStudioSessionStatusGenerating   SheinStudioSessionStatus = "generating"
	SheinStudioSessionStatusGenerated    SheinStudioSessionStatus = "generated"
	SheinStudioSessionStatusReviewing    SheinStudioSessionStatus = "reviewing"
	SheinStudioSessionStatusFailed       SheinStudioSessionStatus = "failed"
	SheinStudioSessionStatusTasksCreated SheinStudioSessionStatus = "tasks_created"
)

type SheinStudioSelectionVariant struct {
	VariantID              int64    `json:"variant_id,omitempty"`
	VariantSKU             string   `json:"variant_sku,omitempty"`
	Size                   string   `json:"size,omitempty"`
	Color                  string   `json:"color,omitempty"`
	Price                  float64  `json:"price,omitempty"`
	Weight                 float64  `json:"weight,omitempty"`
	BoxLength              float64  `json:"box_length,omitempty"`
	BoxWidth               float64  `json:"box_width,omitempty"`
	BoxHeight              float64  `json:"box_height,omitempty"`
	ProductionCycle        int      `json:"production_cycle,omitempty"`
	PrototypeGroupID       int64    `json:"prototype_group_id,omitempty"`
	LayerID                string   `json:"layer_id,omitempty"`
	TemplateImageURL       string   `json:"template_image_url,omitempty"`
	MaskImageURL           string   `json:"mask_image_url,omitempty"`
	BlankDesignURL         string   `json:"blank_design_url,omitempty"`
	MockupImageURL         string   `json:"mockup_image_url,omitempty"`
	MockupImageURLs        []string `json:"mockup_image_urls,omitempty"`
	SizeReferenceImageURLs []string `json:"size_reference_image_urls,omitempty"`
}

type SheinStudioSelection struct {
	ProductID              int64                         `json:"product_id,omitempty"`
	ParentProductID        int64                         `json:"parent_product_id,omitempty"`
	VariantID              int64                         `json:"variant_id,omitempty"`
	PrototypeGroupID       int64                         `json:"prototype_group_id,omitempty"`
	LayerID                string                        `json:"layer_id,omitempty"`
	ProductName            string                        `json:"product_name,omitempty"`
	VariantLabel           string                        `json:"variant_label,omitempty"`
	PrintableWidth         int                           `json:"printable_width,omitempty"`
	PrintableHeight        int                           `json:"printable_height,omitempty"`
	TemplateImageURL       string                        `json:"template_image_url,omitempty"`
	MaskImageURL           string                        `json:"mask_image_url,omitempty"`
	BlankDesignURL         string                        `json:"blank_design_url,omitempty"`
	MockupImageURL         string                        `json:"mockup_image_url,omitempty"`
	MockupImageURLs        []string                      `json:"mockup_image_urls,omitempty"`
	SizeReferenceImageURLs []string                      `json:"size_reference_image_urls,omitempty"`
	SelectedVariantIDs     []int64                       `json:"selected_variant_ids,omitempty"`
	Variants               []SheinStudioSelectionVariant `json:"variants,omitempty"`
}

type SheinStudioProductImagePrompt struct {
	Role   string `json:"role,omitempty"`
	Label  string `json:"label,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type SheinStudioSelectedSDSImageRecord struct {
	ImageURL   string `json:"image_url,omitempty"`
	VariantSKU string `json:"variant_sku,omitempty"`
	Color      string `json:"color,omitempty"`
}

type SheinStudioCreatedTask struct {
	ID       string `json:"id,omitempty"`
	Title    string `json:"title,omitempty"`
	DesignID string `json:"design_id,omitempty"`
}

type SheinStudioGroupedSelection struct {
	SelectionID        string               `json:"selection_id,omitempty"`
	Selection          SheinStudioSelection `json:"selection,omitempty"`
	BaselineKey        string               `json:"baseline_key,omitempty"`
	BaselineStatus     string               `json:"baseline_status,omitempty"`
	BaselineReason     string               `json:"baseline_reason,omitempty"`
	BaselineReasonCode string               `json:"baseline_reason_code,omitempty"`
	SheinStoreID       string               `json:"shein_store_id,omitempty"`
	Eligible           bool                 `json:"eligible"`
	EligibilityReason  string               `json:"eligibility_reason,omitempty"`
}

type SheinStudioSelectionVariants []SheinStudioSelectionVariant

func (value SheinStudioSelectionVariants) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioSelectionVariants) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioSelectionSnapshot SheinStudioSelection

func (value SheinStudioSelectionSnapshot) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioSelectionSnapshot) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioInt64List []int64

func (value SheinStudioInt64List) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioInt64List) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioStringList []string

func (value SheinStudioStringList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioStringList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioProductImagePromptList []SheinStudioProductImagePrompt

func (value SheinStudioProductImagePromptList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioProductImagePromptList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioSelectedSDSImageList []SheinStudioSelectedSDSImageRecord

func (value SheinStudioSelectedSDSImageList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioSelectedSDSImageList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioCreatedTaskList []SheinStudioCreatedTask

func (value SheinStudioCreatedTaskList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioCreatedTaskList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioGroupedSelectionList []SheinStudioGroupedSelection

func (value SheinStudioGroupedSelectionList) Value() (driver.Value, error) {
	return marshalStudioSessionJSON(value)
}

func (value *SheinStudioGroupedSelectionList) Scan(input any) error {
	return unmarshalStudioSessionJSON(input, value)
}

type SheinStudioSession struct {
	ID                      string                            `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TenantID                string                            `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID                  string                            `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	SelectionKey            string                            `json:"selection_key" gorm:"type:varchar(255);index"`
	Status                  SheinStudioSessionStatus          `json:"status" gorm:"type:varchar(32);index"`
	ProductID               int64                             `json:"product_id,omitempty" gorm:"index"`
	ParentProductID         int64                             `json:"parent_product_id,omitempty"`
	VariantID               int64                             `json:"variant_id,omitempty" gorm:"index"`
	PrototypeGroupID        int64                             `json:"prototype_group_id,omitempty"`
	LayerID                 string                            `json:"layer_id,omitempty" gorm:"type:varchar(128)"`
	PrintableWidth          int                               `json:"printable_width,omitempty"`
	PrintableHeight         int                               `json:"printable_height,omitempty"`
	SelectedVariantIDs      SheinStudioInt64List              `json:"selected_variant_ids,omitempty" gorm:"type:text"`
	Selection               SheinStudioSelectionSnapshot      `json:"selection,omitempty" gorm:"type:text"`
	Prompt                  string                            `json:"prompt,omitempty" gorm:"type:text"`
	StyleCount              string                            `json:"style_count,omitempty" gorm:"type:varchar(32)"`
	VariationIntensity      string                            `json:"variation_intensity,omitempty" gorm:"type:varchar(16)"`
	ProductImageCount       string                            `json:"product_image_count,omitempty" gorm:"type:varchar(32)"`
	ProductImagePrompt      string                            `json:"product_image_prompt,omitempty" gorm:"type:text"`
	ProductImagePrompts     SheinStudioProductImagePromptList `json:"product_image_prompts,omitempty" gorm:"type:text"`
	ArtworkModel            string                            `json:"artwork_model,omitempty" gorm:"type:varchar(32)"`
	ImageStrategy           string                            `json:"image_strategy,omitempty" gorm:"type:varchar(32)"`
	SelectedSDSImages       SheinStudioSelectedSDSImageList   `json:"selected_sds_images,omitempty" gorm:"type:text"`
	GroupedSelections       SheinStudioGroupedSelectionList   `json:"grouped_selections,omitempty" gorm:"type:text"`
	TransparentBackground   bool                              `json:"transparent_background"`
	RenderSizeImagesWithSDS bool                              `json:"render_size_images_with_sds"`
	SheinStoreID            string                            `json:"shein_store_id,omitempty" gorm:"type:varchar(64)"`
	GenerationJobID         string                            `json:"generation_job_id,omitempty" gorm:"type:varchar(64);index"`
	GenerationError         string                            `json:"generation_error,omitempty" gorm:"type:text"`
	ApprovedDesignIDs       SheinStudioStringList             `json:"approved_design_ids,omitempty" gorm:"type:text"`
	CreatedTaskIDs          SheinStudioStringList             `json:"created_task_ids,omitempty" gorm:"type:text"`
	CreatedTasks            SheinStudioCreatedTaskList        `json:"created_tasks,omitempty" gorm:"type:text"`
	SavedAsBatch            bool                              `json:"saved_as_batch,omitempty" gorm:"index"`
	BatchName               string                            `json:"batch_name,omitempty" gorm:"type:varchar(255)"`
	CreatedAt               time.Time                         `json:"created_at"`
	UpdatedAt               time.Time                         `json:"updated_at"`
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

type SheinStudioSessionGalleryItem struct {
	TenantID              string `json:"tenant_id,omitempty"`
	SessionID             string `json:"session_id"`
	DesignID              string `json:"design_id"`
	ImageURL              string `json:"image_url"`
	Prompt                string `json:"prompt,omitempty"`
	ProductName           string `json:"product_name,omitempty"`
	Status                string `json:"status,omitempty"`
	CreatedAt             string `json:"created_at,omitempty"`
	UpdatedAt             string `json:"updated_at,omitempty"`
	ReviewNote            string `json:"review_note,omitempty"`
	RevisedPrompt         string `json:"revised_prompt,omitempty"`
	ImageModel            string `json:"image_model,omitempty"`
	TransparentBackground bool   `json:"transparent_background,omitempty"`
	VariationIntensity    string `json:"variation_intensity,omitempty"`
}

type UpsertStudioBatchRequest struct {
	ID                      string                          `json:"id,omitempty"`
	BatchName               string                          `json:"batch_name,omitempty"`
	Prompt                  string                          `json:"prompt"`
	StyleCount              string                          `json:"style_count,omitempty"`
	VariationIntensity      string                          `json:"variation_intensity,omitempty"`
	ProductImageCount       string                          `json:"product_image_count,omitempty"`
	ProductImagePrompt      string                          `json:"product_image_prompt,omitempty"`
	ProductImagePrompts     []SheinStudioProductImagePrompt `json:"product_image_prompts,omitempty"`
	ArtworkModel            string                          `json:"artwork_model,omitempty"`
	ImageStrategy           string                          `json:"image_strategy,omitempty"`
	SelectedSDSImages       []SheinStudioSelectedSDSImage   `json:"selected_sds_images,omitempty"`
	GroupedSelections       []SheinStudioGroupedSelection   `json:"grouped_selections,omitempty"`
	TransparentBackground   bool                            `json:"transparent_background,omitempty"`
	RenderSizeImagesWithSDS bool                            `json:"render_size_images_with_sds,omitempty"`
	SheinStoreID            string                          `json:"shein_store_id,omitempty"`
	Selection               *SheinStudioSelection           `json:"selection,omitempty"`
	ApprovedDesignIDs       []string                        `json:"approved_design_ids,omitempty"`
	CreatedTasks            []SheinStudioCreatedTask        `json:"created_tasks,omitempty"`
	Designs                 []SheinStudioDesign             `json:"designs,omitempty"`
}

type SheinStudioBatchListItem struct {
	ID                      string                          `json:"id"`
	BatchName               string                          `json:"batch_name,omitempty"`
	Prompt                  string                          `json:"prompt,omitempty"`
	StyleCount              string                          `json:"style_count,omitempty"`
	VariationIntensity      string                          `json:"variation_intensity,omitempty"`
	ProductImageCount       string                          `json:"product_image_count,omitempty"`
	ProductImagePrompt      string                          `json:"product_image_prompt,omitempty"`
	ProductImagePrompts     []SheinStudioProductImagePrompt `json:"product_image_prompts,omitempty"`
	ArtworkModel            string                          `json:"artwork_model,omitempty"`
	ImageStrategy           string                          `json:"image_strategy,omitempty"`
	TransparentBackground   bool                            `json:"transparent_background,omitempty"`
	RenderSizeImagesWithSDS bool                            `json:"render_size_images_with_sds,omitempty"`
	SheinStoreID            string                          `json:"shein_store_id,omitempty"`
	Selection               *SheinStudioSelection           `json:"selection,omitempty"`
	GroupedSelections       []SheinStudioGroupedSelection   `json:"grouped_selections,omitempty"`
	ApprovedDesignIDs       []string                        `json:"approved_design_ids,omitempty"`
	CreatedTasks            []SheinStudioCreatedTask        `json:"created_tasks,omitempty"`
	DesignCount             int                             `json:"design_count"`
	UpdatedAt               string                          `json:"updated_at,omitempty"`
}

type StudioBatchListResponse struct {
	Items []SheinStudioBatchListItem `json:"items,omitempty"`
	Total int                        `json:"total"`
}
