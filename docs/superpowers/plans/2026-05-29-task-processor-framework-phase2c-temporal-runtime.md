# Task Processor Framework Phase 2C Temporal Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the module-registration path from HTTP routes and worker pools to the current ListingKit Temporal runtime, so `internal/app/httpapi` and `cmd/listingkit-temporal-worker` stop hand-owning feature-specific Temporal worker startup and workflow-host assembly.

**Architecture:** Reuse the existing `kernel/module.Registry`, the feature-owned `BuildRuntimeModule(...)` pattern, and Temporal’s own worker-registration model. Do **not** invent a generic workflow engine or a new DI/container DSL. This slice should only make the current ListingKit Temporal worker/client runtime look like a module-owned runtime contribution, the same way HTTP routes and worker pools already do.

**Tech Stack:** Go, existing `internal/kernel/module` registry, existing `internal/app/runtime/temporal_runtime.go`, existing `internal/listingkit/httpapi` service/runtime builders, Temporal Go SDK, current `cmd/listingkit-temporal-worker`

**Out of Scope For This Slice:**

- a repository-wide generic workflow engine
- scheduler registration
- non-ListingKit Temporal runtimes
- replacing `internal/app/runtime/temporal_runtime.go` with a new abstraction layer
- redesigning the ListingKit workflow contracts themselves

---

## Root Cause This Slice Addresses

After `Phase 2B`, worker pools and HTTP routes already flow through `kernel/module.Registry`, but the Temporal runtime still breaks that pattern in three places:

1. `internal/listingkit/httpapi/bootstrap.go`
   - still decides whether to start the in-process Temporal worker
   - still wires Temporal workflow clients directly during service bootstrap

2. `internal/app/httpapi/listingkit_temporal_worker.go`
   - still hand-builds the minimum ListingKit prerequisites for the standalone Temporal worker
   - still knows it must call `BuildService(...)` and then `appruntime.StartListingKitSheinPublishTemporalWorker(...)`

3. `cmd/listingkit-temporal-worker/main.go`
   - still reaches a dedicated app-layer helper rather than consuming a shared runtime bundle path

The root problem is not “Temporal exists.” The root problem is that Temporal runtime contributions still bypass the module registry and therefore keep runtime selection and feature-specific startup knowledge split across app-layer entrypoints.

---

## Target Outcome

At the end of `Phase 2C`, the ListingKit Temporal runtime should look like a module-owned runtime contribution:

- ListingKit owns how its Temporal worker service is built
- the kernel registry owns collecting Temporal worker starters the same way it now collects worker pools
- app-layer entrypoints decide **which runtime mode to run**, but not **how ListingKit Temporal internals are assembled**
- standalone and in-process Temporal startup both reuse the same feature-owned runtime registration path

This slice does **not** need to make every workflow in the repo generic. It only needs to make the current ListingKit Temporal runtime stop being a special-case app bootstrap path.

---

## Task 1: Extend `kernel/module.Registry` to collect named Temporal worker starters

**Files:**
- Modify: `internal/kernel/module/interfaces.go`
- Modify: `internal/kernel/module/registry.go`
- Modify: `internal/kernel/module/registry_test.go`

- [ ] **Step 1: Write failing tests for Temporal worker starter registration**

Add focused tests to `internal/kernel/module/registry_test.go` that prove:

- starters preserve registration order
- duplicate Temporal runtime names are rejected
- nil starter functions are rejected

Suggested shape:

