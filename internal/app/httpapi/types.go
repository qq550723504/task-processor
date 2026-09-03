package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	a1688httpapi "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi"
	imageagenthttpapi "task-processor/internal/imageagent/httpapi"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	localagenthttpapi "task-processor/internal/localagent/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdsadapter "task-processor/internal/sds/adapter"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	"task-processor/internal/sheinlogin"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
	"task-processor/internal/taskrpcapi"
)

type runtimeDeps struct {
	shared              *sharedRuntimeDeps
	features            *featureRuntimeState
	constructionClosers []func() error
	featureFlagsCloser  func() error
	traceCloser         func() error
}

type featureRuntimeState struct {
	productSnapshotReader  listingkit.ProductSnapshotReader
	sdsLoginStatusProvider listingkit.SDSLoginStatusProvider
	listingKitSupport      *listingKitSupport
}

type listingKitSupport struct {
	sdsBaselineRemoteProvider listingkit.SDSBaselineRemoteProvider
	sheinCookieStore          *sheinlogin.RedisStore
	approvedAssetReader       sdsadapter.ApprovedAssetReader
	repositories              listingkithttpapi.BuildServiceRepositories
}

type httpFeatureComposition struct {
	amazonListingModule       *amazonlistinghttpapi.Module
	listingKitModule          *listingkithttpapi.Module
	productSourcingModule     *a1688httpapi.BuildResult
	promptModule              *promptmgmtapi.BuildResult
	sdsModule                 *sdshttpapi.BuildResult
	taskRPCResult             *taskrpcapi.BuildResult
	sheinLoginResult          *sheinloginbootstrap.BuildResult
	sdsLoginResult            *sdsloginbootstrap.BuildResult
	crawler1688Module         kernelmodule.Module
	localAgentModule          *localagenthttpapi.BuildResult
	imageAgentModule          *imageagenthttpapi.BuildResult
	workbenchContextModule    kernelmodule.Module
	storeCenterModule         kernelmodule.Module
	workbenchAuthDependencies *routeAuthDependencies
}
