package listingkit

import "time"

type SDSSyncSummary struct {
	VariantID        int64               `json:"variant_id"`
	ProductID        int64               `json:"product_id,omitempty"`
	PrototypeGroupID int64               `json:"prototype_group_id,omitempty"`
	LayerID          string              `json:"layer_id,omitempty"`
	MaterialID       int64               `json:"material_id,omitempty"`
	ProductName      string              `json:"product_name,omitempty"`
	ProductSKU       string              `json:"product_sku,omitempty"`
	VariantSKU       string              `json:"variant_sku,omitempty"`
	VariantSize      string              `json:"variant_size,omitempty"`
	VariantColor     string              `json:"variant_color,omitempty"`
	MockupImageURLs  []string            `json:"mockup_image_urls,omitempty"`
	VariantResults   []SDSSyncSummary    `json:"variant_results,omitempty"`
	Status           string              `json:"status,omitempty"`
	Error            string              `json:"error,omitempty"`
	Diagnostics      *SDSSyncDiagnostics `json:"diagnostics,omitempty"`
}

type SDSSyncDiagnostics struct {
	MaterialImageURL string                             `json:"material_image_url,omitempty"`
	MaterialFileCode string                             `json:"material_file_code,omitempty"`
	MaterialWidth    int                                `json:"material_width,omitempty"`
	MaterialHeight   int                                `json:"material_height,omitempty"`
	LayerContent     string                             `json:"layer_content,omitempty"`
	LayerImgWidth    int                                `json:"layer_img_width,omitempty"`
	LayerImgHeight   int                                `json:"layer_img_height,omitempty"`
	ResizeMode       int                                `json:"resize_mode"`
	FitLevel         float64                            `json:"fit_level,omitempty"`
	RenderedCount    int                                `json:"rendered_count"`
	FinishedProduct  *SDSSyncFinishedProductObservation `json:"finished_product,omitempty"`
	SensitiveWords   []SDSSyncSensitiveWordHit          `json:"sensitive_words,omitempty"`
}

type PodExecutionSummary struct {
	Provider       string                   `json:"provider,omitempty"`
	DependencyMode string                   `json:"dependency_mode,omitempty"`
	Status         string                   `json:"status,omitempty"`
	FailureReason  string                   `json:"failure_reason,omitempty"`
	FallbackType   string                   `json:"fallback_type,omitempty"`
	DecisionSource string                   `json:"decision_source,omitempty"`
	CompletedAt    *time.Time               `json:"completed_at,omitempty"`
	LastAttemptAt  *time.Time               `json:"last_attempt_at,omitempty"`
	RetryCount     int                      `json:"retry_count,omitempty"`
	History        []PodExecutionAuditEvent `json:"history,omitempty"`
}

type PodExecutionAuditEvent struct {
	Kind           string    `json:"kind,omitempty"`
	Code           string    `json:"code,omitempty"`
	Message        string    `json:"message,omitempty"`
	Detail         string    `json:"detail,omitempty"`
	Provider       string    `json:"provider,omitempty"`
	DependencyMode string    `json:"dependency_mode,omitempty"`
	DecisionSource string    `json:"decision_source,omitempty"`
	FromStatus     string    `json:"from_status,omitempty"`
	ToStatus       string    `json:"to_status,omitempty"`
	OccurredAt     time.Time `json:"occurred_at,omitempty"`
}

type SDSSyncFinishedProductObservation struct {
	Found             bool   `json:"found,omitempty"`
	BuildFinish       bool   `json:"build_finish,omitempty"`
	Status            int    `json:"status,omitempty"`
	MaterialImageName string `json:"material_image_name,omitempty"`
	TaskID            string `json:"task_id,omitempty"`
	DesignTaskID      string `json:"design_task_id,omitempty"`
	ItemID            string `json:"item_id,omitempty"`
	ImageCount        int    `json:"image_count,omitempty"`
	ThumbnailCount    int    `json:"thumbnail_count,omitempty"`
}

type SDSSyncSensitiveWordHit struct {
	SensitiveWord string `json:"sensitive_word,omitempty"`
	Type          int    `json:"type,omitempty"`
	TypeStrs      string `json:"type_strs,omitempty"`
	ImgURL        string `json:"img_url,omitempty"`
	IsParent      int    `json:"is_parent,omitempty"`
	PositionStrs  string `json:"position_strs,omitempty"`
}
