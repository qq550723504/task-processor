# Task Processor Framework Phase 2B Worker Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the kernel module registration path from HTTP routes to worker pools, so `internal/app/httpapi` no longer hand-maintains pool lists or local task-health maps for `product`, `image`, `amazon listing`, and `listingkit`.

**Architecture:** Reuse the existing `kernel/module.Registry` and the feature-owned runtime builders added in `Phase 2A`; do not invent a second composition system just for workers. The kernel registry should become the single runtime bundle collector for route descriptors and named worker pools, while feature packages expose small runtime modules that register both their HTTP routes and their worker pools through the same `Register(...)` path.

**Tech Stack:** Go, existing `internal/kernel/module` registry, existing feature `*httpapi.Module` structs, `worker.WorkerPool`, `internal/app/httpapi` composition tests

**Out of Scope For This Slice:** Temporal workflow host registration, standalone Temporal worker boot, generic task engine registration, and scheduler registration. Those should move into a separate `Phase 2C` plan once worker-pool registration is stable.

---

### Task 1: Extend `kernel/module.Registry` to register named worker pools

**Files:**
- Modify: `internal/kernel/module/registry.go`
- Modify: `internal/kernel/module/registry_test.go`
- Modify: `internal/kernel/module/interfaces.go`

- [ ] **Step 1: Write the failing tests for worker-pool registration**

Add focused tests to `internal/kernel/module/registry_test.go`:

```go
func TestRegistryAddWorkerPoolPreservesOrder(t *testing.T) {
	reg := NewRegistry()
	first := stubWorkerPool{}
	second := stubWorkerPool{}

	err := reg.AddWorkerPool("product_enrich", first)
	require.NoError(t, err)
	err = reg.AddWorkerPool("product_image", second)
	require.NoError(t, err)

	pools := reg.WorkerPools()
	require.Len(t, pools, 2)
	require.Equal(t, "product_enrich", pools[0].Name)
	require.Equal(t, first, pools[0].Pool)
	require.Equal(t, "product_image", pools[1].Name)
	require.Equal(t, second, pools[1].Pool)
}

func TestRegistryAddWorkerPoolRejectsDuplicateNames(t *testing.T) {
	reg := NewRegistry()

	err := reg.AddWorkerPool("product_enrich", stubWorkerPool{})
	require.NoError(t, err)

	err = reg.AddWorkerPool("product_enrich", stubWorkerPool{})
	require.ErrorContains(t, err, "worker pool already registered")
}

func TestRegistryAddWorkerPoolRejectsNilPool(t *testing.T) {
	reg := NewRegistry()

	var pool worker.WorkerPool
	err := reg.AddWorkerPool("product_enrich", pool)
	require.ErrorContains(t, err, "worker pool is nil")
}
```

Add the test stub near the bottom of the file:

```go
type stubWorkerPool struct{}

func (stubWorkerPool) Start(context.Context)                 {}
func (stubWorkerPool) Stop(context.Context)                  {}
func (stubWorkerPool) Submit(worker.WorkerJob) error         { return nil }
func (stubWorkerPool) AvailableSlots() int                   { return 0 }
func (stubWorkerPool) GetQueueStats() worker.QueueStats      { return worker.QueueStats{} }
func (stubWorkerPool) SetJobHandler(worker.JobHandler)       {}
func (stubWorkerPool) GetMetrics() *worker.Metrics           { return nil }
```

- [ ] **Step 2: Run the focused registry tests to verify they fail**

Run:

```powershell
go test ./internal/kernel/module -run "TestRegistryAddWorkerPoolPreservesOrder|TestRegistryAddWorkerPoolRejectsDuplicateNames|TestRegistryAddWorkerPoolRejectsNilPool" -count=1
```

Expected: FAIL because `AddWorkerPool(...)` and `WorkerPools()` do not exist yet.

- [ ] **Step 3: Implement worker-pool registration in the registry**

Update `internal/kernel/module/registry.go`:

```go
type NamedWorkerPool struct {
	Name string
	Pool worker.WorkerPool
}

type Registry struct {
	routes             []httproute.Descriptor
	workerPools        []NamedWorkerPool
	workerPoolNames    map[string]struct{}
	taskHandlers       map[string]TaskHandler
	workflowNames      map[string]struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		workerPoolNames: make(map[string]struct{}),
		taskHandlers:    make(map[string]TaskHandler),
		workflowNames:   make(map[string]struct{}),
	}
}

func (r *Registry) AddWorkerPool(name string, pool worker.WorkerPool) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("worker pool name is empty")
	}
	if isNilInterface(pool) {
		return fmt.Errorf("worker pool is nil: %s", name)
	}
	if _, exists := r.workerPoolNames[name]; exists {
		return fmt.Errorf("worker pool already registered: %s", name)
	}
	r.workerPoolNames[name] = struct{}{}
	r.workerPools = append(r.workerPools, NamedWorkerPool{Name: name, Pool: pool})
	return nil
}

func (r *Registry) WorkerPools() []NamedWorkerPool {
	out := make([]NamedWorkerPool, len(r.workerPools))
	copy(out, r.workerPools)
	return out
}
```

