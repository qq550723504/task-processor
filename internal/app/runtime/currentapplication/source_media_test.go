package currentapplication

import (
	"github.com/stretchr/testify/require"
	coreconfig "task-processor/internal/core/config"
	"testing"
)

func TestSourceMediaForCurrentCollectionsDoesNotRequireOfficialPlatform(t *testing.T) {
	c := acquisitionRuntimeConfig()
	c.ProductCollections = true
	c.SourceMedia = &coreconfig.ImageAgentArtifactStoreConfig{Enabled: true, Provider: "s3", PublicBase: "https://files.example.org/products", S3: coreconfig.ImageAgentArtifactStoreS3Config{Bucket: "products", Region: "us-east-1", AccessKeyID: "synthetic-access", SecretAccessKey: "synthetic-secret", ArtifactMode: "aws"}}
	require.Nil(t, c.StoreCenter)
	require.Nil(t, c.SupplyChain)
	require.NoError(t, c.validate(), "current Product file uploads do not consume platform rules")
}

func TestSourceMediaMapsCompleteStorageWithoutEnablingAgent(t *testing.T) {
	c := supplyRuntimeConfig(t)
	wanted := coreconfig.ImageAgentArtifactStoreConfig{Enabled: true, Provider: "s3", PublicBase: "https://files.example.org/products", S3: coreconfig.ImageAgentArtifactStoreS3Config{Bucket: "source-products", Region: "us-east-1", Endpoint: "https://storage.example.org", AccessKeyID: "synthetic-access", SecretAccessKey: "synthetic-secret", UsePathStyle: true, ArtifactMode: "aws"}}
	c.SourceMedia = &wanted
	require.NoError(t, c.validate())
	core := c.CoreConfig()
	require.Equal(t, wanted, core.ProductCollectionSourceMedia)
	require.False(t, core.ImageAgent.Admission.Enabled)
	require.False(t, core.ImageAgent.ArtifactStore.Enabled)
	require.Nil(t, c.ImageAgent)
}

func TestSourceMediaRejectsIncompleteOrIncorrectStorage(t *testing.T) {
	c := supplyRuntimeConfig(t)
	base := coreconfig.ImageAgentArtifactStoreConfig{Enabled: true, Provider: "s3", PublicBase: "https://files.example.org/products", S3: coreconfig.ImageAgentArtifactStoreS3Config{Bucket: "products", Region: "us-east-1", AccessKeyID: "synthetic-access", SecretAccessKey: "synthetic-secret", ArtifactMode: "aws"}}
	for name, change := range map[string]func(*coreconfig.ImageAgentArtifactStoreConfig){"no credentials": func(v *coreconfig.ImageAgentArtifactStoreConfig) { v.S3.SecretAccessKey = "" }, "no bucket": func(v *coreconfig.ImageAgentArtifactStoreConfig) { v.S3.Bucket = "" }, "private public URL": func(v *coreconfig.ImageAgentArtifactStoreConfig) { v.PublicBase = "https://127.0.0.1/images" }, "cos missing immutable policy": func(v *coreconfig.ImageAgentArtifactStoreConfig) {
		v.S3.ArtifactMode = "cos"
		v.S3.Endpoint = "https://storage.example.org"
	}, "unknown mode": func(v *coreconfig.ImageAgentArtifactStoreConfig) { v.S3.ArtifactMode = "other" }} {
		t.Run(name, func(t *testing.T) {
			value := base
			change(&value)
			c.SourceMedia = &value
			require.Error(t, c.validate())
		})
	}
	c.SourceMedia = &base
	c.SupplyChain = nil
	require.NoError(t, c.validate())
	c.ProductCollections = false
	require.Error(t, c.validate())
	c.SourceMedia = nil
	require.NoError(t, c.validate())
}
