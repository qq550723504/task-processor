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
	Repositories                       BuildServiceRepositories
	Hooks                              BuildServiceHooks
	ShouldStartTemporalWorkerInProcess bool
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		ServiceInput: BuildServiceInput{
			Config:                     input.Runtime.Config,
			Logger:                     input.Logger,
			ProductService:             input.Runtime.ProductService,
			ImageService:               input.Runtime.ImageService,
			SDSSyncService:             input.Runtime.SDSSyncService,
			SDSLoginStatusProvider:     input.Runtime.SDSLoginStatusProvider,
			SDSBaselineRemoteProvider:  input.Runtime.SDSBaselineRemoteProvider,
			ImageSubjectExtractor:      input.Runtime.ImageSubjectExtractor,
			ImageWhiteBackgroundRender: input.Runtime.ImageWhiteBackgroundRender,
			ImageSceneRenderer:         input.Runtime.ImageSceneRenderer,
			AICredentialStore:          input.Runtime.AICredentialStore,
			Repositories:               input.Runtime.Repositories,
			Hooks:                      input.Runtime.Hooks,
		},
		ShouldStartTemporalWorkerInProcess: input.Runtime.ShouldStartTemporalWorkerInProcess,
	})
}
