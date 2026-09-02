package listingkit

import "errors"

var ErrStudioSessionNotFound = errors.New("studio session not found")
var ErrStudioSessionConflict = errors.New("studio session has been updated by another request")

type EnsureStudioSessionRequest struct {
	UserID    string                `json:"user_id,omitempty"`
	Selection *SheinStudioSelection `json:"selection,omitempty"`
}

type UpdateStudioSessionRequest struct {
	Status                    *SheinStudioSessionStatus     `json:"status,omitempty"`
	ExpectedUpdatedAt         *string                       `json:"expected_updated_at,omitempty"`
	Prompt                    *string                       `json:"prompt,omitempty"`
	PromptMode                *string                       `json:"prompt_mode,omitempty"`
	StyleCount                *string                       `json:"style_count,omitempty"`
	VariationIntensity        *string                       `json:"variation_intensity,omitempty"`
	ArtworkModel              *string                       `json:"artwork_model,omitempty"`
	GroupedImageMode          *string                       `json:"grouped_image_mode,omitempty"`
	SelectedSDSImages         []SheinStudioSelectedSDSImage `json:"selected_sds_images,omitempty"`
	GroupedSelections         []SheinStudioGroupedSelection `json:"grouped_selections,omitempty"`
	TransparentBackground     *bool                         `json:"transparent_background,omitempty"`
	TransparentBackgroundMode *StudioTransparencyMode       `json:"transparent_background_mode,omitempty"`
	RenderSizeImagesWithSDS   *bool                         `json:"render_size_images_with_sds,omitempty"`
	SheinStoreID              *string                       `json:"shein_store_id,omitempty"`
	GenerationJobID           *string                       `json:"generation_job_id,omitempty"`
	GenerationJobs            []SheinStudioGenerationJob    `json:"generation_jobs,omitempty"`
	GenerationError           *string                       `json:"generation_error,omitempty"`
	ApprovedDesignIDs         []string                      `json:"approved_design_ids,omitempty"`
}

type RetryStudioBatchItemsRequest struct {
	ItemIDs []string `json:"item_ids,omitempty"`
}

type StudioSessionGalleryResponse struct {
	Items []SheinStudioSessionGalleryItem `json:"items,omitempty"`
	Total int                             `json:"total"`
}
