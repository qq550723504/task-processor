# Task Processor Framework Phase 2A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the remaining app-local HTTP feature builders for `product`, `image`, `amazon listing`, and `listingkit` into feature-owned packages, and split `runtimeDeps` along ownership lines so `internal/app/httpapi` stops acting as both a shared runtime container and a feature-state bag.

**Architecture:** Reuse the repo’s existing mature `BuildModule` / `BuildResult` pattern already used by `promptmgmt/api`, `taskrpcapi`, `sds/httpapi`, `sheinlogin/bootstrap`, and `sdslogin/bootstrap` instead of inventing a new container DSL. Keep HTTP route registration on the current module-based path, but move feature construction contracts into the feature packages themselves. In the app layer, replace the current single mutable `runtimeDeps` shape with an explicit split between long-lived shared runtime dependencies and short-lived feature bootstrap state.

**Tech Stack:** Go, Gin, Logrus, existing `internal/app/httpapi` composition tests, existing feature `BuildModule(...)` constructors, existing ListingKit bootstrap/service bundle

**Out of Scope For This Slice:** worker registration, Temporal/workflow registration, scheduler registration, and broad adapter extraction. Those should follow as a separate `Phase 2B` once HTTP feature ownership is stable.

---

### Task 1: Split `runtimeDeps` into shared runtime inputs and feature bootstrap state

**Files:**
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/runtime.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/app/httpapi/runtime_deps_test.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`

- [ ] **Step 1: Write the failing tests for the ownership split**

Add focused tests that prove:

- shared runtime construction does **not** populate feature state
- feature attachment mutates only the feature-state side
- `listingKitSupport` remains lazily created off feature state, not the shared runtime object

Add to `internal/app/httpapi/runtime_deps_test.go`:

```go
func TestBuildRuntimeDepsInitializesSharedRuntimeWithoutFeatureState(t *testing.T) {
	logger := logrus.New()
	deps, err := buildRuntimeDeps(logger, testConfigPath(t))
	require.NoError(t, err)
	require.NotNil(t, deps)
	require.NotNil(t, deps.shared)
	require.NotNil(t, deps.features)
	require.Nil(t, deps.features.productService)
	require.Nil(t, deps.features.imageService)
	require.Nil(t, deps.features.listingKitSupport)
}

func TestRuntimeDepsAttachBuiltFeatureModulesOnlyMutatesFeatureState(t *testing.T) {
	deps := &runtimeDeps{
		shared:   &sharedRuntimeDeps{},
		features: &featureRuntimeState{},
	}

	deps.attachSDSLoginResult(&sdsloginbootstrap.BuildResult{
		StatusProvider: func(context.Context) sdslogin.Status { return sdslogin.Status{} },
	})

	require.Nil(t, deps.shared.openaiMgr)
	require.NotNil(t, deps.features.sdsLoginStatusProvider)
}
```

- [ ] **Step 2: Run the focused tests to confirm the old shape fails**

Run:

```powershell
go test ./internal/app/httpapi -run "TestBuildRuntimeDepsInitializesSharedRuntimeWithoutFeatureState|TestRuntimeDepsAttachBuiltFeatureModulesOnlyMutatesFeatureState" -count=1
```

Expected: FAIL because `runtimeDeps.shared` / `runtimeDeps.features` do not exist yet.

- [ ] **Step 3: Implement the split and move helper methods to the right side**

Refactor `internal/app/httpapi/types.go` so the current `runtimeDeps` becomes an explicit composition:

```go
type runtimeDeps struct {
	shared   *sharedRuntimeDeps
	features *featureRuntimeState
}

type sharedRuntimeDeps struct {
	cfg               *config.Config
	closers           []func() error
	openaiMgr         *openaiclient.Manager
	aiCredentialStore *openaiclient.GormCredentialResolver
	tenantPromptStore prompt.TenantPromptStore
	llmMgr            productenrich.LLMManager
	inputParser       productenrich.InputParser
	understanding     productenrich.ProductUnderstanding
	imageWorkDir      string
	sharedResources   *appbootstrap.SharedResources
}

