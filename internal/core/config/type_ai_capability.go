package config

type AICapabilityConfig struct {
	StudioImageRoutingMode               string   `mapstructure:"studioImageRoutingMode" yaml:"studioImageRoutingMode"`
	ProductImageSceneEnabled             bool     `mapstructure:"productImageSceneEnabled" yaml:"productImageSceneEnabled"`
	ProductImageSceneAllowedTenantIDs    []string `mapstructure:"productImageSceneAllowedTenantIDs" yaml:"productImageSceneAllowedTenantIDs"`
	ProductEnrichTextEnabled             bool     `mapstructure:"productEnrichTextEnabled" yaml:"productEnrichTextEnabled"`
	ProductEnrichTextAllowedTenantIDs    []string `mapstructure:"productEnrichTextAllowedTenantIDs" yaml:"productEnrichTextAllowedTenantIDs"`
	ProductEnrichVisionEnabled           bool     `mapstructure:"productEnrichVisionEnabled" yaml:"productEnrichVisionEnabled"`
	ProductEnrichVisionAllowedTenantIDs  []string `mapstructure:"productEnrichVisionAllowedTenantIDs" yaml:"productEnrichVisionAllowedTenantIDs"`
	ProductEnrichListingEnabled          bool     `mapstructure:"productEnrichListingEnabled" yaml:"productEnrichListingEnabled"`
	ProductEnrichListingAllowedTenantIDs []string `mapstructure:"productEnrichListingAllowedTenantIDs" yaml:"productEnrichListingAllowedTenantIDs"`
}
