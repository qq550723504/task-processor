package config

type AICapabilityConfig struct {
	StudioImageRoutingMode            string   `mapstructure:"studioImageRoutingMode" yaml:"studioImageRoutingMode"`
	ProductImageSceneEnabled          bool     `mapstructure:"productImageSceneEnabled" yaml:"productImageSceneEnabled"`
	ProductImageSceneAllowedTenantIDs []string `mapstructure:"productImageSceneAllowedTenantIDs" yaml:"productImageSceneAllowedTenantIDs"`
}