```go
func TestRegistryAddTemporalWorkerPreservesOrder(t *testing.T) {
	reg := NewRegistry()

	err := reg.AddTemporalWorker("listingkit_publish", stubTemporalWorkerStarter("first"))
	require.NoError(t, err)
	err = reg.AddTemporalWorker("listingkit_layers", stubTemporalWorkerStarter("second"))
	require.NoError(t, err)

	workers := reg.TemporalWorkers()
	require.Len(t, workers, 2)
	require.Equal(t, "listingkit_publish", workers[0].Name)
	require.Equal(t, "listingkit_layers", workers[1].Name)
}

func TestRegistryAddTemporalWorkerRejectsDuplicateNames(t *testing.T) {
	reg := NewRegistry()

	err := reg.AddTemporalWorker("listingkit_publish", stubTemporalWorkerStarter("first"))
	require.NoError(t, err)

	err = reg.AddTemporalWorker("listingkit_publish", stubTemporalWorkerStarter("second"))
	require.ErrorContains(t, err, "temporal worker already registered")
}

func TestRegistryAddTemporalWorkerRejectsNilStarter(t *testing.T) {
	reg := NewRegistry()

	err := reg.AddTemporalWorker("listingkit_publish", nil)
	require.ErrorContains(t, err, "temporal worker starter is nil")
}
```

- [ ] **Step 2: Run the focused registry tests to verify they fail**

Run:

```powershell
go test ./internal/kernel/module -run "TestRegistryAddTemporalWorkerPreservesOrder|TestRegistryAddTemporalWorkerRejectsDuplicateNames|TestRegistryAddTemporalWorkerRejectsNilStarter" -count=1
```

Expected: FAIL because `AddTemporalWorker(...)` and `TemporalWorkers()` do not exist yet.

- [ ] **Step 3: Implement named Temporal worker starter registration**

Extend `internal/kernel/module/registry.go` with a minimal runtime contribution type:

```go
type TemporalWorkerStarter func() (func() error, error)

type NamedTemporalWorker struct {
	Name  string
	Start TemporalWorkerStarter
}
```

Then add:

- a `temporalWorkers []NamedTemporalWorker` slice
- a `temporalWorkerNames map[string]struct{}`
- `AddTemporalWorker(...) error`
- `TemporalWorkers() []NamedTemporalWorker`

Important constraints:

- reuse the existing nil-checking style from the registry
- preserve registration order
- do not introduce a second registry package
- do not add scheduler or generic lifecycle hooks in this task

- [ ] **Step 4: Re-run the full kernel module package**

Run:

```powershell
go test ./internal/kernel/module -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/kernel/module/interfaces.go internal/kernel/module/registry.go internal/kernel/module/registry_test.go
git commit -m "feat: add temporal worker registration to module registry"
```

---

## Task 2: Add a feature-owned ListingKit Temporal runtime module

**Files:**
- Create: `internal/listingkit/httpapi/temporal_runtime.go`
- Create: `internal/listingkit/httpapi/temporal_runtime_test.go`
- Modify: `internal/listingkit/httpapi/runtime_builder.go`
- Modify: `internal/listingkit/httpapi/bootstrap_temporal_module.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Write failing tests for a ListingKit-owned Temporal runtime module**

Add focused tests that prove ListingKit can register its Temporal runtime contribution without app-layer help.

Cover these cases:

1. the runtime module registers the expected workflow names
2. the runtime module registers one named Temporal worker starter
3. when the Temporal worker service is missing, registration degrades cleanly instead of crashing

Suggested assertion shape:

```go
func TestTemporalRuntimeModuleRegistersWorkflowNamesAndStarter(t *testing.T) {
	reg := module.NewRegistry()

	err := NewTemporalRuntimeModule(&TemporalRuntimeResult{
		WorkerService: stubTemporalWorkerService{},
		Starter:       stubTemporalWorkerStarter("listingkit_publish"),
	}).Register(reg)
	require.NoError(t, err)

	require.Len(t, reg.TemporalWorkers(), 1)
	require.Equal(t, "listingkit_publish", reg.TemporalWorkers()[0].Name)
}
```

Also add a guard that workflow names for:

- `PublishWorkflow`
- `StandardProductWorkflow`
- `PlatformAdaptWorkflow`

are registered through the same module-owned path rather than through app-local startup code.

- [ ] **Step 2: Run the focused ListingKit tests to verify they fail**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestTemporalRuntimeModuleRegistersWorkflowNamesAndStarter" -count=1
```

Expected: FAIL because the feature-owned Temporal runtime module does not exist yet.

- [ ] **Step 3: Implement a ListingKit Temporal runtime result and module**

