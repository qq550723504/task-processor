package config

// ImageAgentConfig owns the durable runtime settings consumed by ImageAgent.
type ImageAgentConfig struct {
	ArtifactStore ImageAgentArtifactStoreConfig `mapstructure:"artifactStore" yaml:"artifactStore"`
}

type ImageAgentArtifactStoreConfig struct {
	Enabled    bool                            `mapstructure:"enabled" yaml:"enabled"`
	Provider   string                          `mapstructure:"provider" yaml:"provider"`
	PublicBase string                          `mapstructure:"publicBase" yaml:"publicBase"`
	S3         ImageAgentArtifactStoreS3Config `mapstructure:"s3" yaml:"s3"`
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
