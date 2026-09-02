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
	input, err := newListingKitRuntimeBuildInput(logger, deps, deps.ensureListingKitSupport().repositories)
	if err != nil {
		return nil, err
	}
	listingKitModule, err := b.buildListingKit(input)
	if err != nil {
		return nil, err
	}
	deps.attachListingKitModule(listingKitModule)
	return listingKitModule, nil
}

func newListingKitRuntimeBuildInput(logger *logrus.Logger, deps *runtimeDeps, repositories listingkithttpapi.BuildServiceRepositories) (listingkithttpapi.RuntimeBuildInput, error) {
	approvedAssets := ensureApprovedAssetReader(logger, deps)
	cookieStore, err := ensureListingKitSheinCookieStore(logger, deps)
	if err != nil {
		return listingkithttpapi.RuntimeBuildInput{}, err
	}
	support := listingkithttpapi.BuildRuntimeSupport(listingkithttpapi.RuntimeSupportInput{
		Repositories:              repositories,
		SheinCookieStore:          cookieStore,
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
			Support:                            support,
			ShouldStartTemporalWorkerInProcess: appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess(),
		},
	}, nil
}
