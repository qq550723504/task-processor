package imageagentworker

import (
	"context"
	"errors"
	"testing"
	"time"

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
		BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
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

func TestResolveImageAgentTemporalDependenciesForV2SkipsV3ValidationAndDurableComposition(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image-agent-worker-v2-runtime?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	cfg := &config.Config{Database: &config.DatabaseConfig{}}
	cfg.ProductImage.Publisher = durablePublisherConfig("invalid-v3-mode", false)
	storeBuilds := 0
	resolver := imageAgentWorkerDependencyResolver{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB:    func(*config.DatabaseConfig, *gorm.DB) error { return nil },
		BuildAI: func(*config.Config, *gorm.DB) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error) {
			return nil, nil, nil, nil
		},
		BuildCapabilities: func(productimagehttpapi.RuntimeBuildInput) (productimagehttpapi.ImageAgentCapabilities, error) {
			return productimagehttpapi.ImageAgentCapabilities{
				SubjectExtractor: runtimeSubjectExtractor{}, WhiteBackgroundRenderer: runtimeWhiteBackgroundRenderer{},
				SceneRenderer: runtimeSceneRenderer{}, AssetPublisher: runtimeAssetPublisher{},
			}, nil
		},
		BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
			storeBuilds++
			return workerArtifactStore{}, nil
		},
		ArtifactTiming: imageAgentArtifactTiming{OperationTimeout: 2 * time.Minute, PublicationLeaseDuration: time.Minute},
	}

	dependencies, closeFn, err := resolveImageAgentTemporalDependenciesForMode("config/worker.yaml", nil, imageagenttemporal.WorkerWireModeV2, resolver)
	require.NoError(t, err)
	require.NotNil(t, closeFn)
	require.NotNil(t, dependencies.Repository)
	require.NotNil(t, dependencies.SlotExecutor)
	require.NotNil(t, dependencies.Publisher)
	require.Nil(t, dependencies.StagedSlotExecutor)
	require.Nil(t, dependencies.ArtifactStore)
	require.Nil(t, dependencies.PublisherV3)
	require.Zero(t, dependencies.PublicationLeaseDuration)
	require.Zero(t, storeBuilds)
}

func TestResolveImageAgentTemporalDependenciesForV2AllowsAbsentV3OnlyFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image-agent-worker-v2-no-v3-fields?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	cfg := &config.Config{Database: &config.DatabaseConfig{}}
	cfg.ProductImage.Publisher = durablePublisherConfig("", false)
	storeBuilds := 0
	dependencies, closeFn, err := resolveImageAgentTemporalDependenciesForMode("config/worker.yaml", nil, imageagenttemporal.WorkerWireModeV2, imageAgentWorkerDependencyResolver{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB:    func(*config.DatabaseConfig, *gorm.DB) error { return nil },
		BuildAI: func(*config.Config, *gorm.DB) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error) {
			return nil, nil, nil, nil
		},
		BuildCapabilities: func(productimagehttpapi.RuntimeBuildInput) (productimagehttpapi.ImageAgentCapabilities, error) {
			return productimagehttpapi.ImageAgentCapabilities{
				SubjectExtractor: runtimeSubjectExtractor{}, WhiteBackgroundRenderer: runtimeWhiteBackgroundRenderer{},
				SceneRenderer: runtimeSceneRenderer{}, AssetPublisher: runtimeAssetPublisher{},
			}, nil
		},
		BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
			storeBuilds++
			return workerArtifactStore{}, nil
		},
		ArtifactTiming: imageAgentArtifactTiming{},
	})
	require.NoError(t, err)
	require.NotNil(t, closeFn)
	require.NotNil(t, dependencies.Repository)
	require.NotNil(t, dependencies.SlotExecutor)
	require.NotNil(t, dependencies.Publisher)
	require.Nil(t, dependencies.StagedSlotExecutor)
	require.Nil(t, dependencies.ArtifactStore)
	require.Nil(t, dependencies.PublisherV3)
	require.Zero(t, dependencies.PublicationLeaseDuration)
	require.Zero(t, storeBuilds)
}