type featureRuntimeState struct {
	productService         productenrich.ProductService
	imageService           productimage.Service
	sdsLoginStatusProvider listingkit.SDSLoginStatusProvider
	imageSubjectExtractor  productimage.SubjectExtractor
	imageWhiteBgRenderer   productimage.WhiteBackgroundRenderer
	imageSceneRenderer     productimage.SceneRenderer
	listingKitSupport      *listingKitSupport
}
```

Then update the methods in `runtime.go` and `listingkit_support.go` so:

- `managementClient()` reads from `shared.sharedResources`
- `addClosers(...)` appends to `shared.closers`
- feature attachers write only into `features`
- `ensureListingKitSupport()` lazily initializes `features.listingKitSupport`

Do **not** move any feature logic out of `internal/app/httpapi` yet in this task. The goal here is to make ownership explicit first.

- [ ] **Step 4: Re-run focused and package-level tests**

Run:

```powershell
go test ./internal/app/httpapi -run "TestBuildRuntimeDepsInitializesSharedRuntimeWithoutFeatureState|TestRuntimeDepsAttachBuiltFeatureModulesOnlyMutatesFeatureState|TestRuntimeDepsListingKitSupport" -count=1
go test ./internal/app/httpapi -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/types.go internal/app/httpapi/runtime.go internal/app/httpapi/listingkit_support.go internal/app/httpapi/runtime_deps_test.go internal/app/httpapi/composition_builder.go internal/app/httpapi/composition_builder_test.go
git commit -m "refactor: split http runtime deps by ownership"
```

### Task 2: Add feature-owned runtime builders for `product`, `image`, and `amazon listing`

**Files:**
- Create: `internal/productenrich/httpapi/runtime_builder.go`
- Create: `internal/productimage/httpapi/runtime_builder.go`
- Create: `internal/amazonlisting/httpapi/runtime_builder.go`
- Modify: `internal/productenrich/httpapi/bootstrap.go`
- Modify: `internal/productimage/httpapi/bootstrap.go`
- Modify: `internal/amazonlisting/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/modules.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`

- [ ] **Step 1: Write the failing app-layer tests that forbid app-local wrappers**

Add a focused regression test to `internal/app/httpapi/composition_builder_test.go` that stubs the builder fields and proves the composition builder now calls feature package builders directly instead of `buildProductModule(...)`, `buildImageModule(...)`, and `buildAmazonListingModule(...)` wrappers owned by `internal/app/httpapi`.

Use a shape like:

```go
func TestHTTPFeatureCompositionBuilderUsesFeatureOwnedBuilders(t *testing.T) {
	builder := httpFeatureCompositionBuilder{
		buildProduct: func(logger *logrus.Logger, deps *runtimeDeps) (*productenrichhttpapi.Module, error) {
			t.Helper()
			require.NotNil(t, deps.shared)
			return &productenrichhttpapi.Module{}, nil
		},
		buildImage: func(logger *logrus.Logger, deps *runtimeDeps) (*productimagehttpapi.Module, error) {
			t.Helper()
			require.NotNil(t, deps.features)
			return &productimagehttpapi.Module{}, nil
		},
		buildAmazonListing: func(logger *logrus.Logger, deps *runtimeDeps) (*amazonlistinghttpapi.Module, error) {
			t.Helper()
			require.NotNil(t, deps.features.productService)
			return &amazonlistinghttpapi.Module{}, nil
		},
        // other builders stubbed as nil-safe no-ops
	}
	// ...
}
```

The important failure mode is that `internal/app/httpapi/modules.go` should no longer need those trivial wrapper functions at all.

- [ ] **Step 2: Run the focused tests to confirm the wrappers are still required today**

Run:

```powershell
go test ./internal/app/httpapi -run TestHTTPFeatureCompositionBuilderUsesFeatureOwnedBuilders -count=1
```

Expected: FAIL until the feature packages expose runtime-owned builder entrypoints and the app-layer wrappers are removed.

- [ ] **Step 3: Introduce feature-owned runtime builder entrypoints**

Follow the existing mature pattern in `taskrpcapi.BuildModule(...)` and `sdshttpapi.BuildModule(...)`, but keep the return type as the existing `*Module` to avoid a broad compatibility rewrite.

Create `internal/productenrich/httpapi/runtime_builder.go`:

```go
package httpapi

