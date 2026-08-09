// Package aicapability defines provider-neutral AI capability contracts.
package aicapability

import "time"

type Capability string

const (
	CapabilityListingKitStudioImage Capability = "listingkit.studio.image"
	CapabilityProductImageScene     Capability = "productimage.scene_generation"
)

type Operation string

const (
	OperationImageGenerate             Operation = "image_generate"
	OperationImageEdit                 Operation = "image_edit"
	OperationAsyncImageGenerate        Operation = "async_image_generate"
	OperationAsyncImageEdit            Operation = "async_image_edit"
	OperationAsyncImageQuery           Operation = "async_image_query"
	OperationProductImageSceneGenerate Operation = "productimage_scene_generate"
)

type ModelFeature string

const (
	FeatureImageGenerate ModelFeature = "image_generate"
	FeatureImageEdit     ModelFeature = "image_edit"
	FeatureAsyncImageJob ModelFeature = "async_image_job"
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
