package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLegacyProductRoutesAreNotRegistered(t *testing.T) {
	configureProductImageRuntimePaths(t)
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")

	bootstrap, err := buildBootstrap(quietTestLogger(), Options{
		ConfigPath: "../../../config/config-test.yaml",
		Port:       18080,
	})
	if err != nil {
		t.Fatalf("buildBootstrap() error = %v", err)
	}
	t.Cleanup(func() { closeResources(quietTestLogger(), bootstrap.closers) })

	for _, route := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/products/generate"},
		{method: http.MethodGet, path: "/api/v1/products/tasks/task-1"},
		{method: http.MethodPost, path: "/api/v1/images/process"},
		{method: http.MethodGet, path: "/api/v1/images/tasks/task-1"},
		{method: http.MethodPost, path: "/api/v1/images/tasks/task-1/review"},
	} {
		request := httptest.NewRequest(route.method, route.path, nil)
		response := httptest.NewRecorder()
		bootstrap.server.Handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404", route.method, route.path, response.Code)
		}
	}
}

func TestRuntimeRegistryHasNoLegacyProductModulesOrWorkerPools(t *testing.T) {
	configureProductImageRuntimePaths(t)
	t.Setenv("TASK_PROCESSOR_OPENAI_API_KEY", "sk-test")

	logger := quietTestLogger()
	deps, err := buildRuntimeDeps(logger, "../../../config/config-test.yaml")
	if err != nil {
		t.Fatalf("buildRuntimeDeps() error = %v", err)
	}
	t.Cleanup(func() { cleanupOwnedRuntimeResources(false, deps.constructionClosers) })

	composition, err := newHTTPFeatureCompositionBuilder().build(logger, deps)
	if err != nil {
		t.Fatalf("build composition error = %v", err)
	}
	modules := composition.routeModules()
	for _, module := range modules {
		if module == nil {
			continue
		}
		if module.Name() == "product" || module.Name() == "product-image" {
			t.Errorf("legacy runtime module %q is still composed", module.Name())
		}
	}

	bundle, err := buildRuntimeBundleFromModules(deps.shared.cfg, modules)
	if err != nil {
		t.Fatalf("buildRuntimeBundleFromModules() error = %v", err)
	}
	for _, pool := range bundle.workerPools {
		if pool.Name == "product_enrich" || pool.Name == "product_image" {
			t.Errorf("legacy worker pool %q is still registered", pool.Name)
		}
	}
}

func quietTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	return logger
}
