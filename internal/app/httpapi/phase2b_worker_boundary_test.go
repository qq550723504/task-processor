package httpapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPAPITypesDoesNotOwnNamedWorkerPoolMap(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("types.go")
	require.NoError(t, err)

	content := string(src)
	require.NotContains(t, content, "func (c httpFeatureComposition) namedWorkerPools(")
	require.NotContains(t, content, "func (c httpFeatureComposition) workerPools(")
}

func TestHTTPAPITypesDoesNotOwnFeatureCompositionMethods(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)

	for _, marker := range []string{
		"func (c httpFeatureComposition) runtimeModules(",
		"func (c httpFeatureComposition) routeModules(",
		"func (c httpFeatureComposition) buildRuntimeBundle(",
		"func (c httpFeatureComposition) buildServerBundle(",
		"func (c httpFeatureComposition) productHandler(",
		"func (c httpFeatureComposition) imageHandler(",
		"func (c httpFeatureComposition) amazonListingHandler(",
		"func (c httpFeatureComposition) listingKitHandler(",
		"func (c httpFeatureComposition) studioSessionHandler(",
		"func (c httpFeatureComposition) taskRPCHTTPModule(",
	} {
		require.NotContains(t, string(typesSrc), marker)
	}

	compositionSrc, err := os.ReadFile("composition_modules.go")
	require.NoError(t, err)
	for _, marker := range []string{
		"func (c httpFeatureComposition) runtimeModules(",
		"func (c httpFeatureComposition) routeModules(",
		"func (c httpFeatureComposition) buildRuntimeBundle(",
		"func (c httpFeatureComposition) buildServerBundle(",
		"func (c httpFeatureComposition) taskRPCHTTPModule(",
	} {
		require.Contains(t, string(compositionSrc), marker)
	}

	routeTypesSrc, err := os.ReadFile("route_handler_types.go")
	require.NoError(t, err)
	for _, marker := range []string{
		"func (c httpFeatureComposition) productHandler(",
		"func (c httpFeatureComposition) imageHandler(",
		"func (c httpFeatureComposition) amazonListingHandler(",
		"func (c httpFeatureComposition) listingKitHandler(",
		"func (c httpFeatureComposition) studioSessionHandler(",
	} {
		require.Contains(t, string(routeTypesSrc), marker)
	}
}

func TestHTTPAPITypesDoesNotOwnRouteHandlerContracts(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)

	for _, marker := range []string{
		"type productRouteHandler =",
		"type imageRouteHandler =",
		"type amazonListingRouteHandler =",
		"type listingKitRouteHandler =",
		"type studioSessionRouteHandler =",
		"type taskRPCRouteHandler =",
		"type promptTemplateRouteHandler =",
		"type sdsCatalogRouteHandler =",
		"type sdsLoginRouteHandler =",
		"type sheinLoginRouteHandler interface",
		"type httpModuleHandlers struct",
	} {
		require.NotContains(t, string(typesSrc), marker)
	}

	routeTypesSrc, err := os.ReadFile("route_handler_types.go")
	require.NoError(t, err)
	for _, marker := range []string{
		"type productRouteHandler =",
		"type imageRouteHandler =",
		"type amazonListingRouteHandler =",
		"type listingKitRouteHandler =",
		"type studioSessionRouteHandler =",
		"type taskRPCRouteHandler =",
		"type promptTemplateRouteHandler =",
		"type sdsCatalogRouteHandler =",
		"type sdsLoginRouteHandler =",
		"type sheinLoginRouteHandler interface",
		"type httpModuleHandlers struct",
	} {
		require.NotContains(t, string(routeTypesSrc), marker)
	}
	require.NotContains(t, string(routeTypesSrc), "type routeDescriptor =")
}

