# Task Processor Framework Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a minimal kernel module registry and migrate the HTTP runtime to assemble routes through registered modules instead of hard-coded global bootstrap wiring.

**Architecture:** Keep current business service construction intact in this phase and change only the registration surface around it. Reuse the existing `internal/platforms/*/module.go` pattern for naming and enablement, add a small `internal/kernel/module` package for stable registration contracts, and adapt `internal/app/httpapi` to collect route descriptors from registered HTTP modules while preserving current route behavior.

**Tech Stack:** Go, Gin, Logrus, existing `httproute.Descriptor`, existing `internal/app/httpapi` bootstrap/server tests

---

### Task 1: Add kernel module registry primitives

**Files:**
- Create: `internal/kernel/module/interfaces.go`
- Create: `internal/kernel/module/registry.go`
- Create: `internal/kernel/module/registry_test.go`

- [ ] **Step 1: Write the failing tests for registry behavior**

Create `internal/kernel/module/registry_test.go` with focused tests for deterministic route registration and duplicate task protection:

```go
package module

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
)

type stubModule struct {
	name    string
	enabled bool
	register func(*Registry) error
}

func (m stubModule) Name() string { return m.name }
func (m stubModule) Enabled(*config.Config) bool { return m.enabled }
func (m stubModule) Register(reg *Registry) error { return m.register(reg) }

func TestRegistryAddRoutesPreservesOrder(t *testing.T) {
	reg := NewRegistry()
	handler := func(c *gin.Context) {}

	reg.AddRoutes(
		httproute.Descriptor{Method: http.MethodGet, Path: "/health", Module: "system", Handler: handler},
		httproute.Descriptor{Method: http.MethodPost, Path: "/jobs", Module: "jobs", Handler: handler},
	)

	routes := reg.Routes()
	require.Len(t, routes, 2)
	require.Equal(t, "/health", routes[0].Path)
	require.Equal(t, "/jobs", routes[1].Path)
}

func TestRegistryRegisterTaskHandlerRejectsDuplicateTaskType(t *testing.T) {
	reg := NewRegistry()

	err := reg.RegisterTaskHandler(stubTaskHandler{name: "product_enrich"})
	require.NoError(t, err)

	err = reg.RegisterTaskHandler(stubTaskHandler{name: "product_enrich"})
	require.ErrorContains(t, err, "task handler already registered")
}

type stubTaskHandler struct{ name string }

func (h stubTaskHandler) TaskType() string { return h.name }
func (h stubTaskHandler) Validate(context.Context, any) error { return nil }
func (h stubTaskHandler) Execute(context.Context, any) (any, error) { return nil, nil }
```

- [ ] **Step 2: Run the new package tests to verify they fail**

Run:

```powershell
go test ./internal/kernel/module -count=1
```

Expected: FAIL with missing `Registry`, `NewRegistry`, and module/task handler interfaces.

- [ ] **Step 3: Implement the minimal module contracts and registry**

Create `internal/kernel/module/interfaces.go`:

```go
package module

import (
	"context"

	"task-processor/internal/core/config"
)

type Module interface {
	Name() string
	Enabled(cfg *config.Config) bool
	Register(reg *Registry) error
}

type TaskHandler interface {
	TaskType() string
	Validate(ctx context.Context, input any) error
	Execute(ctx context.Context, task any) (any, error)
}

type WorkflowHandler interface {
	WorkflowName() string
	RegisterWorkflow(reg *WorkflowRegistry) error
}
```

Create `internal/kernel/module/registry.go`:

