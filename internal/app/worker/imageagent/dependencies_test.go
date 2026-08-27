package imageagentworker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
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
	}

	dependencies, closeFn, err := resolveImageAgentTemporalDependencies("config/worker.yaml", nil, resolver)
	require.NoError(t, err)
	require.NotNil(t, dependencies.Repository)
	require.NotNil(t, dependencies.SlotExecutor)
	require.NotNil(t, dependencies.Publisher)
	require.Equal(t, "worker-images", capabilityInput.ImageWorkDir)
	require.NoError(t, closeFn())
	require.Equal(t, 1, closed)
}

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
