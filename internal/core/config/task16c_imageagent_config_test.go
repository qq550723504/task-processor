package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromBytesBindsImageAgentArtifactStoreYAML(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
openai:
  apiKey: test-key
imageagent:
  artifactStore:
    enabled: true
    provider: s3
    publicBase: https://cdn.example.test/image-agent
    s3:
      bucket: image-agent-artifacts
      region: ap-southeast-1
      endpoint: https://s3.example.test
      accessKeyID: access-key
      secretAccessKey: secret-key
      usePathStyle: true
      artifactMode: cos
      cosImmutableNonVersionedBucketPolicy: true
`))
	require.NoError(t, err)

	store := cfg.ImageAgent.ArtifactStore
	assert.True(t, store.Enabled)
	assert.Equal(t, "s3", store.Provider)
	assert.Equal(t, "https://cdn.example.test/image-agent", store.PublicBase)
	assert.Equal(t, "image-agent-artifacts", store.S3.Bucket)
	assert.Equal(t, "ap-southeast-1", store.S3.Region)
	assert.Equal(t, "https://s3.example.test", store.S3.Endpoint)
	assert.Equal(t, "access-key", store.S3.AccessKeyID)
	assert.Equal(t, "secret-key", store.S3.SecretAccessKey)
	assert.True(t, store.S3.UsePathStyle)
	assert.Equal(t, "cos", store.S3.ArtifactMode)
	assert.True(t, store.S3.COSImmutableNonVersionedBucketPolicy)
}

func TestLoadFromBytesBindsImageAgentArtifactStoreEnvironment(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_ENABLED", "true")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_PROVIDER", "s3")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_PUBLIC_BASE", "https://env-cdn.example.test")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_BUCKET", "env-bucket")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_REGION", "env-region")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_ENDPOINT", "https://env-s3.example.test")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_ACCESS_KEY_ID", "env-access")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_SECRET_ACCESS_KEY", "env-secret")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_USE_PATH_STYLE", "true")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_ARTIFACT_MODE", "aws")
	t.Setenv("TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_COS_IMMUTABLE_NON_VERSIONED_BUCKET_POLICY", "false")

	cfg, err := LoadFromBytes([]byte("openai:\n  apiKey: test-key\n"))
	require.NoError(t, err)

	store := cfg.ImageAgent.ArtifactStore
	assert.True(t, store.Enabled)
	assert.Equal(t, "s3", store.Provider)
	assert.Equal(t, "https://env-cdn.example.test", store.PublicBase)
	assert.Equal(t, "env-bucket", store.S3.Bucket)
	assert.Equal(t, "env-region", store.S3.Region)
	assert.Equal(t, "https://env-s3.example.test", store.S3.Endpoint)
	assert.Equal(t, "env-access", store.S3.AccessKeyID)
	assert.Equal(t, "env-secret", store.S3.SecretAccessKey)
	assert.True(t, store.S3.UsePathStyle)
	assert.Equal(t, "aws", store.S3.ArtifactMode)
	assert.False(t, store.S3.COSImmutableNonVersionedBucketPolicy)
}

func TestLoadFromBytesBindsListingKitOwnedImageUploadYAML(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
openai:
  apiKey: test-key
listingkit:
  imageUpload:
    provider: s3
    local:
      rootDir: ./listingkit-owned
    s3:
      bucket: listingkit-inputs
      region: ap-southeast-1
      endpoint: https://listingkit-s3.example.test
      accessKeyID: listingkit-access
      secretAccessKey: listingkit-secret
      usePathStyle: true
      publicBase: https://listingkit-cdn.example.test
`))
	require.NoError(t, err)

	upload := cfg.ListingKit.ImageUpload
	assert.Equal(t, "s3", upload.Provider)
	assert.Equal(t, "./listingkit-owned", upload.Local.RootDir)
	assert.Equal(t, "listingkit-inputs", upload.S3.Bucket)
	assert.Equal(t, "ap-southeast-1", upload.S3.Region)
	assert.Equal(t, "https://listingkit-s3.example.test", upload.S3.Endpoint)
	assert.Equal(t, "listingkit-access", upload.S3.AccessKeyID)
	assert.Equal(t, "listingkit-secret", upload.S3.SecretAccessKey)
	assert.True(t, upload.S3.UsePathStyle)
	assert.Equal(t, "https://listingkit-cdn.example.test", upload.S3.PublicBase)
}

func TestNewDefaultConfigDoesNotSelectListingKitImageUploadStorage(t *testing.T) {
	cfg := NewDefaultConfig()

	assert.Empty(t, cfg.ListingKit.ImageUpload.Provider)
	assert.Empty(t, cfg.ListingKit.ImageUpload.Local.RootDir)
}

func TestLoadFromBytesDoesNotDefaultListingKitImageUploadToLocal(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_PROVIDER", "")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_LOCAL_ROOT_DIR", "")

	cfg, err := LoadFromBytes([]byte("openai:\n  apiKey: test-key\n"))
	require.NoError(t, err)

	assert.Empty(t, cfg.ListingKit.ImageUpload.Provider)
	assert.Empty(t, cfg.ListingKit.ImageUpload.Local.RootDir)
}

func TestLoadFromBytesBindsListingKitImageUploadS3Environment(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_PROVIDER", "s3")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_BUCKET", "env-listingkit-inputs")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_REGION", "ap-southeast-1")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_ENDPOINT", "https://env-listingkit-s3.example.test")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_ACCESS_KEY_ID", "env-listingkit-access")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_SECRET_ACCESS_KEY", "env-listingkit-secret")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_USE_PATH_STYLE", "true")
	t.Setenv("TASK_PROCESSOR_LISTINGKIT_IMAGE_UPLOAD_S3_PUBLIC_BASE", "https://env-listingkit-cdn.example.test")

	cfg, err := LoadFromBytes([]byte("openai:\n  apiKey: test-key\n"))
	require.NoError(t, err)

	upload := cfg.ListingKit.ImageUpload
	assert.Equal(t, "s3", upload.Provider)
	assert.Equal(t, "env-listingkit-inputs", upload.S3.Bucket)
	assert.Equal(t, "ap-southeast-1", upload.S3.Region)
	assert.Equal(t, "https://env-listingkit-s3.example.test", upload.S3.Endpoint)
	assert.Equal(t, "env-listingkit-access", upload.S3.AccessKeyID)
	assert.Equal(t, "env-listingkit-secret", upload.S3.SecretAccessKey)
	assert.True(t, upload.S3.UsePathStyle)
	assert.Equal(t, "https://env-listingkit-cdn.example.test", upload.S3.PublicBase)
}