```go
package module

import (
	"fmt"

	"task-processor/internal/httproute"
)

type Registry struct {
	routes        []httproute.Descriptor
	taskHandlers  map[string]TaskHandler
	workflowNames map[string]struct{}
}

type WorkflowRegistry struct {
	names map[string]struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		taskHandlers:  make(map[string]TaskHandler),
		workflowNames: make(map[string]struct{}),
	}
}

func (r *Registry) AddRoutes(routes ...httproute.Descriptor) {
	r.routes = append(r.routes, routes...)
}

func (r *Registry) Routes() []httproute.Descriptor {
	out := make([]httproute.Descriptor, len(r.routes))
	copy(out, r.routes)
	return out
}

func (r *Registry) RegisterTaskHandler(handler TaskHandler) error {
	taskType := handler.TaskType()
	if _, exists := r.taskHandlers[taskType]; exists {
		return fmt.Errorf("task handler already registered: %s", taskType)
	}
	r.taskHandlers[taskType] = handler
	return nil
}

func (r *Registry) RegisterWorkflowHandler(handler WorkflowHandler) error {
	name := handler.WorkflowName()
	if _, exists := r.workflowNames[name]; exists {
		return fmt.Errorf("workflow handler already registered: %s", name)
	}
	r.workflowNames[name] = struct{}{}
	return nil
}
```

- [ ] **Step 4: Re-run the kernel registry tests**

Run:

```powershell
go test ./internal/kernel/module -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/kernel/module/interfaces.go internal/kernel/module/registry.go internal/kernel/module/registry_test.go
git commit -m "feat: add kernel module registry primitives"
```

### Task 2: Add HTTP runtime modules that wrap current route families

**Files:**
- Create: `internal/app/httpapi/http_module.go`
- Create: `internal/app/httpapi/http_modules.go`
- Create: `internal/app/httpapi/http_module_test.go`
- Modify: `internal/app/httpapi/types.go`

- [ ] **Step 1: Write the failing tests for registered HTTP modules**

Create `internal/app/httpapi/http_module_test.go` with tests that prove modules register the same route families now built centrally:

```go
package httpapi

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	kernelmodule "task-processor/internal/kernel/module"
)

func TestCoreHTTPModuleRegistersHealthRoute(t *testing.T) {
	reg := kernelmodule.NewRegistry()

	err := newCoreHTTPModule().Register(reg)
	require.NoError(t, err)

	routes := reg.Routes()
	require.Len(t, routes, 1)
	require.Equal(t, http.MethodGet, routes[0].Method)
	require.Equal(t, "/health", routes[0].Path)
	require.Equal(t, "system", routes[0].Module)
}

func TestOpsHTTPModuleRegistersOnlyConfiguredHandlers(t *testing.T) {
	reg := kernelmodule.NewRegistry()
	handler := stubTaskRPCHandler{}

	err := newOpsHTTPModule(httpModuleHandlers{
		taskRPC: handler,
	}).Register(reg)
	require.NoError(t, err)

	routes := reg.Routes()
	require.NotEmpty(t, routes)
	require.Equal(t, "/api/v1/management/tasks/health", routes[0].Path)
}

type stubTaskRPCHandler struct{}

func (stubTaskRPCHandler) GetHealth(c *gin.Context)         {}
func (stubTaskRPCHandler) GetTaskStatus(c *gin.Context)     {}
func (stubTaskRPCHandler) RetryTask(c *gin.Context)         {}
func (stubTaskRPCHandler) CancelTask(c *gin.Context)        {}
func (stubTaskRPCHandler) GetQueueStats(c *gin.Context)     {}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run:

```powershell
go test ./internal/app/httpapi -run "Test(CoreHTTPModuleRegistersHealthRoute|OpsHTTPModuleRegistersOnlyConfiguredHandlers)" -count=1
```

Expected: FAIL with missing `newCoreHTTPModule`, `newOpsHTTPModule`, and `httpModuleHandlers`.

- [ ] **Step 3: Add the HTTP module adapter layer**

Create `internal/app/httpapi/http_module.go`:

```go
package httpapi

import (
	kernelmodule "task-processor/internal/kernel/module"
)

type httpModuleHandlers struct {
	product        productRouteHandler
	image          imageRouteHandler
	amazonListing  amazonListingRouteHandler
	listingKit     listingKitRouteHandler
	promptTemplate promptTemplateRouteHandler
	studioSession  studioSessionRouteHandler
	sheinLogin     sheinLoginRouteHandler
	sdsLogin       sdsLoginRouteHandler
	taskRPC        taskRPCRouteHandler
	sdsCatalog     sdsCatalogRouteHandler
}

