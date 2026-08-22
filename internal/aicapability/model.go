// Package aicapability defines provider-neutral AI capability contracts.
package aicapability

import "time"

type Capability string

const (
	CapabilityListingKitStudioImage Capability = "listingkit.studio.image"
	CapabilityProductImageScene     Capability = "productimage.scene_generation"
	CapabilityProductEnrichText     Capability = "productenrich.text_understanding"
	CapabilityProductEnrichVision   Capability = "productenrich.vision_understanding"
	CapabilityProductEnrichListing  Capability = "productenrich.listing_generation"
	CapabilityProductEnrichFusion   Capability = "productenrich.multimodal_fusion"
)

type Operation string

const (
	OperationImageGenerate                   Operation = "image_generate"
	OperationImageEdit                       Operation = "image_edit"
	OperationAsyncImageGenerate              Operation = "async_image_generate"
	OperationAsyncImageEdit                  Operation = "async_image_edit"
	OperationAsyncImageQuery                 Operation = "async_image_query"
	OperationProductImageSceneGenerate       Operation = "productimage_scene_generate"
	OperationProductImageSubjectExtract      Operation = "productimage_subject_extract"
	OperationProductImageWhiteBackground     Operation = "productimage_white_background"
	OperationProductImageReview              Operation = "productimage_review"
	OperationProductEnrichTextExtract        Operation = "productenrich_text_extract_attributes"
	OperationProductEnrichImageAnalyze       Operation = "productenrich_image_analyze"
	OperationProductEnrichJSONGenerate       Operation = "productenrich_json_generate"
	OperationProductEnrichSpecsGenerate      Operation = "productenrich_specs_generate"
	OperationProductEnrichVariantsGenerate   Operation = "productenrich_variants_generate"
	OperationProductEnrichMultimodalFuse     Operation = "productenrich_multimodal_fuse"
	OperationProductEnrichTextQualityScore   Operation = "productenrich_text_quality_score"
	OperationProductEnrichVisionQualityScore Operation = "productenrich_vision_quality_score"
)

type ModelFeature string

const (
	FeatureImageGenerate ModelFeature = "image_generate"
	FeatureImageEdit     ModelFeature = "image_edit"
	FeatureAsyncImageJob ModelFeature = "async_image_job"
	FeatureTextGenerate  ModelFeature = "text_generate"
	FeatureVisionAnalyze ModelFeature = "vision_analyze"
)

type ModelPricing struct {
	Currency         string
	InputUnitMicros  int64
	OutputUnitMicros int64
	ImageUnitMicros  int64
}

type ModelDefinition struct {
	ProviderID           string
	ModelID              string
	RoutingKey           string
	CredentialReference  string
	Features             []ModelFeature
	InputModalities      []string
	OutputModalities     []string
	SupportsJSONSchema   bool
	SupportsTools        bool
	SupportsAsync        bool
	Region               string
	DataPolicyTags       []string
	DefaultTimeout       time.Duration
	MaxConcurrency       int
	Pricing              ModelPricing
	Enabled              bool
	ConfigurationVersion string
}

func (m ModelDefinition) Supports(feature ModelFeature) bool {
	for _, candidate := range m.Features {
		if candidate == feature {
			return true
		}
	}
	return false
}

type TenantModelPolicy struct {
	TenantID                   string
	Capability                 Capability
	AllowedRoutingKeys         []string
	PreferredRoutingKeys       []string
	MaxEstimatedCostMicros     int64
	MaxRuntime                 time.Duration
	RequiredDataPolicyTags     []string
	AllowCrossProviderFallback bool
	CredentialReference        string
	Version                    string
}