Important:

- reuse `isNilInterface(...)`
- preserve registration order
- do not add a second worker registry type unless this task actually needs it

- [ ] **Step 4: Re-run the full kernel module test package**

Run:

```powershell
go test ./internal/kernel/module -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/kernel/module/interfaces.go internal/kernel/module/registry.go internal/kernel/module/registry_test.go
git commit -m "feat: add worker pool registration to module registry"
```

### Task 2: Add feature-owned runtime modules that register both routes and worker pools

**Files:**
- Create: `internal/productenrich/httpapi/runtime_module.go`
- Create: `internal/productimage/httpapi/runtime_module.go`
- Create: `internal/amazonlisting/httpapi/runtime_module.go`
- Create: `internal/listingkit/httpapi/runtime_module.go`
- Create: `internal/productenrich/httpapi/runtime_module_test.go`
- Create: `internal/amazonlisting/httpapi/runtime_module_test.go`
- Modify: `internal/listingkit/httpapi/http_module_test.go`
- Modify: `internal/app/httpapi/http_modules.go`

- [ ] **Step 1: Write the failing tests for feature-owned runtime modules**

Add focused tests that prove a prebuilt feature module can register its route family and named pool through the same `Register(...)` call.

Create `internal/productenrich/httpapi/runtime_module_test.go`:

```go
func TestRuntimeModuleRegistersRoutesAndWorkerPool(t *testing.T) {
	reg := module.NewRegistry()
	built := &Module{
		Handler: stubProductHandler{},
		Pool:    stubWorkerPool{},
	}

	err := NewRuntimeModule(built).Register(reg)
	require.NoError(t, err)

	routes := reg.Routes()
	require.NotEmpty(t, routes)
	require.Equal(t, "/api/v1/products/generate", routes[0].Path)

	pools := reg.WorkerPools()
	require.Len(t, pools, 1)
	require.Equal(t, "product_enrich", pools[0].Name)
}
```

Create the Amazon and ListingKit equivalents with names:

- `amazon_listing`
- `listing_kit`

For ListingKit, keep the HTTP route assertion on `/api/v1/listing-kits/generate`.

- [ ] **Step 2: Run the focused feature tests to verify they fail**

Run:

```powershell
go test ./internal/productenrich/httpapi ./internal/amazonlisting/httpapi ./internal/listingkit/httpapi -run TestRuntimeModuleRegistersRoutesAndWorkerPool -count=1
```

Expected: FAIL because `NewRuntimeModule(...)` does not exist yet.

- [ ] **Step 3: Implement runtime modules in the feature packages**

Create `internal/productenrich/httpapi/runtime_module.go`:

```go
package httpapi

import (
	module "task-processor/internal/kernel/module"
)

func NewRuntimeModule(built *Module) module.Module {
	return runtimeModule{
		name: "product",
		register: func(reg *module.Registry) error {
			if built == nil {
				return nil
			}
			reg.AddRoutes(AppendProductRouteDescriptors(nil, built.Handler, nil)...)
			if built.Pool != nil {
				if err := reg.AddWorkerPool("product_enrich", built.Pool); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
```

Mirror the pattern for:

- `internal/productimage/httpapi/runtime_module.go`
- `internal/amazonlisting/httpapi/runtime_module.go`
- `internal/listingkit/httpapi/runtime_module.go`

Keep each runtime module tiny and feature-owned. Do **not** centralize worker-pool name strings back into `internal/app/httpapi`.

- [ ] **Step 4: Update app-layer module selection to prefer feature runtime modules**

Update `internal/app/httpapi/http_modules.go` so these functions consume prebuilt modules when available:

```go
func newProductHTTPModule(handlers httpModuleHandlers, built *productenrichhttpapi.Module) kernelmodule.Module {
	if built != nil {
		return productenrichhttpapi.NewRuntimeModule(built)
	}
	return productenrichhttpapi.NewHTTPModule(handlers.product, handlers.image)
}
```

Apply the same pattern for:

- `newAmazonListingHTTPModule(...)`
- `newListingKitHTTPModule(...)`