type httpModule struct {
	name     string
	register func(reg *kernelmodule.Registry) error
}

func (m httpModule) Name() string { return m.name }
func (m httpModule) Enabled(_ *config.Config) bool { return true }
func (m httpModule) Register(reg *kernelmodule.Registry) error { return m.register(reg) }
```

Create `internal/app/httpapi/http_modules.go`:

```go
package httpapi

import (
	kernelmodule "task-processor/internal/kernel/module"
)

func newCoreHTTPModule() httpModule {
	return httpModule{
		name: "system",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(buildCoreRouteDescriptors()...)
			return nil
		},
	}
}

func newProductHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "product",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(productenrichhttpapi.AppendProductRouteDescriptors(nil, handlers.product, handlers.image)...)
			return nil
		},
	}
}

func newAmazonListingHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "amazon-listing",
		register: func(reg *kernelmodule.Registry) error {
			reg.AddRoutes(amazonlistinghttpapi.AppendRouteDescriptors(nil, handlers.amazonListing)...)
			return nil
		},
	}
}

func newListingKitHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "listingkit",
		register: func(reg *kernelmodule.Registry) error {
			routes := listingkithttpapi.AppendRouteDescriptors(nil, handlers.listingKit)
			routes = listingkithttpapi.AppendPromptTemplateRouteDescriptors(routes, handlers.promptTemplate)
			routes = listingkithttpapi.AppendStudioSessionRouteDescriptors(routes, handlers.studioSession)
			reg.AddRoutes(routes...)
			return nil
		},
	}
}

func newOpsHTTPModule(handlers httpModuleHandlers) httpModule {
	return httpModule{
		name: "ops",
		register: func(reg *kernelmodule.Registry) error {
			routes := appendTaskRPCRouteDescriptors(nil, handlers.taskRPC)
			routes = appendSheinLoginRouteDescriptors(routes, handlers.sheinLogin)
			routes = appendSDSLoginRouteDescriptors(routes, handlers.sdsLogin)
			routes = appendSDSCatalogRouteDescriptors(routes, handlers.sdsCatalog)
			reg.AddRoutes(routes...)
			return nil
		},
	}
}
```

Update `internal/app/httpapi/types.go` imports to include `task-processor/internal/core/config` if `httpModule.Enabled` uses it directly.

- [ ] **Step 4: Re-run the focused HTTP module tests**

Run:

```powershell
go test ./internal/app/httpapi -run "Test(CoreHTTPModuleRegistersHealthRoute|OpsHTTPModuleRegistersOnlyConfiguredHandlers)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/http_module.go internal/app/httpapi/http_modules.go internal/app/httpapi/http_module_test.go internal/app/httpapi/types.go
git commit -m "feat: add registered HTTP module adapters"
```

### Task 3: Build route descriptors from the module registry

**Files:**
- Create: `internal/app/httpapi/module_runtime.go`
- Modify: `internal/app/httpapi/server.go`
- Test: `internal/app/httpapi/server_test.go`

- [ ] **Step 1: Write the failing route-equivalence tests**

Add focused tests to `internal/app/httpapi/server_test.go` that compare old and new route assembly shapes:

```go
func TestBuildRegisteredRoutesMatchesLegacyRouteDescriptors(t *testing.T) {
	handlers := httpModuleHandlers{
		product:        stubProductHandler{},
		image:          stubImageHandler{},
		amazonListing:  stubAmazonListingHandler{},
		listingKit:     stubListingKitHandler{},
		promptTemplate: stubPromptTemplateHandler{},
		studioSession:  stubStudioSessionHandler{},
		taskRPC:        stubTaskRPCHandler{},
	}

	legacy := buildRouteDescriptorsWithShein(
		handlers.product,
		handlers.image,
		handlers.amazonListing,
		handlers.listingKit,
		handlers.promptTemplate,
		handlers.studioSession,
		nil,
		nil,
		handlers.taskRPC,
		nil,
	)

	registered, err := buildRegisteredRoutes(handlers)
	require.NoError(t, err)
	require.Equal(t, routePaths(legacy), routePaths(registered))
}

