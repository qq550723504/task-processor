# ListingKit HTTPAPI Module Registrar Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `internal/listingkit/httpapi` bootstrap assembly into package-local module registrars so task, submit, admin, and Temporal wiring evolve behind narrower boundaries without changing public bootstrap APIs.

**Architecture:** Keep `BuildService` and `BuildModule` as the composition root entrypoints, but move repository views, dependency resolution, and runtime wiring into module-scoped registrar files. Preserve current closer ordering, service construction semantics, and handler/runtime behavior while shrinking the amount of code that needs to know the full dependency graph.

**Tech Stack:** Go, Logrus, Gin-adjacent handler wiring, existing ListingKit service/config types, existing bootstrap tests

---

### Task 1: Introduce module-scoped registrar data structures

**Files:**
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Create: `internal/listingkit/httpapi/bootstrap_task_module.go`
- Create: `internal/listingkit/httpapi/bootstrap_submit_module.go`
- Create: `internal/listingkit/httpapi/bootstrap_admin_module.go`
- Create: `internal/listingkit/httpapi/bootstrap_temporal_module.go`
- Test: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Write the failing tests for module-scoped outputs**

Add focused tests that describe the new boundary shape without changing behavior:

```go
func TestBuildTaskModuleUsesOnlyTaskScopedRepositories(t *testing.T) {
	taskModule := buildTaskModule(taskModuleInput{
		TaskRepository:           stubTaskRepository{},
		StudioAsyncJobRepository: stubStudioAsyncJobRepository{},
		SubscriptionService:      stubSubscriptionService(),
	})

	require.NotNil(t, taskModule)
	require.NotNil(t, taskModule.handlerDependencies)
}

func TestBuildAdminModuleMapsAdminRepositories(t *testing.T) {
	adminModule := buildAdminModule(adminModuleInput{
		StoreRepository:           stubStoreRepository{},
		CategoryRepository:        stubCategoryRepository{},
		ProductDataRepository:     stubProductDataRepository{},
		OperationStrategyRepository: stubOperationStrategyRepository{},
	})

	require.NotNil(t, adminModule.handlerDependencies)
}
```

- [ ] **Step 2: Run targeted tests to verify they fail**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestBuild(Task|Admin)Module" -count=1
```

Expected: FAIL with undefined registrar/module symbols.

- [ ] **Step 3: Add package-local registrar files and output structs**

Create narrow input/output structs in new files, for example:

```go
type taskModuleInput struct {
	TaskRepository           listingkit.Repository
	StudioAsyncJobRepository listingkit.StudioAsyncJobRepository
	SubscriptionService      *listingsubscription.Service
}

type taskModule struct {
	taskRepository           listingkit.Repository
	studioAsyncJobRepository listingkit.StudioAsyncJobRepository
	subscriptionService      *listingsubscription.Service
}

func buildTaskModule(in taskModuleInput) taskModule {
	return taskModule{
		taskRepository:           in.TaskRepository,
		studioAsyncJobRepository: in.StudioAsyncJobRepository,
		subscriptionService:      in.SubscriptionService,
	}
}
```

Mirror this pattern for:

- `adminModule`
- `submitModule`
- `temporalModule`

- [ ] **Step 4: Run targeted registrar tests to verify they pass**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestBuild(Task|Admin)Module" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_*_module.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "refactor: add listingkit httpapi module registrar skeletons"
```

### Task 2: Move submit and service-config dependency resolution behind registrars

**Files:**
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/listingkit/httpapi/bootstrap_submit_module.go`
- Test: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Write the failing test for submit module dependency resolution**

Add a focused test that proves the submit registrar owns SHEIN-related hook resolution:

```go
func TestBuildSubmitModuleResolvesSheinDependencies(t *testing.T) {
	module := buildSubmitModule(submitModuleInput{
		Config:             testConfig(),
		Logger:             logrus.New(),
		AICredentialStore:  stubAICredentialStore{},
		Hooks:              testHooks(),
		StoreRepository:    stubStoreRepository{},
		ResolutionCache:    stubResolutionCacheStore{},
	})

	require.NotNil(t, module.assembler)
	require.NotNil(t, module.sheinProductAPIBuilder)
	require.NotNil(t, module.sheinAPIClientFactory)
}
```

- [ ] **Step 2: Run the targeted test to verify it fails**

Run:

```powershell
go test ./internal/listingkit/httpapi -run TestBuildSubmitModuleResolvesSheinDependencies -count=1
```

Expected: FAIL until submit registrar owns the resolved fields.

- [ ] **Step 3: Move `resolveModuleDependencies` logic into `buildSubmitModule`**

Shift SHEIN-specific resolution into the submit registrar and keep `bootstrap.go` orchestration-only:

```go
func buildSubmitModule(in submitModuleInput) submitModule {
	categoryLLM := in.Hooks.SheinCategoryLLMClientBuilder(in.Config, in.AICredentialStore)
	saleAttrLLM := in.Hooks.SheinSaleAttributeLLMBuilder(in.Config, in.AICredentialStore)
	categoryResolver := in.Hooks.SheinCategoryResolverBuilder(in.StoreRepository, categoryLLM, in.ResolutionCache)

	return submitModule{
		assembler: listingkit.NewAssemblerWithConfig(listingkit.AssemblerConfig{
			SheinCategoryResolver: categoryResolver,
		}),
		sheinProductAPIBuilder: in.Hooks.SheinProductAPIBuilderFactory(in.StoreRepository),
	}
}
```

Then change `buildListingKitServiceConfig` to accept module outputs instead of a flattened dependency list.

- [ ] **Step 4: Re-run submit/config mapping tests**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestBuildSubmitModuleResolvesSheinDependencies|TestBuildListingKitServiceConfigMapsResolvedDependencies|TestResolveModuleDependenciesBuildsExpectedResolvedValues" -count=1
```