Keep `prompt`, `taskrpc`, `sds`, `shein login`, and `sds login` unchanged in this task.

- [ ] **Step 5: Re-run package verification**

Run:

```powershell
go test ./internal/productenrich/httpapi ./internal/productimage/httpapi ./internal/amazonlisting/httpapi ./internal/listingkit/httpapi -count=1
go test ./internal/app/httpapi -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/productenrich/httpapi/runtime_module.go internal/productimage/httpapi/runtime_module.go internal/amazonlisting/httpapi/runtime_module.go internal/listingkit/httpapi/runtime_module.go internal/productenrich/httpapi/runtime_module_test.go internal/amazonlisting/httpapi/runtime_module_test.go internal/listingkit/httpapi/http_module_test.go internal/app/httpapi/http_modules.go
git commit -m "feat: add feature-owned runtime modules for worker pools"
```

### Task 3: Build app runtime bundles from registered modules instead of hand-maintained pool lists

**Files:**
- Create: `internal/app/httpapi/runtime_bundle.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/modules.go`
- Modify: `internal/app/httpapi/http_module_test.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`

- [ ] **Step 1: Write the failing runtime-bundle tests**

Add focused tests to `internal/app/httpapi/http_module_test.go`:

```go
func TestBuildRuntimeBundleFromModulesCollectsRoutesAndWorkerPools(t *testing.T) {
	moduleWithPool := httpModule{
		name: "product",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(routeDescriptor{Method: http.MethodGet, Path: "/health"})
			return reg.AddWorkerPool("product_enrich", stubWorkerPool{})
		},
	}

	bundle, err := buildRuntimeBundleFromModules([]kernelmodule.Module{moduleWithPool})
	require.NoError(t, err)
	require.Len(t, bundle.routes, 1)
	require.Len(t, bundle.workerPools, 1)
	require.Equal(t, "product_enrich", bundle.workerPools[0].Name)
}
```

Add a second test that proves `localTaskHealthProvider()` now derives names from the registered worker pools rather than `httpFeatureComposition.namedWorkerPools()`.

- [ ] **Step 2: Run the focused tests to verify they fail**

Run:

```powershell
go test ./internal/app/httpapi -run "TestBuildRuntimeBundleFromModulesCollectsRoutesAndWorkerPools|TestRuntimeBundleBuildsLocalTaskHealthProviderFromRegisteredPools" -count=1
```

Expected: FAIL because `runtimeBundle` and `buildRuntimeBundleFromModules(...)` do not exist yet.

- [ ] **Step 3: Implement the runtime bundle and route/pool assembly**

Create `internal/app/httpapi/runtime_bundle.go`:

```go
type namedWorkerPool struct {
	Name string
	Pool worker.WorkerPool
}

type runtimeBundle struct {
	routeModules []kernelmodule.Module
	routes       []routeDescriptor
	workerPools  []namedWorkerPool
}

func buildRuntimeBundleFromModules(modules []kernelmodule.Module) (runtimeBundle, error) {
	reg := kernelmodule.NewRegistry()
	filtered := make([]kernelmodule.Module, 0, len(modules))
	for _, mod := range modules {
		if mod == nil {
			continue
		}
		if err := mod.Register(reg); err != nil {
			return runtimeBundle{}, fmt.Errorf("register module %s: %w", mod.Name(), err)
		}
		filtered = append(filtered, mod)
	}

	pools := reg.WorkerPools()
	bundlePools := make([]namedWorkerPool, 0, len(pools))
	for _, item := range pools {
		bundlePools = append(bundlePools, namedWorkerPool{Name: item.Name, Pool: item.Pool})
	}

	return runtimeBundle{
		routeModules: filtered,
		routes:       reg.Routes(),
		workerPools:  bundlePools,
	}, nil
}
```

Then refactor `internal/app/httpapi/types.go`:

- delete `workerPools()`
- delete `namedWorkerPools()`
- delete `localTaskHealthProvider()`
- keep `routeModules()` temporarily, or rename it to `httpModules()` if that reads better

Move local task-health generation behind the new runtime bundle:

```go
func (b runtimeBundle) localTaskHealthProvider() taskrpcapi.LocalStatusProvider {
	pools := make(map[string]worker.WorkerPool, len(b.workerPools))
	for _, item := range b.workerPools {
		pools[item.Name] = item.Pool
	}
	return buildLocalTaskHealthProvider(pools)
}
```

- [ ] **Step 4: Rewire `buildBootstrap(...)` and composition builder to consume the runtime bundle**

Update `internal/app/httpapi/composition_builder.go` and `modules.go` so the flow becomes:

