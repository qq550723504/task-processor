package config

type AICapabilityConfig struct {
	StudioImageRoutingMode   string `mapstructure:"studioImageRoutingMode" yaml:"studioImageRoutingMode"`
	ProductImageSceneEnabled bool   `mapstructure:"productImageSceneEnabled" yaml:"productImageSceneEnabled"`
}