Expected: PASS after the test names are updated to the new registrar-oriented behavior.

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_submit_module.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "refactor: move listingkit submit dependency resolution behind registrar"
```

### Task 3: Rewire `BuildService` and `BuildModule` to compose registrars

**Files:**
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/listingkit/httpapi/bootstrap_task_module.go`
- Modify: `internal/listingkit/httpapi/bootstrap_admin_module.go`
- Modify: `internal/listingkit/httpapi/bootstrap_temporal_module.go`
- Test: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Write the failing integration-style tests for registrar composition**

Add tests that assert the composition root now uses registrars while behavior stays the same:

```go
func TestBuildServiceAssemblesBundleFromModuleRegistrars(t *testing.T) {
	bundle, err := BuildService(validBuildServiceInput())

	require.NoError(t, err)
	require.NotNil(t, bundle.TaskRepository)
	require.NotNil(t, bundle.StoreRepository)
	require.NotNil(t, bundle.SubscriptionService)
}

func TestBuildModuleRuntimeStillUsesTemporalWorkerService(t *testing.T) {
	module, err := BuildModule(validBuildModuleInput())

	require.NoError(t, err)
	require.NotNil(t, module.Handler)
	require.NotNil(t, module.StudioSessionHandler)
}
```

- [ ] **Step 2: Run the targeted tests to verify they fail where composition is still old-shaped**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestBuildServiceAssemblesBundleFromModuleRegistrars|TestBuildModuleRuntimeStillUsesTemporalWorkerService" -count=1
```

Expected: FAIL until `BuildService`/`BuildModule` consume module outputs directly.

- [ ] **Step 3: Rewire the composition root**

Update `BuildService` to follow this sequence:

```go
func BuildService(input BuildServiceInput) (_ *ServiceBundle, err error) {
	repositories, err := buildRepositories(input, closers)
	if err != nil {
		return nil, err
	}

	task := buildTaskModule(taskModuleInputFromRepositories(repositories))
	admin := buildAdminModule(adminModuleInputFromRepositories(repositories))
	submit := buildSubmitModule(submitModuleInputFromBuildServiceInput(input, repositories))

	svc, err := buildModuleService(input, repositories, task, admin, submit, closers)
	if err != nil {
		return nil, err
	}

	temporal := buildTemporalModule(temporalModuleInput{Service: svc})
	return assembleServiceBundle(repositories, temporal.service, closers.Snapshot()), nil
}
```

Keep current closer and auth semantics unchanged.

- [ ] **Step 4: Run focused and full package verification**

Run:

```powershell
go test ./internal/listingkit/httpapi -count=1
go test ./internal/listingkit ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_*_module.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "refactor: compose listingkit httpapi bootstrap from module registrars"
```

### Task 4: Clean up obsolete aggregate helpers and rename tests around the new boundaries

**Files:**
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Remove or rename helpers that still describe the pre-registrar shape**

Retire names like `resolvedModuleDependencies` once their responsibilities move into registrar outputs. Keep only compatibility aggregates that still serve the composition root.

- [ ] **Step 2: Rename tests to match the new boundary language**

Examples:

- `TestResolveModuleDependenciesBuildsExpectedResolvedValues` -> `TestBuildSubmitModuleResolvesExpectedDependencies`
- `TestBuildListingKitServiceConfigMapsResolvedDependencies` -> `TestBuildListingKitServiceConfigMapsModuleOutputs`

- [ ] **Step 3: Run final verification**

Run:

```powershell
go test ./internal/listingkit/httpapi -count=1
go test ./internal/listingkit ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "refactor: align listingkit httpapi tests with registrar boundaries"
```
