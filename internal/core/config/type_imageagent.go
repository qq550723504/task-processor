package config

import "strings"

// ImageAgentConfig owns the durable runtime settings consumed by ImageAgent.
type ImageAgentConfig struct {
	Generation    ImageAgentGenerationConfig    `mapstructure:"generation" yaml:"generation"`
	Admission     ImageAgentAdmissionConfig     `mapstructure:"admission" yaml:"admission"`
	ArtifactStore ImageAgentArtifactStoreConfig `mapstructure:"artifactStore" yaml:"artifactStore"`
}

// ImageAgentGenerationConfig is a server-owned price, never a sample balance
// or provider token conversion. Absent configuration keeps generation closed.
type ImageAgentGenerationConfig struct {
	PriceVersion   string `mapstructure:"priceVersion" yaml:"priceVersion" json:"priceVersion"`
	PointsPerImage int64  `mapstructure:"pointsPerImage" yaml:"pointsPerImage" json:"pointsPerImage"`
}

func (c ImageAgentGenerationConfig) Configured() bool {
	return c.PointsPerImage > 0 && c.PriceVersion != "" && len(c.PriceVersion) <= 192 && strings.TrimSpace(c.PriceVersion) == c.PriceVersion && !strings.ContainsAny(c.PriceVersion, "\r\n\x00")
}

// ImageAgentAdmissionConfig is the explicit tenant boundary for provider work.
// Route permissions identify the caller; this gate identifies eligible tenants.
type ImageAgentAdmissionConfig struct {
	Enabled          bool     `mapstructure:"enabled" yaml:"enabled"`
	AllowedTenantIDs []string `mapstructure:"allowedTenantIDs" yaml:"allowedTenantIDs"`
}

type ImageAgentArtifactStoreConfig struct {
	Enabled                    bool                            `mapstructure:"enabled" yaml:"enabled"`
	Provider                   string                          `mapstructure:"provider" yaml:"provider"`
	PublicBase                 string                          `mapstructure:"publicBase" yaml:"publicBase"`
	IsolatedTrialGeneratedURLs bool                            `mapstructure:"isolatedTrialGeneratedURLs" yaml:"isolatedTrialGeneratedURLs"`
	S3                         ImageAgentArtifactStoreS3Config `mapstructure:"s3" yaml:"s3"`
}

type ImageAgentArtifactStoreS3Config struct {
	Bucket                               string `mapstructure:"bucket" yaml:"bucket"`
	Region                               string `mapstructure:"region" yaml:"region"`
	Endpoint                             string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKeyID                          string `mapstructure:"accessKeyID" yaml:"accessKeyID"`
	SecretAccessKey                      string `mapstructure:"secretAccessKey" yaml:"secretAccessKey"`
	UsePathStyle                         bool   `mapstructure:"usePathStyle" yaml:"usePathStyle"`
	ArtifactMode                         string `mapstructure:"artifactMode" yaml:"artifactMode"`
	COSImmutableNonVersionedBucketPolicy bool   `mapstructure:"cosImmutableNonVersionedBucketPolicy" yaml:"cosImmutableNonVersionedBucketPolicy"`
}
