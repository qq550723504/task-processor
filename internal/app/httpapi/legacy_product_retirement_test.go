package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestLegacyProductRoutesAreNotRegisteredByProductionComposition(t *testing.T) {
	composition, cfg := buildPersistentProductionCompositionFixture(t)
	bundle, err := composition.buildRuntimeBundle(cfg)
	if err != nil {
		t.Fatalf("build production runtime bundle: %v", err)
	}
	server, routes := bundle.buildServerBundle(0, appHTTPTestRouteAuthorization)
	if len(routes) == 0 {
		t.Fatal("production composition registered no routes")
	}

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
		server.Handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404", route.method, route.path, response.Code)
		}
	}
}

func TestProductionRuntimeRegistryHasNoLegacyProductModulesOrWorkerPools(t *testing.T) {
	composition, cfg := buildPersistentProductionCompositionFixture(t)
	modules := composition.routeModules()
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		t.Fatalf("build production runtime bundle: %v", err)
	}
	if err := validateNoLegacyProductRuntime(modules, bundle.workerPools); err != nil {
		t.Fatal(err)
	}
	assertModuleRegistered(t, modules, "amazon-listing")
	assertModuleRegistered(t, modules, "listing-kit")
	requireCurrentE2EPool(t, bundle, "amazon_listing")
	requireCurrentE2EPool(t, bundle, "listing_kit")

	mutatedModules := append(append([]kernelmodule.Module(nil), modules...), legacyProductMutationModule{name: "product"})
	if err := validateNoLegacyProductRuntime(mutatedModules, bundle.workerPools); err == nil {
		t.Fatal("legacy module mutation was not detected")
	}
	mutatedPools := append(append([]kernelmodule.NamedWorkerPool(nil), bundle.workerPools...), kernelmodule.NamedWorkerPool{Name: "product_image"})
	if err := validateNoLegacyProductRuntime(modules, mutatedPools); err == nil {
		t.Fatal("legacy worker-pool mutation was not detected")
	}
}

func validateNoLegacyProductRuntime(modules []kernelmodule.Module, pools []kernelmodule.NamedWorkerPool) error {
	for _, module := range modules {
		if module == nil {
			continue
		}
		if module.Name() == "product" || module.Name() == "product-image" {
			return fmt.Errorf("legacy runtime module %q is composed", module.Name())
		}
	}
	for _, pool := range pools {
		if pool.Name == "product_enrich" || pool.Name == "product_image" {
			return fmt.Errorf("legacy worker pool %q is registered", pool.Name)
		}
	}
	return nil
}

func assertModuleRegistered(t *testing.T, modules []kernelmodule.Module, name string) {
	t.Helper()
	for _, module := range modules {
		if module != nil && module.Name() == name {
			return
		}
	}
	t.Fatalf("production module %q is not registered", name)
}

type legacyProductMutationModule struct{ name string }

func (m legacyProductMutationModule) Name() string                        { return m.name }
func (legacyProductMutationModule) Enabled(*config.Config) bool           { return true }
func (legacyProductMutationModule) Register(*kernelmodule.Registry) error { return nil }