import "github.com/sirupsen/logrus"

type RuntimeBuildInput struct {
	Logger        *logrus.Logger
	Config        *config.Config
	LLMManager    productenrich.LLMManager
	InputParser   productenrich.InputParser
	Understanding productenrich.ProductUnderstanding
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		Config:        input.Config,
		Logger:        input.Logger,
		LLMManager:    input.LLMManager,
		InputParser:   input.InputParser,
		Understanding: input.Understanding,
	})
}
```

Mirror that pattern for:

- `internal/productimage/httpapi/runtime_builder.go`
- `internal/amazonlisting/httpapi/runtime_builder.go`

For Amazon listing, the runtime build input should explicitly declare its dependency on `ProductService` and `ImageService` so that dependency ownership is defined by the feature package, not by `internal/app/httpapi`.

- [ ] **Step 4: Rewire app composition to call the feature-owned builders and delete the app-local wrappers**

Update `internal/app/httpapi/composition_builder.go`:

```go
func newHTTPFeatureCompositionBuilder() httpFeatureCompositionBuilder {
	return httpFeatureCompositionBuilder{
		buildProduct: productenrichhttpapi.BuildRuntimeModule,
		buildImage:   productimagehttpapi.BuildRuntimeModule,
		buildAmazonListing: amazonlistinghttpapi.BuildRuntimeModule,
        // unchanged builders...
	}
}
```

Then delete these app-local wrappers from `internal/app/httpapi/modules.go`:

- `buildProductModule(...)`
- `buildImageModule(...)`
- `buildAmazonListingModule(...)`

Keep `buildListingKitModule(...)` for the next task. It still owns too much app-local knowledge to move cleanly in the same step.

- [ ] **Step 5: Re-run package verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/productenrich/httpapi -count=1
go test ./internal/productimage/httpapi -count=1
go test ./internal/amazonlisting/httpapi -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/productenrich/httpapi/runtime_builder.go internal/productimage/httpapi/runtime_builder.go internal/amazonlisting/httpapi/runtime_builder.go internal/productenrich/httpapi/bootstrap.go internal/productimage/httpapi/bootstrap.go internal/amazonlisting/httpapi/bootstrap.go internal/app/httpapi/composition_builder.go internal/app/httpapi/modules.go internal/app/httpapi/composition_builder_test.go
git commit -m "refactor: move core http feature builders into feature packages"
```

### Task 3: Move ListingKit runtime input shaping behind `listingkit/httpapi`