func TestResolveImageAgentTemporalDependenciesForV3FailsBeforeDatabaseOnInvalidPolicy(t *testing.T) {
	cfg := &config.Config{Database: &config.DatabaseConfig{}}
	cfg.ProductImage.Publisher = durablePublisherConfig("cos", false)
	databaseOpens, storeBuilds := 0, 0
	_, _, err := resolveImageAgentTemporalDependenciesForMode("config/worker.yaml", nil, imageagenttemporal.WorkerWireModeV3, imageAgentWorkerDependencyResolver{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			databaseOpens++
			return &gorm.DB{}, nil
		},
		BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
			storeBuilds++
			return workerArtifactStore{}, nil
		},
	})
	require.ErrorContains(t, err, "immutable non-versioned")
	require.Zero(t, databaseOpens)
	require.Zero(t, storeBuilds)
}

func TestResolveImageAgentTemporalDependenciesForV3FailsBeforeDatabaseWhenArtifactModeIsAbsent(t *testing.T) {
	cfg := &config.Config{Database: &config.DatabaseConfig{}}
	cfg.ProductImage.Publisher = durablePublisherConfig("", false)
	databaseOpens, storeBuilds := 0, 0
	_, _, err := resolveImageAgentTemporalDependenciesForMode("config/worker.yaml", nil, imageagenttemporal.WorkerWireModeV3, imageAgentWorkerDependencyResolver{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			databaseOpens++
			return &gorm.DB{}, nil
		},
		BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
			storeBuilds++
			return workerArtifactStore{}, nil
		},
	})
	require.ErrorContains(t, err, "artifact mode")
	require.Zero(t, databaseOpens)
	require.Zero(t, storeBuilds)
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
		{name: "missing credentials", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.S3.AccessKeyID = ""; cfg.S3.SecretAccessKey = "" }, wantErr: "access key ID and secret access key"},
		{name: "missing access key", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.S3.AccessKeyID = "" }, wantErr: "access key ID and secret access key"},
		{name: "missing secret key", mutate: func(cfg *config.ProductImagePublisherConfig) { cfg.S3.SecretAccessKey = "" }, wantErr: "access key ID and secret access key"},
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

func TestResolveImageAgentTemporalDependenciesRejectsCredentialsBeforeDatabaseOpen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		access string
		secret string
	}{
		{name: "both missing"},
		{name: "access missing", secret: "do-not-leak-secret"},
		{name: "secret missing", access: "do-not-leak-access"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Database: &config.DatabaseConfig{}}
			cfg.ProductImage.Publisher = durablePublisherConfig("aws", true)
			cfg.ProductImage.Publisher.S3.AccessKeyID = tc.access
			cfg.ProductImage.Publisher.S3.SecretAccessKey = tc.secret
			dbOpens, storeBuilds := 0, 0
			_, _, err := resolveImageAgentTemporalDependencies("config/worker.yaml", nil, imageAgentWorkerDependencyResolver{
				LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
				OpenDB:     func(*config.DatabaseConfig) (*gorm.DB, error) { dbOpens++; return &gorm.DB{}, nil },
				BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
					storeBuilds++
					return workerArtifactStore{}, nil
				},
			})
			require.ErrorContains(t, err, "access key ID and secret access key")
			if tc.access != "" {
				require.NotContains(t, err.Error(), tc.access)
			}
			if tc.secret != "" {
				require.NotContains(t, err.Error(), tc.secret)
			}
			require.Zero(t, dbOpens)
			require.Zero(t, storeBuilds)
		})
	}
}