1. build `httpFeatureComposition`
2. build `runtimeBundle` from registered modules
3. feed `runtimeBundle.localTaskHealthProvider()` into `taskrpcapi.BuildModule(...)`
4. build the server from `runtimeBundle.routeModules`
5. return worker pools from `runtimeBundle.workerPools`

The important outcome is that the names `product_enrich`, `product_image`, `amazon_listing`, and `listing_kit` are no longer maintained in a hand-built app-layer map.

- [ ] **Step 5: Re-run package verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/httpapi/runtime_bundle.go internal/app/httpapi/types.go internal/app/httpapi/composition_builder.go internal/app/httpapi/modules.go internal/app/httpapi/http_module_test.go internal/app/httpapi/composition_builder_test.go
git commit -m "refactor: build http runtime bundle from registered modules"
```

### Task 4: Lock the worker-runtime boundary and stop `internal/app/httpapi` from regressing

**Files:**
- Create: `internal/app/httpapi/phase2b_worker_boundary_test.go`
- Modify: `internal/app/httpapi/e2e_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_live_test.go`
- Modify: `cmd/product-listing-api/wrappers.go`
- Modify: `cmd/productenrich-api/wrappers.go`

- [ ] **Step 1: Add a boundary test that forbids hard-coded pool-name maps in app layer**

Create `internal/app/httpapi/phase2b_worker_boundary_test.go`:

```go
func TestHTTPAPITypesDoesNotOwnNamedWorkerPoolMap(t *testing.T) {
	src, err := os.ReadFile("types.go")
	require.NoError(t, err)
	require.NotContains(t, string(src), "func (c httpFeatureComposition) namedWorkerPools(")
	require.NotContains(t, string(src), "func (c httpFeatureComposition) workerPools(")
}
```

This is intentionally narrow: the app layer may still expose pools, but it should not own the hard-coded worker-name map anymore.

- [ ] **Step 2: Run the focused boundary test to verify it fails before cleanup**

Run:

```powershell
go test ./internal/app/httpapi -run TestHTTPAPITypesDoesNotOwnNamedWorkerPoolMap -count=1
```

Expected: FAIL until the old helper methods are removed.

- [ ] **Step 3: Update side-entry tests and wrapper commands to consume the new runtime bundle output**

Make sure:

- the existing E2E tests still build pools from the registered-module path
- Historical note: `cmd/product-listing-api/wrappers.go` and `cmd/productenrich-api/wrappers.go` previously consumed `httpapi.BuildHandlers(...)`; that compatibility facade is now retired in favor of the HTTP API module runtime bootstrap.
- no new pool-name wiring leaks back into the commands

- [ ] **Step 4: Run full verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/productenrich/httpapi ./internal/productimage/httpapi ./internal/amazonlisting/httpapi ./internal/listingkit/httpapi -count=1
go test ./cmd/product-listing-api ./cmd/productenrich-api -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/phase2b_worker_boundary_test.go internal/app/httpapi/e2e_test.go internal/app/httpapi/e2e_listingkit_sds_test.go internal/app/httpapi/e2e_listingkit_sds_live_test.go cmd/product-listing-api/wrappers.go cmd/productenrich-api/wrappers.go
git commit -m "test: lock worker runtime registration boundary"
```

## Self-Review

### Spec coverage

- The framework design says `Phase 2` should let modules register runtime contributions beyond HTTP routes. This plan does that for worker pools and explicitly moves pool collection onto the module registry path.
- The `Phase 1` checkpoint recommended extending module registration beyond routes and shrinking app-owned orchestration. This plan covers the worker/runtime half of that recommendation and deliberately leaves Temporal workflow host registration for a separate `Phase 2C` because it is a separate subsystem with different runtime semantics.

### Reuse check

- The plan reuses the current `kernel/module.Registry` instead of introducing a parallel worker registry framework.
- The feature-owned runtime modules reuse the already-landed `BuildRuntimeModule(...)` pattern from `Phase 2A`.
- No custom DI layer, task container, or Temporal abstraction DSL is introduced.

### Placeholder scan

- No `TODO`, `TBD`, or “implement later” placeholders remain.
- Every task includes exact files, concrete code snippets, commands, and commit boundaries.

### Type consistency

- `NamedWorkerPool` is defined in Task 1 before later tasks consume `Registry.WorkerPools()`.
- `NewRuntimeModule(...)` is introduced in Task 2 before `runtimeBundle` relies on registered worker pools in Task 3.
- The app-layer boundary test in Task 4 checks for helpers that Task 3 removes, so the guardrail order is consistent.
