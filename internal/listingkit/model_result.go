package listingkit

import (
	"sort"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	listingplatform "task-processor/internal/listing/platform"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

type ListingKitResult struct {
	TaskID                          string                                      `json:"task_id"`
	Status                          string                                      `json:"status"`
	ReviewReasons                   []string                                    `json:"review_reasons,omitempty"`
	Platforms                       []string                                    `json:"platforms,omitempty"`
	Country                         string                                      `json:"country,omitempty"`
	Language                        string                                      `json:"language,omitempty"`
	PodExecution                    *PodExecutionSummary                        `json:"pod_execution,omitempty"`
	StandardProductSnapshot         *StandardProductSnapshot                    `json:"standard_product_snapshot,omitempty"`
	CatalogProduct                  *catalog.Product                            `json:"catalog_product,omitempty"`
	AssetBundle                     *asset.Bundle                               `json:"asset_bundle,omitempty"`
	AssetInventorySummary           *asset.InventorySummary                     `json:"asset_inventory_summary,omitempty"`
	AssetBundlesByTarget            map[string]*asset.Bundle                    `json:"asset_bundles_by_target,omitempty"`
	AssetInventorySummariesByTarget map[string]*asset.InventorySummary          `json:"asset_inventory_summaries_by_target,omitempty"`
	AssetRenderPreviews             []AssetRenderPreview                        `json:"asset_render_previews,omitempty"`
	PlatformAssetRenderPreviews     []PlatformAssetRenderPreviews               `json:"platform_asset_render_previews,omitempty"`
	AssetGenerationSummary          *AssetGenerationSummary                     `json:"asset_generation_summary,omitempty"`
	AssetGenerationTasks            []assetgeneration.Task                      `json:"asset_generation_tasks,omitempty"`
	AssetGenerationQueue            *GenerationWorkQueue                        `json:"asset_generation_queue,omitempty"`
	AssetGenerationOverview         *AssetGenerationOverview                    `json:"asset_generation_overview,omitempty"`
	ReviewSummary                   *GenerationReviewSummary                    `json:"review_summary,omitempty"`
	ReviewRecords                   []GenerationReviewRecord                    `json:"review_records,omitempty"`
	CanonicalProduct                *canonical.Product                          `json:"canonical_product,omitempty"`
	ImageAssets                     *productimage.ImageProcessResult            `json:"image_assets,omitempty"`
	ImageAssetsByTarget             map[string]*productimage.ImageProcessResult `json:"image_assets_by_target,omitempty"`
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

func (r *ListingKitResult) ImageAssetsForTarget(target string) *productimage.ImageProcessResult {
	if r == nil {
		return nil
	}
	if len(r.ImageAssetsByTarget) == 0 {
		return r.ImageAssets
	}
	return r.ImageAssetsByTarget[listingplatform.Normalize(target)]
}

func (r *ListingKitResult) AssetBundleForTarget(target string) *asset.Bundle {
	if r == nil {
		return nil
	}
	if len(r.AssetBundlesByTarget) == 0 {
		return r.AssetBundle
	}
	return r.AssetBundlesByTarget[listingplatform.Normalize(target)]
}

func (r *ListingKitResult) AssetInventorySummaryForTarget(target string) *asset.InventorySummary {
	if r == nil {
		return nil
	}
	if len(r.AssetInventorySummariesByTarget) == 0 {
		return r.AssetInventorySummary
	}
	return r.AssetInventorySummariesByTarget[listingplatform.Normalize(target)]
}

func (r *ListingKitResult) recordTargetImageAssets(target string, image *productimage.ImageProcessResult, bundle *asset.Bundle, summary *asset.InventorySummary, compatibilityTarget string) {
	if r == nil || !listingplatform.IsSupported(target) {
		return
	}
	target = listingplatform.Normalize(target)
	if r.ImageAssetsByTarget == nil {
		r.ImageAssetsByTarget = map[string]*productimage.ImageProcessResult{}
	}
	if r.AssetBundlesByTarget == nil {
		r.AssetBundlesByTarget = map[string]*asset.Bundle{}
	}
	if r.AssetInventorySummariesByTarget == nil {
		r.AssetInventorySummariesByTarget = map[string]*asset.InventorySummary{}
	}
	r.ImageAssetsByTarget[target] = image
	r.AssetBundlesByTarget[target] = bundle
	r.AssetInventorySummariesByTarget[target] = summary
	r.applyCompatibilityAssetProjection(compatibilityTarget)
}

func (r *ListingKitResult) applyCompatibilityAssetProjection(compatibilityTarget string) {
	if r == nil {
		return
	}
	if len(r.ImageAssetsByTarget) == 0 {
		return
	}
	target := listingplatform.Normalize(compatibilityTarget)
	if target == "" && len(r.ImageAssetsByTarget) == 1 {
		for target = range r.ImageAssetsByTarget {
			break
		}
	}
	if target == "" || r.ImageAssetsByTarget[target] == nil {
		r.ImageAssets = nil
		r.AssetBundle = nil
		r.AssetInventorySummary = nil
		return
	}
	r.ImageAssets = r.ImageAssetsByTarget[target]
	r.AssetBundle = r.AssetBundlesByTarget[target]
	r.AssetInventorySummary = r.AssetInventorySummariesByTarget[target]
}

// StandardProductSnapshot captures the stable boundary between the standard
// product layer and later platform adaptation. It is intentionally persisted
// on the task result so the standard layer can later be executed and resumed
// independently from platform-specific workflows such as SHEIN adaptation.
type StandardProductSnapshot struct {
	CatalogProduct                  *catalog.Product                            `json:"catalog_product,omitempty"`
	CanonicalProduct                *canonical.Product                          `json:"canonical_product,omitempty"`
	AssetBundle                     *asset.Bundle                               `json:"asset_bundle,omitempty"`
	AssetInventorySummary           *asset.InventorySummary                     `json:"asset_inventory_summary,omitempty"`
	ImageAssets                     *productimage.ImageProcessResult            `json:"image_assets,omitempty"`
	ImageAssetsByTarget             map[string]*productimage.ImageProcessResult `json:"image_assets_by_target,omitempty"`
	AssetBundlesByTarget            map[string]*asset.Bundle                    `json:"asset_bundles_by_target,omitempty"`
	AssetInventorySummariesByTarget map[string]*asset.InventorySummary          `json:"asset_inventory_summaries_by_target,omitempty"`
	PodExecution                    *PodExecutionSummary                        `json:"pod_execution,omitempty"`
	// Deprecated: kept only for JSON/history compatibility. New business code should use SDSDesignResult.
	SDSSync *SDSSyncSummary `json:"sds_sync,omitempty"`
	// SDSDesignResult is the canonical SDS design execution result used by current business logic.
	SDSDesignResult *SDSSyncSummary    `json:"sds_design_result,omitempty"`
	Summary         *GenerationSummary `json:"summary,omitempty"`
	ChildTasks      []ChildTaskState   `json:"child_tasks,omitempty"`
	WorkflowStages  []WorkflowStage    `json:"workflow_stages,omitempty"`
	WorkflowIssues  []WorkflowIssue    `json:"workflow_issues,omitempty"`
}

func (r *ListingKitResult) assetBundleForInventory() *asset.Bundle {
	if r == nil {
		return nil
	}
	if len(r.AssetBundlesByTarget) == 0 {
		return r.AssetBundle
	}
	if len(r.AssetBundlesByTarget) == 1 {
		for _, bundle := range r.AssetBundlesByTarget {
			return bundle
		}
	}
	targets := make([]string, 0, len(r.AssetBundlesByTarget))
	for target := range r.AssetBundlesByTarget {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	combined := &asset.Bundle{}
	for _, target := range targets {
		bundle := r.AssetBundlesByTarget[target]
		if bundle == nil {
			continue
		}
		combined.Assets = append(combined.Assets, bundle.Assets...)
	}
	return combined
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