**Files:**
- Create: `internal/listingkit/httpapi/runtime_builder.go`
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/listingkit_feature_builder.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`
- Modify: `internal/app/httpapi/listingkit_feature_builder_test.go`

- [ ] **Step 1: Write the failing tests for ListingKit runtime builder ownership**

Add tests that prove app-layer code only provides prereq services/support and does **not** shape the full `listingkithttpapi.BuildModuleInput` itself.

Examples:

```go
func TestHTTPFeatureCompositionBuilderUsesListingKitFeatureBuilder(t *testing.T) {
	var sawInput listingkithttpapi.RuntimeBuildInput

	builder := httpFeatureCompositionBuilder{
		buildListingKit: func(logger *logrus.Logger, deps *runtimeDeps) (*listingkithttpapi.Module, error) {
			module, err := listingkithttpapi.BuildRuntimeModule(listingkithttpapi.RuntimeBuildInput{
				Logger:  logger,
				Runtime: sawInput.Runtime,
			})
			return module, err
		},
	}
	// ...
}
```

The key assertion is not the exact type name. The key assertion is that the ListingKit package now owns the shape of its runtime-facing builder contract.

- [ ] **Step 2: Run the focused tests to confirm app layer still owns the input shaping**

Run:

```powershell
go test ./internal/app/httpapi -run "TestHTTPFeatureCompositionBuilderUsesListingKitFeatureBuilder|TestListingKitFeatureBuilderBuildsRequestedFeatures" -count=1
```

Expected: FAIL until `newListingKitBuildModuleInput(...)` is no longer app-owned.

- [ ] **Step 3: Introduce a package-owned runtime builder contract in `listingkit/httpapi`**

Create `internal/listingkit/httpapi/runtime_builder.go` with a feature-owned builder input that wraps the current `BuildModuleInput` and the current app-local support inputs:

```go
type RuntimeBuildInput struct {
	Logger  *logrus.Logger
	Runtime RuntimeDependencies
}

type RuntimeDependencies struct {
	Config                     *config.Config
	ProductService             productenrich.ProductService
	ImageService               productimage.Service
	SDSSyncService             sdsusecase.Service
	SDSLoginStatusProvider     listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider  listingkit.SDSBaselineRemoteProvider
	ImageSubjectExtractor      productimage.SubjectExtractor
	ImageWhiteBackgroundRender productimage.WhiteBackgroundRenderer
	ImageSceneRenderer         productimage.SceneRenderer
	AICredentialStore          aiCredentialStore
	ShouldStartTemporalWorkerInProcess bool
}

func BuildRuntimeModule(input RuntimeBuildInput) (*Module, error) {
	return BuildModule(BuildModuleInput{
		ServiceInput: buildRuntimeServiceInput(input),
		ShouldStartTemporalWorkerInProcess: input.Runtime.ShouldStartTemporalWorkerInProcess,
	})
}
```

Important:

- keep reusing the existing `BuildModule(...)` and `BuildService(...)`
- do **not** redesign ListingKit bootstrap internals in this slice
- move only the runtime-facing contract ownership into `listingkit/httpapi`

- [ ] **Step 4: Reduce app-layer ListingKit knowledge to support preparation only**

After `listingkithttpapi.BuildRuntimeModule(...)` exists:

- replace `newListingKitBuildModuleInput(...)` with a smaller helper that prepares only SDS baseline / cookie store / AI credential support
- keep `listingkit_support.go` focused on support assembly, not full input shaping
- change `composition_builder.go` and `listingkit_feature_builder.go` to call `listingkithttpapi.BuildRuntimeModule(...)`

The target is that `internal/app/httpapi` still knows how to prepare app-local support, but no longer knows the full `BuildModuleInput` contract of ListingKit.

- [ ] **Step 5: Re-run package verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/listingkit/httpapi -count=1
go test ./internal/listingkit/... -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/httpapi/runtime_builder.go internal/listingkit/httpapi/bootstrap.go internal/app/httpapi/listingkit_support.go internal/app/httpapi/composition_builder.go internal/app/httpapi/listingkit_feature_builder.go internal/app/httpapi/composition_builder_test.go internal/app/httpapi/listingkit_feature_builder_test.go
git commit -m "refactor: move listingkit runtime builder into feature package"
```

### Task 4: Shrink `internal/app/httpapi` to a shared composition root and lock the new boundary with tests

**Files:**
- Modify: `internal/app/httpapi/modules.go`
- Modify: `internal/app/httpapi/runtime.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/e2e_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_live_test.go`
- Modify: `internal/app/httpapi/http_module_test.go`
- Create: `internal/app/httpapi/phase2_boundary_test.go`

- [ ] **Step 1: Add boundary tests that describe the desired end state**

