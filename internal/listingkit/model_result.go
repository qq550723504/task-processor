package listingkit

import (
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

type ListingKitResult struct {
	TaskID                      string                           `json:"task_id"`
	Status                      string                           `json:"status"`
	ReviewReasons               []string                         `json:"review_reasons,omitempty"`
	Platforms                   []string                         `json:"platforms,omitempty"`
	Country                     string                           `json:"country,omitempty"`
	Language                    string                           `json:"language,omitempty"`
	PodExecution                *PodExecutionSummary             `json:"pod_execution,omitempty"`
	StandardProductSnapshot     *StandardProductSnapshot         `json:"standard_product_snapshot,omitempty"`
	CatalogProduct              *catalog.Product                 `json:"catalog_product,omitempty"`
	AssetBundle                 *asset.Bundle                    `json:"asset_bundle,omitempty"`
	AssetInventorySummary       *asset.InventorySummary          `json:"asset_inventory_summary,omitempty"`
	AssetRenderPreviews         []AssetRenderPreview             `json:"asset_render_previews,omitempty"`
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews    `json:"platform_asset_render_previews,omitempty"`
	AssetGenerationSummary      *AssetGenerationSummary          `json:"asset_generation_summary,omitempty"`
	AssetGenerationTasks        []assetgeneration.Task           `json:"asset_generation_tasks,omitempty"`
	AssetGenerationQueue        *GenerationWorkQueue             `json:"asset_generation_queue,omitempty"`
	AssetGenerationOverview     *AssetGenerationOverview         `json:"asset_generation_overview,omitempty"`
	ReviewSummary               *GenerationReviewSummary         `json:"review_summary,omitempty"`
	ReviewRecords               []GenerationReviewRecord         `json:"review_records,omitempty"`
	CanonicalProduct            *canonical.Product               `json:"canonical_product,omitempty"`
	ImageAssets                 *productimage.ImageProcessResult `json:"image_assets,omitempty"`
	// Deprecated: kept only for JSON/history compatibility. New business code should use SDSDesignResult.
	SDSSync *SDSSyncSummary `json:"sds_sync,omitempty"`
	// SDSDesignResult is the canonical SDS design execution result used by current business logic.
	SDSDesignResult      *SDSSyncSummary              `json:"sds_design_result,omitempty"`
	Amazon               *AmazonPackage               `json:"amazon,omitempty"`
	Shein                *sheinpub.Package            `json:"shein,omitempty"`
	SheinStoreResolution *SheinStoreResolutionSummary `json:"shein_store_resolution,omitempty"`
	Temu                 *TemuPackage                 `json:"temu,omitempty"`
	Walmart              *WalmartPackage              `json:"walmart,omitempty"`
	Summary              *GenerationSummary           `json:"summary,omitempty"`
	Revision             *ListingKitRevisionSummary   `json:"revision,omitempty"`
	RevisionHistoryTotal int                          `json:"revision_history_total,omitempty"`
	RevisionHistory      []ListingKitRevisionRecord   `json:"revision_history,omitempty"`
	ChildTasks           []ChildTaskState             `json:"child_tasks,omitempty"`
	WorkflowStages       []WorkflowStage              `json:"workflow_stages,omitempty"`
	WorkflowIssues       []WorkflowIssue              `json:"workflow_issues,omitempty"`
	CreatedAt            time.Time                    `json:"created_at"`
	UpdatedAt            time.Time                    `json:"updated_at"`
}

// StandardProductSnapshot captures the stable boundary between the standard
// product layer and later platform adaptation. It is intentionally persisted
// on the task result so the standard layer can later be executed and resumed
// independently from platform-specific workflows such as SHEIN adaptation.
type StandardProductSnapshot struct {
	CatalogProduct        *catalog.Product                 `json:"catalog_product,omitempty"`
	CanonicalProduct      *canonical.Product               `json:"canonical_product,omitempty"`
	AssetBundle           *asset.Bundle                    `json:"asset_bundle,omitempty"`
	AssetInventorySummary *asset.InventorySummary          `json:"asset_inventory_summary,omitempty"`
	ImageAssets           *productimage.ImageProcessResult `json:"image_assets,omitempty"`
	PodExecution          *PodExecutionSummary             `json:"pod_execution,omitempty"`
	// Deprecated: kept only for JSON/history compatibility. New business code should use SDSDesignResult.
	SDSSync *SDSSyncSummary `json:"sds_sync,omitempty"`
	// SDSDesignResult is the canonical SDS design execution result used by current business logic.
	SDSDesignResult *SDSSyncSummary    `json:"sds_design_result,omitempty"`
	Summary         *GenerationSummary `json:"summary,omitempty"`
	ChildTasks      []ChildTaskState   `json:"child_tasks,omitempty"`
	WorkflowStages  []WorkflowStage    `json:"workflow_stages,omitempty"`
	WorkflowIssues  []WorkflowIssue    `json:"workflow_issues,omitempty"`
}

type GenerationSummary struct {
	SourceType    string   `json:"source_type,omitempty"`
	ImageCount    int      `json:"image_count"`
	VariantCount  int      `json:"variant_count"`
	NeedsReview   bool     `json:"needs_review"`
	Warnings      []string `json:"warnings,omitempty"`
	IssueCount    int      `json:"issue_count,omitempty"`
	WarningCount  int      `json:"warning_count,omitempty"`
	ReviewCount   int      `json:"review_count,omitempty"`
	BlockingCount int      `json:"blocking_count,omitempty"`
}