Create a small feature-owned runtime surface in `internal/listingkit/httpapi/temporal_runtime.go`.

Recommended shape:

- `TemporalRuntimeInput`
- `TemporalRuntimeResult`
- `BuildTemporalRuntime(...)`
- `NewTemporalRuntimeModule(...)`

Responsibilities:

- build or reuse the ListingKit `TemporalWorkerService`
- expose a named starter closure that internally reuses the existing `appruntime.StartListingKitSheinPublishTemporalWorker(...)`
- register workflow names through the existing `kernel/module.WorkflowHandler` path
- register the named Temporal worker starter through `reg.AddTemporalWorker(...)`

Important constraints:

- keep the Temporal worker startup mechanics in existing Temporal code paths
- do not duplicate worker registration logic from `internal/listingkit/temporal/worker.go`
- do not move SDK-specific `RegisterWorkflowWithOptions(...)` calls into app-layer code

- [ ] **Step 4: Rewire `BuildRuntimeModule(...)` to carry Temporal runtime output**

Update `internal/listingkit/httpapi/runtime_builder.go` and related bootstrap structs so the ListingKit runtime build path can return:

- the regular HTTP/worker module
- the Temporal runtime contribution

without forcing app-layer code to rebuild the service separately.

This is the key root-cause fix for the current duplicate startup path in `internal/app/httpapi/listingkit_temporal_worker.go`.

- [ ] **Step 5: Re-run ListingKit package verification**

Run:

```powershell
go test ./internal/listingkit/httpapi -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/httpapi/temporal_runtime.go internal/listingkit/httpapi/temporal_runtime_test.go internal/listingkit/httpapi/runtime_builder.go internal/listingkit/httpapi/bootstrap_temporal_module.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "feat: add listingkit temporal runtime module"
```

---

## Task 3: Build Temporal runtime bundles from registered modules

**Files:**
- Create: `internal/app/runtime/temporal_bundle.go`
- Create: `internal/app/runtime/temporal_bundle_test.go`
- Modify: `internal/app/runtime/temporal_runtime.go`
- Modify: `internal/app/httpapi/listingkit_temporal_worker.go`
- Modify: `cmd/listingkit-temporal-worker/main.go`

- [ ] **Step 1: Write failing tests for registry-backed Temporal runtime bundles**

Add focused tests to `internal/app/runtime/temporal_bundle_test.go` that prove:

- registered Temporal workers are collected in order
- disabled modules are skipped
- the bundle can start all registered starters and return closers

Suggested test shape:

```go
func TestBuildTemporalRuntimeBundleFromModulesCollectsNamedWorkers(t *testing.T) {
	bundle, err := BuildTemporalRuntimeBundleFromModules(&config.Config{}, []module.Module{
		stubTemporalModule("listingkit_publish"),
	})
	require.NoError(t, err)
	require.Len(t, bundle.Workers, 1)
	require.Equal(t, "listingkit_publish", bundle.Workers[0].Name)
}
```

- [ ] **Step 2: Run the focused runtime tests to verify they fail**

Run:

```powershell
go test ./internal/app/runtime -run "TestBuildTemporalRuntimeBundleFromModulesCollectsNamedWorkers" -count=1
```

Expected: FAIL because the bundle does not exist yet.

- [ ] **Step 3: Implement the Temporal runtime bundle**

Create `internal/app/runtime/temporal_bundle.go` with a small bundle type that:

- builds a registry from modules
- extracts registered workflow names and named Temporal worker starters
- starts all enabled worker starters
- returns closer functions in deterministic order

Keep the bundle minimal. This slice does **not** need a generic runtime manager for every subsystem in the repo.

- [ ] **Step 4: Route standalone ListingKit Temporal startup through the bundle**

Update `internal/app/httpapi/listingkit_temporal_worker.go` so it no longer:

- calls `listingkithttpapi.BuildService(...)` directly
- calls `appruntime.StartListingKitSheinPublishTemporalWorker(...)` directly

Instead it should:

1. build shared runtime deps
2. ask ListingKit for its Temporal runtime module/result
3. build a Temporal runtime bundle from that module
4. start the bundle