func TestHTTPAPITypesDoesNotOwnAppBootstrapState(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)
	typesContent := string(typesSrc)

	for _, marker := range []string{
		"type appBootstrap struct",
		"server         *http.Server",
		"routes         []routeDescriptor",
		"routes         []httproute.Descriptor",
		"pools          []worker.WorkerPool",
		"closers        []func() error",
	} {
		require.NotContains(t, typesContent, marker)
	}

	bootstrapTypesSrc, err := os.ReadFile("bootstrap_types.go")
	require.NoError(t, err)
	bootstrapTypesContent := string(bootstrapTypesSrc)
	for _, marker := range []string{
		"type appBootstrap struct",
		"server         *http.Server",
		"routes         []httproute.Descriptor",
		"pools          []worker.WorkerPool",
		"closers        []func() error",
	} {
		require.Contains(t, bootstrapTypesContent, marker)
	}
}

func TestHTTPAPITypesDoesNotOwnRunOptions(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)
	typesContent := string(typesSrc)

	for _, marker := range []string{
		"type Options struct",
		"ConfigPath     string",
		"Port           int",
		"ShutdownSignal chan os.Signal",
	} {
		require.NotContains(t, typesContent, marker)
	}

	optionsSrc, err := os.ReadFile("options.go")
	require.NoError(t, err)
	optionsContent := string(optionsSrc)
	for _, marker := range []string{
		"type Options struct",
		"ConfigPath     string",
		"Port           int",
		"ShutdownSignal chan os.Signal",
	} {
		require.Contains(t, optionsContent, marker)
	}
}

func TestLegacyBuildHandlersFacadeStaysRetired(t *testing.T) {
	t.Parallel()

	_, err := os.Stat("handlers_legacy.go")
	require.Truef(t, os.IsNotExist(err), "handlers_legacy.go still exists; use module runtime bootstrap instead of the legacy handler facade")
}

func TestHTTPAPIAppDoesNotOwnModuleRuntimeHelpers(t *testing.T) {
	t.Parallel()

	appSrc, err := os.ReadFile("app.go")
	require.NoError(t, err)
	appContent := string(appSrc)

	for _, marker := range []string{
		"func buildHTTPServerBundleFromModules(",
		"buildRuntimeBundleFromModules(cfg, modules)",
		"bundle.buildServerBundle(port)",
	} {
		require.NotContains(t, appContent, marker)
	}

	moduleRuntimeSrc, err := os.ReadFile("module_runtime.go")
	require.NoError(t, err)
	moduleRuntimeContent := string(moduleRuntimeSrc)
	for _, marker := range []string{
		"func buildHTTPServerBundleFromModules(",
		"func buildRegisteredRoutesForModules(",
		"buildRuntimeBundleFromModules(cfg, modules)",
		"bundle.buildServerBundle(port)",
	} {
		require.Contains(t, moduleRuntimeContent, marker)
	}
}

func TestHTTPAPIServerDoesNotOwnListingKitAuthMiddlewareSelection(t *testing.T) {
	t.Parallel()

	serverSrc, err := os.ReadFile("server.go")
	require.NoError(t, err)
	serverContent := string(serverSrc)

	for _, marker := range []string{
		`"task-processor/internal/listingkit/httpapi"`,
		"NewZitadelAuthMiddlewareFromEnv(",
		"RouteRequiresZitadelAuth(",
		"NewRouteRoleMiddleware(",
	} {
		require.NotContains(t, serverContent, marker)
	}

	authSrc, err := os.ReadFile("server_auth.go")
	require.NoError(t, err)
	authContent := string(authSrc)
	for _, marker := range []string{
		`"task-processor/internal/listingkit/httpapi"`,
		"NewZitadelAuthMiddlewareFromEnv(",
		"RouteRequiresZitadelAuth(",
		"NewRouteRoleMiddleware(",
	} {
		require.Contains(t, authContent, marker)
	}
}