func TestResolveImageAgentTemporalDependenciesRejectsOperationTimeoutOutsidePublicationLeaseBeforeStoreOrDatabase(t *testing.T) {
	for _, tc := range []struct {
		name   string
		timing imageAgentArtifactTiming
	}{
		{name: "zero operation timeout", timing: imageAgentArtifactTiming{PublicationLeaseDuration: time.Minute}},
		{name: "zero publication lease", timing: imageAgentArtifactTiming{OperationTimeout: time.Second}},
		{name: "equal to publication lease", timing: imageAgentArtifactTiming{PublicationLeaseDuration: time.Minute, OperationTimeout: time.Minute}},
		{name: "longer than publication lease", timing: imageAgentArtifactTiming{PublicationLeaseDuration: time.Minute, OperationTimeout: time.Minute + time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Database: &config.DatabaseConfig{}}
			cfg.ProductImage.Publisher = durablePublisherConfig("aws", true)
			storeBuilds, databaseOpens := 0, 0
			_, _, err := resolveImageAgentTemporalDependencies("config/worker.yaml", nil, imageAgentWorkerDependencyResolver{
				ArtifactTiming: tc.timing,
				LoadConfig:     func(string) (*config.Config, error) { return cfg, nil },
				BuildArtifactStore: func(*config.Config, imageAgentArtifactTiming) (imageagenttemporal.DurableArtifactStore, error) {
					storeBuilds++
					return workerArtifactStore{}, nil
				},
				OpenDB:  func(*config.DatabaseConfig) (*gorm.DB, error) { databaseOpens++; return &gorm.DB{}, nil },
				CloseDB: func(*config.DatabaseConfig, *gorm.DB) error { return nil },
				BuildAI: func(*config.Config, *gorm.DB) (*openaiclient.Manager, openaiclient.ClientConfigResolver, aicapability.InvocationRecorder, error) {
					return nil, nil, nil, errors.New("test must stop before AI composition")
				},
			})
			require.ErrorContains(t, err, "operation timeout")
			require.Zero(t, storeBuilds)
			require.Zero(t, databaseOpens)
		})
	}
}

func TestBuildImageAgentDurableArtifactStoreUsesConfiguredS3ClientPath(t *testing.T) {
	cfg := &config.Config{ProductImage: config.ProductImageConfig{Publisher: durablePublisherConfig("aws", true)}}
	artifactStore, err := buildImageAgentDurableArtifactStore(cfg, defaultImageAgentArtifactTiming)
	require.NoError(t, err)
	require.NotNil(t, artifactStore)
}

func durablePublisherConfig(mode string, cosPolicy bool) config.ProductImagePublisherConfig {
	return config.ProductImagePublisherConfig{
		Enabled: true, Provider: "s3", PublicBase: "https://cdn.example.test/images",
		S3: config.ProductImagePublisherS3Config{Bucket: "image-assets", Region: "ap-southeast-1", Endpoint: "https://s3.example.test", AccessKeyID: "test-access", SecretAccessKey: "test-secret", ArtifactMode: mode, COSImmutableNonVersionedBucketPolicy: cosPolicy},
	}
}

type workerArtifactStore struct{}

func (workerArtifactStore) PublicURL(key string) string { return "https://cdn.example.test/" + key }

func (workerArtifactStore) PrepareSlotArtifacts(objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	return objectstore.PreparedSlotArtifacts{}, nil
}
func (workerArtifactStore) PreserveSlotArtifacts(context.Context, imageagent.SlotExternalEffectIdentity, objectstore.PreparedSlotArtifacts) error {
	return nil
}
func (workerArtifactStore) RecoverSlotArtifacts(_ context.Context, _ imageagent.SlotExternalEffectIdentity, expected imageagent.StagingManifest) (objectstore.PreparedSlotArtifacts, error) {
	return objectstore.PreparedSlotArtifacts{Manifest: expected}, nil
}
func (workerArtifactStore) EnsureStaged(context.Context, objectstore.PreparedSlotArtifacts) error {
	return nil
}
func (workerArtifactStore) Finalize(context.Context, imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return imageagent.FinalManifest{}, nil
}
func (workerArtifactStore) FinalizeWithProgress(context.Context, imageagent.StagingManifest, func(context.Context, int) error) (imageagent.FinalManifest, error) {
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