func routePaths(routes []routeDescriptor) []string {
	out := make([]string, 0, len(routes))
	for _, route := range routes {
		out = append(out, route.Method+" "+route.Path)
	}
	return out
}
```

- [ ] **Step 2: Run the targeted server tests to verify they fail**

Run:

```powershell
go test ./internal/app/httpapi -run TestBuildRegisteredRoutesMatchesLegacyRouteDescriptors -count=1
```

Expected: FAIL with missing `buildRegisteredRoutes`.

- [ ] **Step 3: Implement registry-backed route assembly**

Create `internal/app/httpapi/module_runtime.go`:

```go
package httpapi

import (
	"fmt"

	kernelmodule "task-processor/internal/kernel/module"
)

func buildRegisteredRoutes(handlers httpModuleHandlers) ([]routeDescriptor, error) {
	reg := kernelmodule.NewRegistry()
	modules := []kernelmodule.Module{
		newCoreHTTPModule(),
		newProductHTTPModule(handlers),
		newAmazonListingHTTPModule(handlers),
		newListingKitHTTPModule(handlers),
		newOpsHTTPModule(handlers),
	}

	for _, module := range modules {
		if err := module.Register(reg); err != nil {
			return nil, fmt.Errorf("register module %s: %w", module.Name(), err)
		}
	}

	return reg.Routes(), nil
}
```

Update `internal/app/httpapi/server.go` so `buildRouteDescriptorsWithShein` delegates to the registered route path:

```go
func buildRouteDescriptorsWithShein(
	productHandler productRouteHandler,
	imageHandler imageRouteHandler,
	amazonListingHandler amazonListingRouteHandler,
	listingKitHandler listingKitRouteHandler,
	promptTemplateHandler promptTemplateRouteHandler,
	studioSessionHandler studioSessionRouteHandler,
	sheinLoginHandler sheinLoginRouteHandler,
	sdsLoginHandler sdsLoginRouteHandler,
	taskRPCHandler taskRPCRouteHandler,
	sdsCatalogHandlers ...sdsCatalogRouteHandler,
) []routeDescriptor {
	var sdsCatalogHandler sdsCatalogRouteHandler
	if len(sdsCatalogHandlers) > 0 {
		sdsCatalogHandler = sdsCatalogHandlers[0]
	}

	routes, err := buildRegisteredRoutes(httpModuleHandlers{
		product:        productHandler,
		image:          imageHandler,
		amazonListing:  amazonListingHandler,
		listingKit:     listingKitHandler,
		promptTemplate: promptTemplateHandler,
		studioSession:  studioSessionHandler,
		sheinLogin:     sheinLoginHandler,
		sdsLogin:       sdsLoginHandler,
		taskRPC:        taskRPCHandler,
		sdsCatalog:     sdsCatalogHandler,
	})
	if err != nil {
		panic(err)
	}
	return routes
}
```

- [ ] **Step 4: Re-run focused and package-level route tests**

Run:

```powershell
go test ./internal/app/httpapi -run "TestBuildRegisteredRoutesMatchesLegacyRouteDescriptors|TestRegisterRoutes" -count=1
```

Then run:

```powershell
go test ./internal/app/httpapi -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/module_runtime.go internal/app/httpapi/server.go internal/app/httpapi/server_test.go
git commit -m "refactor: build HTTP routes from registered modules"
```

### Task 4: Rewire bootstrap assembly to use the module runtime without changing service construction

**Files:**
- Modify: `internal/app/httpapi/modules.go`
- Modify: `internal/app/httpapi/app.go`
- Test: `internal/app/httpapi/server_test.go`
- Test: `internal/app/httpapi/sds_catalog_handler_test.go`

- [ ] **Step 1: Write the failing bootstrap regression tests**

Add tests that lock in current bootstrap behavior while proving route assembly now flows through the module runtime:

```go
func TestBuildBootstrapBuildsServerFromRegisteredModules(t *testing.T) {
	logger := logrus.New()
	bootstrap, err := buildBootstrap(logger, Options{
		ConfigPath: testConfigPath(t),
		Port:       18080,
	})

	require.NoError(t, err)
	require.NotNil(t, bootstrap.server)
	require.NotEmpty(t, bootstrap.routes)
	require.Contains(t, routePaths(bootstrap.routes), "GET /health")
}

