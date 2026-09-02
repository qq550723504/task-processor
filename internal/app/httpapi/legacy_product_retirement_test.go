package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

func TestLegacyProductRoutesAreNotRegistered(t *testing.T) {
	deps := &runtimeDeps{shared: &sharedRuntimeDeps{cfg: &config.Config{}}, features: &featureRuntimeState{}}
	bootstrap, err := buildBootstrapWithDependencies(quietTestLogger(), Options{Port: 18080}, bootstrapBuildDependencies{
		buildRuntimeDeps: func(*logrus.Logger, string) (*runtimeDeps, error) { return deps, nil },
		buildComposition: func(*logrus.Logger, *runtimeDeps) (httpFeatureComposition, error) {
			return httpFeatureComposition{}, nil
		},
		buildRuntimeBundle: func(composition httpFeatureComposition, cfg *config.Config) (runtimeBundle, error) {
			return composition.buildRuntimeBundle(cfg)
		},
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
	composition := httpFeatureComposition{}
	modules := composition.routeModules()
	for _, module := range modules {
		if module == nil {
			continue
		}
		if module.Name() == "product" || module.Name() == "product-image" {
			t.Errorf("legacy runtime module %q is still composed", module.Name())
		}
	}

	bundle, err := buildRuntimeBundleFromModules(&config.Config{}, modules)
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