func TestHTTPAPIModulesFileDoesNotOwnWorkerRuntimeSupport(t *testing.T) {
	t.Parallel()

	modulesContent := readRetiredModulesFileIfPresent(t)

	for _, marker := range []string{
		"func newWorkerPool(",
		"func buildLocalTaskHealthProvider(",
		"worker.NewPoolWithConfig(",
		"TaskTimeout:",
		"GetQueueStats()",
		"GetMetrics()",
	} {
		require.NotContains(t, modulesContent, marker)
	}

	workerSupportSrc, err := os.ReadFile("runtime_worker_pools.go")
	require.NoError(t, err)
	workerSupportContent := string(workerSupportSrc)
	for _, marker := range []string{
		"func newWorkerPool(",
		"func buildLocalTaskHealthProvider(",
		"worker.NewPoolWithConfig(",
		"TaskTimeout:",
		"GetQueueStats()",
		"GetMetrics()",
	} {
		require.Contains(t, workerSupportContent, marker)
	}
}

func TestHTTPAPIModulesFileDoesNotOwnLoginRuntimeSupport(t *testing.T) {
	t.Parallel()

	modulesContent := readRetiredModulesFileIfPresent(t)

	for _, marker := range []string{
		`"task-processor/internal/shein/client"`,
		`"task-processor/internal/sheinlogin/bootstrap"`,
		`"task-processor/internal/sdslogin/bootstrap"`,
		"sheinclient.ConfigureLoginAccountFromConfig(",
		"func buildSheinLoginModuleResult(",
		"func buildSDSLoginModuleResult(",
		"sheinloginbootstrap.BuildHandler(",
		"sdsloginbootstrap.BuildHandler(",
	} {
		require.NotContains(t, modulesContent, marker)
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		`"task-processor/internal/shein/client"`,
		`"task-processor/internal/sheinlogin/bootstrap"`,
		`"task-processor/internal/sdslogin/bootstrap"`,
		"func configureSheinLoginAccount(",
		"func buildSheinLoginModuleResult(",
		"func buildSDSLoginModuleResult(",
		"sheinclient.ConfigureLoginAccountFromConfig(",
		"sheinloginbootstrap.BuildHandler(",
		"sdsloginbootstrap.BuildHandler(",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnLoginBootstrapTypes(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		`"task-processor/internal/sheinlogin/bootstrap"`,
		`"task-processor/internal/sdslogin/bootstrap"`,
		"*sheinloginbootstrap.BuildResult",
		"*sdsloginbootstrap.BuildResult",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		"type sheinLoginModuleBuilder func(",
		"type sdsLoginModuleBuilder func(",
		"*sheinloginbootstrap.BuildResult",
		"*sdsloginbootstrap.BuildResult",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
	for _, marker := range []string{
		"type sheinLoginModuleResult = sheinloginbootstrap.BuildResult",
		"type sdsLoginModuleResult = sdsloginbootstrap.BuildResult",
		"*sheinLoginModuleResult",
		"*sdsLoginModuleResult",
	} {
		require.NotContains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnLoginFeatureAssembly(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		"deps.addClosers(sheinLoginCloser)",
		"deps.addClosers(sdsLoginCloser)",
		"deps.attachSDSLoginResult(",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		"type loginFeatureBuilder struct",
		"type loginFeatureSet struct",
		"deps.addClosers(sheinLoginCloser)",
		"deps.addClosers(sdsLoginCloser)",
		"deps.attachSDSLoginResult(",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPIRuntimeStateUsesOwningLoginBootstrapResultTypes(t *testing.T) {
	t.Parallel()

	expectedByFile := map[string][]string{
		"types.go": {
			`"task-processor/internal/sheinlogin/bootstrap"`,
			`"task-processor/internal/sdslogin/bootstrap"`,
			"*sheinloginbootstrap.BuildResult",
			"*sdsloginbootstrap.BuildResult",
		},
		"runtime_deps_methods.go": {
			`"task-processor/internal/sdslogin/bootstrap"`,
			"*sdsloginbootstrap.BuildResult",
		},
	}
	for name, expectedMarkers := range expectedByFile {
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		content := string(src)

		for _, marker := range expectedMarkers {
			require.Contains(t, content, marker)
		}
		for _, marker := range []string{
			"*sheinLoginModuleResult",
			"*sdsLoginModuleResult",
		} {
			require.NotContains(t, content, marker)
		}
	}

	loginSupportSrc, err := os.ReadFile("runtime_login_modules.go")
	require.NoError(t, err)
	loginSupportContent := string(loginSupportSrc)
	for _, marker := range []string{
		"type sheinLoginModuleBuilder func(",
		"type sdsLoginModuleBuilder func(",
		"*sheinloginbootstrap.BuildResult",
		"*sdsloginbootstrap.BuildResult",
	} {
		require.Contains(t, loginSupportContent, marker)
	}
	for _, marker := range []string{
		"type sheinLoginModuleResult = sheinloginbootstrap.BuildResult",
		"type sdsLoginModuleResult = sdsloginbootstrap.BuildResult",
		"*sheinLoginModuleResult",
		"*sdsLoginModuleResult",
	} {
		require.NotContains(t, loginSupportContent, marker)
	}
}

func TestHTTPAPIRuntimeStateUsesOwningSupportModuleResultTypes(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)
	typesContent := string(typesSrc)

	for _, marker := range []string{
		`"task-processor/internal/promptmgmt/api"`,
		`"task-processor/internal/sds/httpapi"`,
		`"task-processor/internal/taskrpcapi"`,
		"*promptmgmtapi.BuildResult",
		"*sdshttpapi.BuildResult",
		"*taskrpcapi.BuildResult",
	} {
		require.Contains(t, typesContent, marker)
	}
	for _, marker := range []string{
		"*promptModuleResult",
		"*sdsModuleResult",
		"*taskRPCModuleResult",
	} {
		require.NotContains(t, typesContent, marker)
	}

	resultSrc, err := os.ReadFile("runtime_module_results.go")
	require.NoError(t, err)
	resultContent := string(resultSrc)
	for _, marker := range []string{
		"type promptModuleResult = promptmgmtapi.BuildResult",
		"type sdsModuleResult = sdshttpapi.BuildResult",
		"type taskRPCModuleResult = taskrpcapi.BuildResult",
	} {
		require.NotContains(t, resultContent, marker)
	}
}

func TestHTTPAPIRuntimeStateUsesOwningFeatureHTTPAPIModuleTypes(t *testing.T) {
	t.Parallel()

	typesSrc, err := os.ReadFile("types.go")
	require.NoError(t, err)
	typesContent := string(typesSrc)

	for _, marker := range []string{
		`"task-processor/internal/amazonlisting/httpapi"`,
		`"task-processor/internal/listingkit/httpapi"`,
		`"task-processor/internal/productenrich/httpapi"`,
		`"task-processor/internal/productimage/httpapi"`,
		"*amazonlistinghttpapi.Module",
		"*listingkithttpapi.Module",
		"*productenrichhttpapi.Module",
		"*productimagehttpapi.Module",
	} {
		require.Contains(t, typesContent, marker)
	}
	for _, marker := range []string{
		"*productModuleResult",
		"*imageModuleResult",
		"*amazonListingModuleResult",
		"*listingKitModuleResult",
		"productimagehttpapi.RouteHandler",
	} {
		require.NotContains(t, typesContent, marker)
	}

	contractSrc, err := os.ReadFile("feature_module_builders.go")
	require.NoError(t, err)
	contractContent := string(contractSrc)
	for _, marker := range []string{
		"type productModuleResult = productenrichhttpapi.Module",
		"type imageModuleResult = productimagehttpapi.Module",
		"type amazonListingModuleResult = amazonlistinghttpapi.Module",
		"type listingKitModuleResult = listingkithttpapi.Module",
	} {
		require.NotContains(t, contractContent, marker)
	}
}

func TestHTTPAPIRuntimeDepsMethodsUseOwningFeatureHTTPAPIModuleTypes(t *testing.T) {
	t.Parallel()

	methodsSrc, err := os.ReadFile("runtime_deps_methods.go")
	require.NoError(t, err)
	methodsContent := string(methodsSrc)

	for _, marker := range []string{
		`"task-processor/internal/amazonlisting/httpapi"`,
		`"task-processor/internal/listingkit/httpapi"`,
		`"task-processor/internal/productenrich/httpapi"`,
		`"task-processor/internal/productimage/httpapi"`,
		"*amazonlistinghttpapi.Module",
		"*listingkithttpapi.Module",
		"*productenrichhttpapi.Module",
		"*productimagehttpapi.Module",
	} {
		require.Contains(t, methodsContent, marker)
	}
	for _, marker := range []string{
		"*productModuleResult",
		"*imageModuleResult",
		"*amazonListingModuleResult",
		"*listingKitModuleResult",
	} {
		require.NotContains(t, methodsContent, marker)
	}

	contractSrc, err := os.ReadFile("feature_module_builders.go")
	require.NoError(t, err)
	contractContent := string(contractSrc)
	for _, marker := range []string{
		"type productModuleResult = productenrichhttpapi.Module",
		"type imageModuleResult = productimagehttpapi.Module",
		"type amazonListingModuleResult = amazonlistinghttpapi.Module",
		"type listingKitModuleResult = listingkithttpapi.Module",
	} {
		require.NotContains(t, contractContent, marker)
	}
}

func TestHTTPModulesDoNotOwnFeatureHTTPAPIModuleSelectionSignatures(t *testing.T) {
	t.Parallel()

	modulesSrc, err := os.ReadFile("http_modules.go")
	require.NoError(t, err)
	modulesContent := string(modulesSrc)

	for _, marker := range []string{
		"built *productenrichhttpapi.Module",
		"imageBuilt *productimagehttpapi.Module",
		"built *amazonlistinghttpapi.Module",
		"built *listingkithttpapi.Module",
		"built *productModuleResult",
		"imageBuilt *imageModuleResult",
		"built *amazonListingModuleResult",
		"built *listingKitModuleResult",
	} {
		require.NotContains(t, modulesContent, marker)
	}
}

func TestHTTPAPIFeatureBuildersUseOwningFeatureHTTPAPIModuleTypesInSignatures(t *testing.T) {
	t.Parallel()

	expectedModules := map[string][]string{
		"feature_builder_listingkit.go": {
			"*productenrichhttpapi.Module",
			"*productimagehttpapi.Module",
			"*listingkithttpapi.Module",
		},
		"feature_builder_amazonlisting.go": {
			"*amazonlistinghttpapi.Module",
		},
	}
	for name, moduleMarkers := range expectedModules {
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		content := string(src)

		for _, marker := range moduleMarkers {
			require.Contains(t, content, marker)
		}
		for _, marker := range []string{
			"*productModuleResult",
			"*imageModuleResult",
			"*amazonListingModuleResult",
			"*listingKitModuleResult",
		} {
			require.NotContains(t, content, marker)
		}
	}

	listingKitFeatureSrc, err := os.ReadFile("feature_builder_listingkit.go")
	require.NoError(t, err)
	listingKitFeatureContent := string(listingKitFeatureSrc)
	for _, marker := range []string{
		"productModule    *productenrichhttpapi.Module",
		"imageModule      *productimagehttpapi.Module",
		"listingKitModule *listingkithttpapi.Module",
		"buildProduct    productModuleBuilder",
		"buildImage      imageModuleBuilder",
		"buildListingKit listingKitModuleBuilder",
	} {
		require.Contains(t, listingKitFeatureContent, marker)
	}

	amazonListingFeatureSrc, err := os.ReadFile("feature_builder_amazonlisting.go")
	require.NoError(t, err)
	amazonListingFeatureContent := string(amazonListingFeatureSrc)
	for _, marker := range []string{
		"buildAmazonListing amazonListingModuleBuilder",
		"func (b amazonListingFeatureBuilder) build(logger *logrus.Logger, deps *runtimeDeps) (*amazonlistinghttpapi.Module, error)",
	} {
		require.Contains(t, amazonListingFeatureContent, marker)
	}
}

func TestFeatureModuleBuilderContractsUseOwningModuleTypes(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("feature_module_builders.go")
	require.NoError(t, err)
	content := string(src)

	for _, marker := range []string{
		"(*productenrichhttpapi.Module, error)",
		"(*productimagehttpapi.Module, error)",
		"(*amazonlistinghttpapi.Module, error)",
		"(*listingkithttpapi.Module, error)",
	} {
		require.Contains(t, content, marker)
	}

	for _, marker := range []string{
		"type productModuleBuilder func(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error)",
		"type imageModuleBuilder func(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error)",
		"type amazonListingModuleBuilder func(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)",
		"type listingKitModuleBuilder func(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error)",
		"func buildProductModuleResult(input productenrichhttpapi.RuntimeBuildInput) (*productenrichhttpapi.Module, error)",
		"func buildImageModuleResult(input productimagehttpapi.RuntimeBuildInput) (*productimagehttpapi.Module, error)",
		"func buildAmazonListingModuleResult(input amazonlistinghttpapi.RuntimeBuildInput) (*amazonlistinghttpapi.Module, error)",
		"func buildListingKitModuleResult(input listingkithttpapi.RuntimeBuildInput) (*listingkithttpapi.Module, error)",
	} {
		require.Contains(t, content, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnSupportModuleBuilderContracts(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		`"task-processor/internal/core/config"`,
		`"task-processor/internal/prompt"`,
		`"task-processor/internal/promptmgmt/api"`,
		`"task-processor/internal/sds/httpapi"`,
		`"task-processor/internal/taskrpcapi"`,
		"*promptmgmtapi.BuildResult",
		"*sdshttpapi.BuildResult",
		"*taskrpcapi.BuildResult",
		"taskrpcapi.ClientProvider",
		"taskrpcapi.LocalStatusProvider",
		"prompt.TenantPromptStore",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	resultSrc, err := os.ReadFile("runtime_module_results.go")
	require.NoError(t, err)
	resultContent := string(resultSrc)
	for _, marker := range []string{
		"type promptModuleBuilder func(",
		"type sdsModuleBuilder func(",
		"type taskRPCModuleBuilder func(",
		"func buildPromptModuleResult(",
		"func buildSDSModuleResult(",
		"func buildTaskRPCModuleResult(",
		"*promptmgmtapi.BuildResult",
		"*sdshttpapi.BuildResult",
		"*taskrpcapi.BuildResult",
	} {
		require.Contains(t, resultContent, marker)
	}
	for _, marker := range []string{
		"*promptModuleResult",
		"*sdsModuleResult",
		"*taskRPCModuleResult",
	} {
		require.NotContains(t, resultContent, marker)
	}
}

func TestHTTPAPICompositionBuilderDoesNotOwnSupportFeatureAssembly(t *testing.T) {
	t.Parallel()

	compositionSrc, err := os.ReadFile("composition_builder.go")
	require.NoError(t, err)
	compositionContent := string(compositionSrc)

	for _, marker := range []string{
		"deps.shared.tenantPromptStore",
		"buildRuntimeBundleFromModules(",
		"runtimeBundle.localTaskHealthProvider()",
		"composition.promptModule = b.buildPrompt(",
		"composition.taskRPCResult = taskRPCResult",
		"composition.sdsModule = b.buildSDS(",
	} {
		require.NotContains(t, compositionContent, marker)
	}

	resultSrc, err := os.ReadFile("runtime_module_results.go")
	require.NoError(t, err)
	resultContent := string(resultSrc)
	for _, marker := range []string{
		"type supportFeatureBuilder struct",
		"type supportFeatureSet struct",
		"deps.shared.tenantPromptStore",
		"buildRuntimeBundleFromModules(",
		"runtimeBundle.localTaskHealthProvider()",
	} {
		require.Contains(t, resultContent, marker)
	}
}
