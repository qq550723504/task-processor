package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
)

type RuntimeBuildInput struct {
	Logger  *logrus.Logger
	Runtime RuntimeDependencies
}

type RuntimeDependencies struct {
	Config                             *config.Config
	ProductSnapshotReader              listingkit.ProductSnapshotReader
	AICredentialStore                  aiCredentialStore
	AIInvocationRecorder               aicapability.InvocationRecorder
	AIAsyncJobStore                    aicapability.AsyncJobBindingStore
	Support                            RuntimeSupport
	ShouldStartTemporalWorkerInProcess bool
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		ServiceInput:                       buildRuntimeServiceInput(input.Logger, input.Runtime),
		ShouldStartTemporalWorkerInProcess: input.Runtime.ShouldStartTemporalWorkerInProcess,
	})
}

func buildRuntimeServiceInput(logger *logrus.Logger, runtime RuntimeDependencies) BuildServiceInput {
	support := runtime.Support
	return BuildServiceInput{
		Config:                    runtime.Config,
		Logger:                    logger,
		ProductSnapshotReader:     runtime.ProductSnapshotReader,
		SDSSyncService:            support.SDSSyncService,
		SDSLoginStatusProvider:    support.SDSLoginStatusProvider,
		SDSBaselineRemoteProvider: support.SDSBaselineRemoteProvider,
		AICredentialStore:         runtime.AICredentialStore,
		AIInvocationRecorder:      runtime.AIInvocationRecorder,
		AIAsyncJobStore:           runtime.AIAsyncJobStore,
		Repositories:              support.Repositories,
		Hooks:                     support.Hooks,
	}
}