Then update `cmd/listingkit-temporal-worker/main.go` only as needed to stay on that shared path.

This is the main app-layer shrink target for this slice.

- [ ] **Step 5: Re-run runtime and command verification**

Run:

```powershell
go test ./internal/app/runtime ./internal/listingkit/httpapi ./cmd/listingkit-temporal-worker -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/runtime/temporal_bundle.go internal/app/runtime/temporal_bundle_test.go internal/app/runtime/temporal_runtime.go internal/app/httpapi/listingkit_temporal_worker.go cmd/listingkit-temporal-worker/main.go
git commit -m "refactor: build temporal runtime bundle from registered modules"
```

---

## Task 4: Lock the Temporal runtime boundary and stop app-layer regressions

**Files:**
- Create: `internal/app/httpapi/phase2c_temporal_boundary_test.go`
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/app/httpapi/listingkit_temporal_worker.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Add a narrow boundary test for app-layer Temporal ownership**

Create `internal/app/httpapi/phase2c_temporal_boundary_test.go`:

```go
func TestListingKitTemporalWorkerEntrypointDoesNotOwnDirectServiceStartup(t *testing.T) {
	src, err := os.ReadFile("listingkit_temporal_worker.go")
	require.NoError(t, err)

	content := string(src)
	require.NotContains(t, content, "BuildService(")
	require.NotContains(t, content, "StartListingKitSheinPublishTemporalWorker(")
}
```

This is intentionally narrow. The goal is only to prevent the old app-layer direct startup path from returning.

- [ ] **Step 2: Run the focused boundary test to verify it fails before cleanup**

Run:

```powershell
go test ./internal/app/httpapi -run TestListingKitTemporalWorkerEntrypointDoesNotOwnDirectServiceStartup -count=1
```

Expected: FAIL until the direct startup path is removed.

- [ ] **Step 3: Clean remaining app-local Temporal wiring**

Make sure:

- `listingkit_support.go` does not decide more than the env-driven enablement flag and ListingKit runtime input assembly
- `listingkit_temporal_worker.go` stays a thin runtime entrypoint
- feature-owned tests cover the runtime contribution path so app-layer tests do not need to know internal ListingKit Temporal service details

- [ ] **Step 4: Run final verification**

Run:

```powershell
go test ./internal/kernel/module -count=1
go test ./internal/app/runtime ./internal/app/httpapi -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
go test ./cmd/listingkit-temporal-worker -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/phase2c_temporal_boundary_test.go internal/app/httpapi/listingkit_support.go internal/app/httpapi/listingkit_temporal_worker.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "test: lock temporal runtime registration boundary"
```

---

## Self-Review

### Spec coverage

- The framework design says `Phase 2` should extend module registration beyond HTTP routes and worker pools into workflow/runtime surfaces. This plan covers the current highest-value workflow runtime: the ListingKit Temporal worker/client path.
- The `Phase 2B` completion left the biggest runtime hole in Temporal startup. This plan closes that specific hole without pretending the repo already needs a generic workflow platform.

### Reuse check

- The plan reuses `kernel/module.Registry` instead of inventing a separate runtime-discovery package.
- The plan reuses the existing ListingKit service/runtime builders instead of introducing a parallel temporal bootstrap tree.
- The plan reuses Temporal’s existing worker registration path in `internal/listingkit/temporal/worker.go` instead of wrapping SDK registration in a new DSL.

### Root-cause check

- The issue is not just “the command is large.” The issue is that runtime startup knowledge for ListingKit Temporal is duplicated across app-layer entrypoints and bypasses module registration.
- This plan fixes that by moving the runtime contribution back behind a feature-owned module and making entrypoints consume a registry-backed Temporal runtime bundle.

### Placeholder scan

- No `TODO` or “decide later” placeholders remain in the task list.
- Each task includes concrete files, verification commands, and commit boundaries.

### Scope discipline

- This slice does not attempt a repo-wide generic workflow engine.
- This slice intentionally limits itself to ListingKit Temporal runtime registration so the architecture move stays incremental and testable.
