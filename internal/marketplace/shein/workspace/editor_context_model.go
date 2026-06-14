package workspace

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

type EditorContext struct {
	Basics           *EditorBasicsContext        `json:"basics,omitempty"`
	Category         *EditorCategoryContext      `json:"category,omitempty"`
	Attributes       *EditorAttributeContext     `json:"attributes,omitempty"`
	SaleAttributes   *EditorSaleAttributeContext `json:"sale_attributes,omitempty"`
	RevisionSkeleton *EditorRevisionSkeleton     `json:"revision_skeleton,omitempty"`
	DirtyHints       *EditorDirtyHints           `json:"dirty_hints,omitempty"`
	Progress         *EditorProgress             `json:"progress,omitempty"`
}

type EditorBasicsContext struct {
	SpuName       string           `json:"spu_name,omitempty"`
	ProductNameEn string           `json:"product_name_en,omitempty"`
	BrandName     string           `json:"brand_name,omitempty"`
	Description   string           `json:"description,omitempty"`
	Images        *common.ImageSet `json:"images,omitempty"`
	ReviewNotes   []string         `json:"review_notes,omitempty"`
}

type EditorCategoryContext struct {
	Current        *sheinpub.InspectionCategoryPayload `json:"current,omitempty"`
	SuggestedPatch *CategoryResolutionPatch            `json:"suggested_patch,omitempty"`
	Recommendation *EditorRecommendationMeta           `json:"recommendation,omitempty"`
	PreviewEffects []EditorEffect                      `json:"preview_effects,omitempty"`
}

type EditorAttributeContext struct {
	Current        *sheinpub.InspectionAttributePayload `json:"current,omitempty"`
	SuggestedPatch *AttributeResolutionPatch            `json:"suggested_patch,omitempty"`
	Recommendation *EditorRecommendationMeta            `json:"recommendation,omitempty"`
	Suggestions    []EditorAttributeSuggestion          `json:"suggestions,omitempty"`
	PreviewEffects []EditorEffect                       `json:"preview_effects,omitempty"`
}

type EditorSaleAttributeContext struct {
	Current                  *sheinpub.InspectionSaleAttributePayload `json:"current,omitempty"`
	SuggestedResolutionPatch *SaleAttributeResolutionPatch            `json:"suggested_resolution_patch,omitempty"`
	SuggestedSKCPatches      []SKCRevisionPatch                       `json:"suggested_skc_patches,omitempty"`
	Recommendation           *EditorRecommendationMeta                `json:"recommendation,omitempty"`
	CandidateSuggestions     []EditorSaleCandidateSuggestion          `json:"candidate_suggestions,omitempty"`
	PreviewEffects           []EditorEffect                           `json:"preview_effects,omitempty"`
}
