package httpapi

import (
	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
)

// Runtime-only support bootstraps, including sheinloginbootstrap.BuildRedisStore,
// now live in runtime_support_listingkit.go to keep this assembly file thin.
func newListingKitRuntimeBuildInput(logger *logrus.Logger, deps *runtimeDeps) listingkithttpapi.RuntimeBuildInput {
	return listingkithttpapi.RuntimeBuildInput{
		Logger: logger,
		Runtime: listingkithttpapi.RuntimeDependencies{
			Config:                     deps.shared.cfg,
			ProductService:             deps.features.productService,
			ImageService:               deps.features.imageService,
			ImageSubjectExtractor:      deps.features.imageSubjectExtractor,
			ImageWhiteBackgroundRender: deps.features.imageWhiteBgRenderer,
			ImageSceneRenderer:         deps.features.imageSceneRenderer,
			AICredentialStore:          deps.shared.aiCredentialStore,
			Support: listingkithttpapi.BuildRuntimeSupport(listingkithttpapi.RuntimeSupportInput{
				SheinCookieStore:          ensureListingKitSheinCookieStore(logger, deps),
				SDSSyncService:            buildSDSSyncService(logger, deps),
				SDSLoginStatusProvider:    deps.features.sdsLoginStatusProvider,
				SDSBaselineRemoteProvider: buildSDSBaselineRemoteProvider(logger, deps),
			}),
			ShouldStartTemporalWorkerInProcess: appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess(),
		},
	}
}
