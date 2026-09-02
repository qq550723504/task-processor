package httpapi

import (
	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

type listingKitFeatureBuilder struct {
	buildListingKit listingKitModuleBuilder
}

func newListingKitFeatureBuilder() listingKitFeatureBuilder {
	return listingKitFeatureBuilder{
		buildListingKit: buildListingKitModuleResult,
	}
}

func (b listingKitFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (*listingkithttpapi.Module, error) {
	if deps == nil || deps.features == nil || deps.features.productSnapshotReader == nil {
		return nil, nil
	}
	if ensureApprovedAssetReader(logger, deps) == nil {
		return nil, nil
	}
	listingKitModule, err := b.buildListingKit(newListingKitRuntimeBuildInput(logger, deps, deps.ensureListingKitSupport().repositories))
	if err != nil {
		return nil, err
	}
	deps.attachListingKitModule(listingKitModule)
	return listingKitModule, nil
}

func newListingKitRuntimeBuildInput(logger *logrus.Logger, deps *runtimeDeps, repositories listingkithttpapi.BuildServiceRepositories) listingkithttpapi.RuntimeBuildInput {
	approvedAssets := ensureApprovedAssetReader(logger, deps)
	support := listingkithttpapi.BuildRuntimeSupport(listingkithttpapi.RuntimeSupportInput{
		Repositories:              repositories,
		SheinCookieStore:          ensureListingKitSheinCookieStore(logger, deps),
		ApprovedAssets:            approvedAssets,
		SDSSyncService:            buildSDSSyncService(logger, deps),
		SDSLoginStatusProvider:    deps.features.sdsLoginStatusProvider,
		SDSBaselineRemoteProvider: buildSDSBaselineRemoteProvider(logger, deps),
	})
	return listingkithttpapi.RuntimeBuildInput{
		Logger: logger,
		Runtime: listingkithttpapi.RuntimeDependencies{
			Config:                             deps.shared.cfg,
			ProductSnapshotReader:              deps.features.productSnapshotReader,
			AICredentialStore:                  deps.shared.aiCredentialStore,
			AIInvocationRecorder:               deps.shared.aiInvocationRecorder,
			AIAsyncJobStore:                    deps.shared.aiAsyncJobStore,
			Support:                            support,
			ShouldStartTemporalWorkerInProcess: appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess(),
		},
	}
}
