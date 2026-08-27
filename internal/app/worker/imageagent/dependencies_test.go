package imageagentworker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/storage"
	"task-processor/internal/productimage"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func TestResolveImageAgentTemporalDependenciesComposesRealRepositoryExecutorPublisherAndCloser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image-agent-worker-runtime?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	closed := 0
	var capabilityInput productimagehttpapi.RuntimeBuildInput
	resolver := imageAgentWorkerDependencyResolver{
		LoadConfig: func(path string) (*config.Config, error) {
			require.Equal(t, "config/worker.yaml", path)
			cfg := &config.Config{Database: &config.DatabaseConfig{}}
			cfg.ProductImage.WorkDir = "worker-images"
			cfg.ProductImage.Publisher = durablePublisherConfig("aws", true)
			return cfg, nil
		},
		OpenDB:  func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB: func(*config.DatabaseConfig, *gorm.DB) error { closed++; return nil },
		BuildAI: func(*config.Config, *gorm.DB) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error) {
			return nil, nil, nil, nil
		},
		BuildCapabilities: func(input productimagehttpapi.RuntimeBuildInput) (productimagehttpapi.ImageAgentCapabilities, error) {
			capabilityInput = input
			return productimagehttpapi.ImageAgentCapabilities{
				SubjectExtractor: runtimeSubjectExtractor{}, WhiteBackgroundRenderer: runtimeWhiteBackgroundRenderer{},
				SceneRenderer: runtimeSceneRenderer{}, AssetPublisher: runtimeAssetPublisher{},
			}, nil
		},
		BuildArtifactStore: func(*config.Config) (imageagenttemporal.DurableArtifactStore, error) {
			return workerArtifactStore{}, nil
		},
	}

	dependencies, closeFn, err := resolveImageAgentTemporalDependencies("config/worker.yaml", nil, resolver)
	require.NoError(t, err)
	require.NotNil(t, dependencies.Repository)
	require.NotNil(t, dependencies.SlotExecutor)
	require.NotNil(t, dependencies.StagedSlotExecutor)
	require.NotNil(t, dependencies.ArtifactStore)
	require.NotNil(t, dependencies.Publisher)
	require.Equal(t, "worker-images", capabilityInput.ImageWorkDir)
	require.NoError(t, closeFn())
	require.Equal(t, 1, closed)
}

func TestArtifactStorageCapabilitiesFromConfigFailsClosed(t *testing.T) {
	validAWS := durablePublisherConfig("aws", true)
	validCOS := durablePublisherConfig("cos", true)
	tests := []struct {
		name    string
		mutate  func(*config.ProductImagePublisherConfig)
		want    storage.ArtifactStorageCapabilities
		wantErr string
	}{
		{name: "aws", want: storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeAWS}},
		{name: "cos", mutate: func(cfg *config.ProductImagePublisherConfig) { *cfg = validCOS }, want: storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeCOS, COSImmutableNonVersionedBucketPolicy: true}},
		{name: "disabled", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.Enabled = false }, wantErr: "disabled"},
		{name: "wrong provider", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.Provider = "local" }, wantErr: "provider must be s3"},
		{name: "missing bucket", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.S3.Bucket = "" }, wantErr: "bucket"},
		{name: "missing region", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.S3.Region = "" }, wantErr: "region"},
		{name: "missing public URL", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.PublicBase = "" }, wantErr: "public base"},
		{name: "empty mode", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.S3.ArtifactMode = "" }, wantErr: "artifact mode"},
		{name: "unknown mode", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.S3.ArtifactMode = "minio" }, wantErr: "artifact mode"},
		{name: "COS missing endpoint", mutate: func(cfg *config.ProductImagePublisherConfig) { *cfg = validCOS; cfg.S3.Endpoint = "" }, wantErr: "COS endpoint"},
		{name: "COS policy unconfirmed", mutate: func(cfg *config.ProductImagePublisherConfig) {
			*cfg = validCOS
			cfg.S3.COSImmutableNonVersionedBucketPolicy = false
		}, wantErr: "immutable non-versioned"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			publisher := validAWS
			if test.mutate != nil {
				test.mutate(&publisher)
			}
			got, err := artifactStorageCapabilitiesFromConfig(publisher)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestBuildImageAgentDurableArtifactStoreUsesConfiguredS3ClientPath(t *testing.T) {
	cfg := &config.Config{ProductImage: config.ProductImageConfig{Publisher: durablePublisherConfig("aws", true)}}
	artifactStore, err := buildImageAgentDurableArtifactStore(cfg)
	require.NoError(t, err)
	require.NotNil(t, artifactStore)
}

func durablePublisherConfig(mode string, cosPolicy bool) config.ProductImagePublisherConfig {
	return config.ProductImagePublisherConfig{
		Enabled: true, Provider: "s3", PublicBase: "https://cdn.example.test/images",
		S3: config.ProductImagePublisherS3Config{Bucket: "image-assets", Region: "ap-southeast-1", Endpoint: "https://s3.example.test", ArtifactMode: mode, COSImmutableNonVersionedBucketPolicy: cosPolicy},
	}
}

type workerArtifactStore struct{}

func (workerArtifactStore) PrepareSlotArtifacts(objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	return objectstore.PreparedSlotArtifacts{}, nil
}
func (workerArtifactStore) EnsureStaged(context.Context, objectstore.PreparedSlotArtifacts) error {
	return nil
}
func (workerArtifactStore) Finalize(context.Context, imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return imageagent.FinalManifest{}, nil
}

var _ imageagenttemporal.DurableArtifactStore = workerArtifactStore{}

type runtimeSubjectExtractor struct{}

func (runtimeSubjectExtractor) Extract(context.Context, string, *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return &productimage.ImageAsset{}, nil
}

type runtimeWhiteBackgroundRenderer struct{}

func (runtimeWhiteBackgroundRenderer) Render(context.Context, *productimage.ImageAsset, *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return &productimage.ImageAsset{}, nil
}

type runtimeSceneRenderer struct{}

func (runtimeSceneRenderer) Render(context.Context, *productimage.ImageAsset, *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	return nil, nil
}

type runtimeAssetPublisher struct{}

func (runtimeAssetPublisher) Publish(context.Context, *productimage.ImageProcessRequest, *productimage.ImageProcessResult) error {
	return nil
}