Create `internal/app/httpapi/phase2_boundary_test.go` with focused invariants:

```go
func TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers(t *testing.T) {
	src, err := os.ReadFile("modules.go")
	require.NoError(t, err)
	require.NotContains(t, string(src), "func buildProductModule(")
	require.NotContains(t, string(src), "func buildImageModule(")
	require.NotContains(t, string(src), "func buildAmazonListingModule(")
	require.NotContains(t, string(src), "func buildListingKitModule(")
}
```

This is intentionally a high-signal architectural guardrail. The point is to lock the boundary after the refactor lands.

- [ ] **Step 2: Run the focused boundary tests to verify they fail before cleanup is complete**

Run:

```powershell
go test ./internal/app/httpapi -run TestHTTPAPIModulesFileDoesNotOwnFeatureBuildWrappers -count=1
```

Expected: FAIL until the app-local wrappers are removed.

- [ ] **Step 3: Clean up remaining app-layer staging and update side-entry tests**

Finish the slice by ensuring:

- `modules.go` is limited to runtime orchestration and `buildBootstrap(...)`
- `composition_builder.go` depends only on feature-owned builders plus app-local support prep
- the ListingKit side-entry flows still go through the same feature-owned build path
- E2E tests still pass on the composition-based server path

Do **not** reopen worker/workflow registration in this cleanup task.

- [ ] **Step 4: Run full verification**

Run:

```powershell
gopls check internal/app/httpapi/types.go internal/app/httpapi/runtime.go internal/app/httpapi/modules.go internal/app/httpapi/composition_builder.go internal/app/httpapi/listingkit_support.go internal/app/httpapi/listingkit_feature_builder.go internal/productenrich/httpapi/runtime_builder.go internal/productimage/httpapi/runtime_builder.go internal/amazonlisting/httpapi/runtime_builder.go internal/listingkit/httpapi/runtime_builder.go
go test ./internal/app/httpapi -count=1
go test ./internal/productenrich/httpapi ./internal/productimage/httpapi ./internal/amazonlisting/httpapi ./internal/listingkit/httpapi -count=1
go test ./internal/listingkit/... -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/modules.go internal/app/httpapi/runtime.go internal/app/httpapi/types.go internal/app/httpapi/e2e_test.go internal/app/httpapi/e2e_listingkit_sds_test.go internal/app/httpapi/e2e_listingkit_sds_live_test.go internal/app/httpapi/http_module_test.go internal/app/httpapi/phase2_boundary_test.go
git commit -m "refactor: narrow http app layer to shared composition"
```

## Self-Review

### Spec coverage

- The framework design says `Phase 2` should convert business assembly into real modules and reduce global bootstrap ownership. This plan does that for the highest-value HTTP slice first: `product`, `image`, `amazon listing`, and `listingkit`.
- The `Phase 1` checkpoint explicitly recommended:
  1. feature-owned builders for `product/image/amazon/listingkit`
  2. smaller runtime dependency ownership
  3. worker/workflow registration next

This plan covers only items 1 and 2, intentionally leaving item 3 for a later `Phase 2B` so the slice stays executable.

### Reuse check

- The plan explicitly reuses the repo’s existing mature `BuildModule(...)` / `BuildResult` style instead of inventing a new bootstrap framework.
- The ListingKit step keeps reusing `BuildModule(...)` and `BuildService(...)` rather than rewriting ListingKit bootstrap internals.

### Placeholder scan

- No `TODO`, `TBD`, or “implement later” placeholders are left in the task steps.
- Every task includes concrete files, expected commands, and a commit boundary.

### Type consistency

- `runtimeDeps` ownership split lands before feature builder rewiring depends on it.
- core feature runtime builders land before ListingKit’s more complex runtime builder.
- the app-layer boundary test lands only after the feature-owned builders exist, so it guards the intended final state rather than blocking earlier incremental steps.