func TestRegisterRoutesStillMountsSDSCatalogEndpoints(t *testing.T) {
	router := gin.New()
	RegisterRoutes(router, nil, nil, nil, nil, nil, newSDSCatalogHandler(stubSDSCatalogService{}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sds/categories", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.NotEqual(t, http.StatusNotFound, resp.Code)
}
```

- [ ] **Step 2: Run the bootstrap-focused tests to verify they fail where assembly is still direct**

Run:

```powershell
go test ./internal/app/httpapi -run "TestBuildBootstrapBuildsServerFromRegisteredModules|TestRegisterRoutesStillMountsSDSCatalogEndpoints" -count=1
```

Expected: FAIL until `modules.go` and `server.go` fully rely on the registry-backed route path.

- [ ] **Step 3: Replace remaining direct route-family assumptions in bootstrap**

Update `internal/app/httpapi/modules.go` so bootstrap code only assembles handlers and pools, then lets the server helpers derive routes through the registered path:

```go
server, routes := buildHTTPServerBundleWithStudio(
	options.Port,
	productModule.handler,
	imageModule.handler,
	amazonListingModule.handler,
	listingKitModule.handler,
	promptTemplateHandler,
	listingKitModule.studioSessionHandler,
	sheinLoginHandler,
	sdsLoginHandler,
	taskRPCHandler,
	sdsCatalogHandler,
)

return &appBootstrap{
	productHandler:        productModule.handler,
	imageHandler:          imageModule.handler,
	amazonListingHandler:  amazonListingModule.handler,
	listingKitHandler:     listingKitModule.handler,
	promptTemplateHandler: promptTemplateHandler,
	studioSessionHandler:  listingKitModule.studioSessionHandler,
	sdsCatalogHandler:     sdsCatalogHandler,
	sheinLoginHandler:     sheinLoginHandler,
	sdsLoginHandler:       sdsLoginHandler,
	taskRPCHandler:        taskRPCHandler,
	server:                server,
	routes:                routes,
	pools:                 []worker.WorkerPool{productModule.pool, imageModule.pool, amazonListingModule.pool, listingKitModule.pool},
	closers:               deps.closers,
}, nil
```

Do not change how product/image/amazon/listingkit services are constructed in this phase. The only desired behavior change is where route registration responsibility lives.

- [ ] **Step 4: Run full package verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/amazonlisting/httpapi -count=1
go test ./internal/listingkit/httpapi -count=1
go test ./internal/sheinlogin -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/modules.go internal/app/httpapi/app.go internal/app/httpapi/server_test.go internal/app/httpapi/sds_catalog_handler_test.go
git commit -m "refactor: route HTTP bootstrap through module registry"
```

## Self-Review

### Spec coverage

- The spec’s `Phase 1` goal was a kernel registration surface plus HTTP runtime migration. Task 1 introduces the kernel registry contracts. Tasks 2 through 4 move the HTTP runtime to registered module assembly while preserving existing service construction and route behavior.
- The spec also called out reuse of existing mature patterns. The plan explicitly reuses `internal/platforms/*/module.go` naming and enablement style and reuses existing route descriptor appenders instead of designing a new routing DSL.

### Placeholder scan

- No `TODO`, `TBD`, or “implement later” markers are left in the plan.
- Every code-changing step includes concrete file paths, code snippets, commands, and expected results.

### Type consistency

- `kernelmodule.Registry`, `kernelmodule.Module`, and `httpModuleHandlers` are introduced before later tasks depend on them.
- `newAmazonListingHTTPModule` is defined in Task 2 before Task 3 uses it in `buildRegisteredRoutes`.
- `buildRegisteredRoutes` is defined in Task 3 before Task 4 depends on registry-backed bootstrap behavior.
- All route-focused steps consistently use `httproute.Descriptor` via the existing `routeDescriptor` alias and current `buildCoreRouteDescriptors` / `append*RouteDescriptors` helpers.
