package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sdsusecase "task-processor/internal/sds/usecase"
)

type RuntimeBuildInput struct {
	Logger  *logrus.Logger
	Runtime RuntimeDependencies
}

type RuntimeDependencies struct {
	Config                             *config.Config
	ProductService                     productenrich.ProductService
	ImageService                       productimage.Service
	SDSSyncService                     sdsusecase.Service
	SDSLoginStatusProvider             listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider          listingkit.SDSBaselineRemoteProvider
	ImageSubjectExtractor              productimage.SubjectExtractor
	ImageWhiteBackgroundRender         productimage.WhiteBackgroundRenderer
	ImageSceneRenderer                 productimage.SceneRenderer
	AICredentialStore                  aiCredentialStore
	Support                            RuntimeSupport
	Repositories                       BuildServiceRepositories
	Hooks                              BuildServiceHooks
	ShouldStartTemporalWorkerInProcess bool
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		ServiceInput:                       buildRuntimeServiceInput(input.Logger, input.Runtime),
		ShouldStartTemporalWorkerInProcess: input.Runtime.ShouldStartTemporalWorkerInProcess,
	})
}

func buildRuntimeServiceInput(logger *logrus.Logger, runtime RuntimeDependencies) BuildServiceInput {
	support := resolveRuntimeSupport(runtime)
	return BuildServiceInput{
		Config:                     runtime.Config,
		Logger:                     logger,
		ProductService:             runtime.ProductService,
		ImageService:               runtime.ImageService,
		SDSSyncService:             support.SDSSyncService,
		SDSLoginStatusProvider:     support.SDSLoginStatusProvider,
		SDSBaselineRemoteProvider:  support.SDSBaselineRemoteProvider,
		ImageSubjectExtractor:      runtime.ImageSubjectExtractor,
		ImageWhiteBackgroundRender: runtime.ImageWhiteBackgroundRender,
		ImageSceneRenderer:         runtime.ImageSceneRenderer,
		AICredentialStore:          runtime.AICredentialStore,
		Repositories:               support.Repositories,
		Hooks:                      support.Hooks,
	}
}

func resolveRuntimeSupport(runtime RuntimeDependencies) RuntimeSupport {
	if hasRuntimeSupport(runtime.Support) {
		return runtime.Support
	}
	return RuntimeSupport{
		Repositories:              runtime.Repositories,
		Hooks:                     runtime.Hooks,
		SDSSyncService:            runtime.SDSSyncService,
		SDSLoginStatusProvider:    runtime.SDSLoginStatusProvider,
		SDSBaselineRemoteProvider: runtime.SDSBaselineRemoteProvider,
	}
}

func hasRuntimeSupport(support RuntimeSupport) bool {
	return support.Repositories.Core.Task != nil ||
		support.Hooks.ConfigureAuthorization != nil ||
		support.SDSSyncService != nil ||
		support.SDSLoginStatusProvider != nil ||
		support.SDSBaselineRemoteProvider != nil
}
