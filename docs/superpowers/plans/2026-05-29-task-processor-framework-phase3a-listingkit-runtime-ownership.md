# Task Processor Framework Phase 3A ListingKit Runtime Ownership Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move ListingKit runtime input shaping out of `internal/app/httpapi/listingkit_support.go` and into feature-owned builders inside `internal/listingkit/httpapi`, so the app layer stops owning ListingKit-specific repository bundles, hook bundles, SDS support bootstrap, and SHEIN runtime support bootstrap.

**Architecture:** Reuse the existing `BuildRuntimeModule(...)`, `BuildTemporalRuntime(...)`, and `BuildService(...)` seams already present in `internal/listingkit/httpapi`. Do not redesign ListingKit’s service internals or introduce a new runtime manifest. This slice is about ownership transfer of runtime-input shaping, not service rewrite.

**Tech Stack:** Go, existing `internal/app/httpapi` composition path, existing `internal/listingkit/httpapi` runtime builders, Logrus, existing ListingKit HTTP/Temporal tests

**Out of Scope For This Slice:**

- changing ListingKit service contracts
- adapterizing `internal/app/runtime/temporal_runtime.go`
- inventing a unified runtime manifest for all features
- restructuring all ListingKit bootstrap internals
- touching non-ListingKit feature builders except where tests need shared composition updates

---

## Root Cause This Slice Addresses

After `Phase 2A`, `Phase 2B`, and `Phase 2C`, the app layer no longer owns:

- direct HTTP route assembly
- worker-pool name maps
- standalone Temporal worker startup

But it still owns too much of ListingKit’s runtime contract through:

- [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1)

Today that file still:

1. shapes the full `listingkithttpapi.RuntimeBuildInput`
2. constructs ListingKit repository-builder bundles
3. constructs ListingKit hook-builder bundles
4. bootstraps SDS sync support
5. bootstraps SDS baseline remote-provider support
6. bootstraps SHEIN cookie-store support
7. chooses ListingKit-specific runtime toggles

The root problem is not file size. The root problem is that app-layer code still knows the full feature-owned runtime contract instead of handing shared runtime dependencies to ListingKit and letting the feature package shape its own input.

---

## Target Outcome

At the end of `Phase 3A`:

- `internal/app/httpapi` prepares only shared runtime prerequisites and ListingKit support-state handles
- `internal/listingkit/httpapi` owns the runtime-facing input contract and how repository/hook bundles are assembled
- app-layer code no longer knows ListingKit’s full repository-builder set or hook-builder set
- ListingKit HTTP runtime and Temporal runtime both consume the same feature-owned runtime support contract
- `listingkit_support.go` becomes materially smaller and more obviously app-local

---

## Task 1: Introduce a feature-owned ListingKit runtime support contract

**Files:**
- Create: `internal/listingkit/httpapi/runtime_support.go`
- Modify: `internal/listingkit/httpapi/runtime_builder.go`
- Modify: `internal/listingkit/httpapi/temporal_runtime.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Create: `internal/listingkit/httpapi/runtime_support_test.go`

- [ ] **Step 1: Write failing tests for feature-owned runtime support shaping**

Add focused tests that prove ListingKit can assemble its own runtime support bundles without app-layer helper functions directly constructing:

- `BuildServiceRepositories`
- `BuildServiceHooks`

Cover at least:

1. repository builders are filled by a feature-owned helper
2. hook builders are filled by a feature-owned helper
3. `BuildRuntimeModule(...)` and `BuildTemporalRuntime(...)` can both consume the same support contract

Suggested shape:

```go
func TestBuildRuntimeSupportProvidesRepositoryAndHookBundles(t *testing.T) {
	support := BuildRuntimeSupport(RuntimeSupportInput{
		SheinCookieStore: &sheinlogin.RedisStore{},
	})

	require.NotNil(t, support.Repositories.Core.Task)
	require.NotNil(t, support.Repositories.Admin.Store)
	require.NotNil(t, support.Hooks.SheinPricingPolicyBuilder)
	require.NotNil(t, support.Hooks.ConfigureAuthorization)
}
```

- [ ] **Step 2: Run the focused ListingKit tests to confirm support ownership is still app-local**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestBuildRuntimeSupportProvidesRepositoryAndHookBundles" -count=1
```

Expected: FAIL because the feature-owned runtime-support helper does not exist yet.

- [ ] **Step 3: Implement a ListingKit-owned runtime support builder**

Create `internal/listingkit/httpapi/runtime_support.go` with a compact feature-owned support contract.

Recommended shape:

```go
type RuntimeSupportInput struct {
	SheinCookieStore *sheinlogin.RedisStore
}

type RuntimeSupport struct {
	Repositories BuildServiceRepositories
	Hooks        BuildServiceHooks
}

func BuildRuntimeSupport(input RuntimeSupportInput) RuntimeSupport
```

Responsibilities:

- own `BuildServiceRepositories` creation
- own `BuildServiceHooks` creation
- accept only true app-owned support inputs such as cookie-store handles

Important:

- reuse existing repository and hook builder functions
- do not rewrite `BuildService(...)`
- do not move SDS sync/bootstrap logic here yet in this task

- [ ] **Step 4: Rewire `BuildRuntimeModule(...)` and `BuildTemporalRuntime(...)` to consume the support contract**

Update ListingKit’s runtime builders so they can accept a `RuntimeSupport` payload rather than forcing app-layer code to create:

- repository builder sets
- hook builder sets

This should make `runtime_builder.go` and `temporal_runtime.go` share the same feature-owned support-shaping path.

- [ ] **Step 5: Re-run ListingKit package verification**

Run:

```powershell
go test ./internal/listingkit/httpapi -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/httpapi/runtime_support.go internal/listingkit/httpapi/runtime_builder.go internal/listingkit/httpapi/temporal_runtime.go internal/listingkit/httpapi/bootstrap_test.go internal/listingkit/httpapi/runtime_support_test.go
git commit -m "feat: add listingkit runtime support builder"
```

---

## Task 2: Reduce app-layer ListingKit support shaping to app-local prerequisites only

**Files:**
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/listingkit_feature_builder.go`
- Modify: `internal/app/httpapi/listingkit_temporal_worker.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`
- Modify: `internal/app/httpapi/listingkit_feature_builder_test.go`

- [ ] **Step 1: Write failing app-layer tests for the narrower support boundary**

Add tests that describe the new boundary:

1. app-layer ListingKit helpers may prepare shared runtime support values
2. they may not create feature-owned repository/hook bundles directly

Suggested guardrail shape:

```go
func TestListingKitSupportDoesNotOwnRepositoryAndHookBundleBuilders(t *testing.T) {
	src, err := os.ReadFile("listingkit_support.go")
	require.NoError(t, err)
	content := string(src)
	require.NotContains(t, content, "func newListingKitBuildServiceRepositories(")
	require.NotContains(t, content, "func newListingKitBuildServiceHooks(")
}
```

- [ ] **Step 2: Run the focused tests to confirm those builders still live in app layer**

Run:

```powershell
go test ./internal/app/httpapi -run "TestListingKitSupportDoesNotOwnRepositoryAndHookBundleBuilders" -count=1
```

Expected: FAIL until the repository/hook bundle builders move out.

- [ ] **Step 3: Rewire app-layer ListingKit support helpers**

Refactor `internal/app/httpapi/listingkit_support.go` so it now only prepares:

- SDS sync service
- SDS baseline provider
- SHEIN cookie-store
- runtime flags
- shared runtime collaborators already owned by app/runtime

Then have it pass those into the new feature-owned `listingkithttpapi.BuildRuntimeSupport(...)`.

The target is:

- app layer prepares **support values**
- ListingKit package prepares **feature-owned builder bundles**

- [ ] **Step 4: Update composition and side-entry flows to use the new support path**

Update:

- `composition_builder.go`
- `listingkit_feature_builder.go`
- `listingkit_temporal_worker.go`

so they all construct ListingKit through the same narrower app-layer handoff.

This is the key root-cause fix for app-layer runtime-input over-ownership.

- [ ] **Step 5: Re-run app-layer verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/httpapi/listingkit_support.go internal/app/httpapi/composition_builder.go internal/app/httpapi/listingkit_feature_builder.go internal/app/httpapi/listingkit_temporal_worker.go internal/app/httpapi/composition_builder_test.go internal/app/httpapi/listingkit_feature_builder_test.go
git commit -m "refactor: narrow app-owned listingkit support shaping"
```

---

## Task 3: Move ListingKit SDS support and remote-provider shaping behind feature-owned seams

**Files:**
- Modify: `internal/listingkit/httpapi/runtime_support.go`
- Modify: `internal/listingkit/httpapi/runtime_builder.go`
- Modify: `internal/listingkit/httpapi/temporal_runtime.go`
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/app/httpapi/runtime_deps_test.go`
- Modify: `internal/listingkit/httpapi/runtime_support_test.go`

- [ ] **Step 1: Write failing tests for SDS support ownership**

Add focused tests that prove the feature-owned runtime support path can accept app-layer prepared SDS collaborators without the app layer shaping the whole runtime input object.

Cover:

1. SDS sync service passes through support cleanly
2. SDS baseline provider passes through support cleanly
3. nil SDS support still degrades safely

- [ ] **Step 2: Run the focused tests to confirm app layer still shapes too much of the SDS path**

Run:

```powershell
go test ./internal/listingkit/httpapi -run "TestBuildRuntimeSupportCarriesSDSCollaborators" -count=1
```

Expected: FAIL until the runtime support contract carries these collaborators cleanly.

- [ ] **Step 3: Expand the feature-owned runtime support contract**

Add support fields so `RuntimeSupport` can carry:

- `SDSSyncService`
- `SDSBaselineRemoteProvider`
- `SDSLoginStatusProvider`

without app-layer code rebuilding the entire ListingKit runtime input shape each time.

This should let app-layer code hand over collaborators instead of hand over a fully-shaped feature contract.

- [ ] **Step 4: Simplify app-layer helpers around SDS support**

After the contract expands:

- remove redundant shaping in `listingkit_support.go`
- keep only the truly app-owned bootstrap logic there
- make `runtime_deps` tests assert that app-layer ListingKit support is now mostly state/bootstrap preparation, not feature contract shaping

- [ ] **Step 5: Re-run package verification**

Run:

```powershell
go test ./internal/listingkit/httpapi -count=1
go test ./internal/app/httpapi -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/httpapi/runtime_support.go internal/listingkit/httpapi/runtime_builder.go internal/listingkit/httpapi/temporal_runtime.go internal/app/httpapi/listingkit_support.go internal/app/httpapi/runtime_deps_test.go internal/listingkit/httpapi/runtime_support_test.go
git commit -m "refactor: move listingkit sds support shaping into feature package"
```

---

## Task 4: Lock the new ListingKit runtime-ownership boundary

**Files:**
- Create: `internal/app/httpapi/phase3a_listingkit_boundary_test.go`
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/listingkit/httpapi/runtime_support_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_live_test.go`

- [ ] **Step 1: Add a narrow boundary test for app-layer ListingKit ownership**

Create `internal/app/httpapi/phase3a_listingkit_boundary_test.go`:

```go
func TestListingKitSupportDoesNotOwnRepositoryAndHookBundleBuilders(t *testing.T) {
	src, err := os.ReadFile("listingkit_support.go")
	require.NoError(t, err)

	content := string(src)
	require.NotContains(t, content, "func newListingKitBuildServiceRepositories(")
	require.NotContains(t, content, "func newListingKitBuildServiceHooks(")
	require.NotContains(t, content, "BuildServiceRepositories{")
	require.NotContains(t, content, "BuildServiceHooks{")
}
```

This is intentionally narrow: it only locks the ownership transfer we want from app layer to feature package.

- [ ] **Step 2: Run the focused boundary test to verify it fails before cleanup**

Run:

```powershell
go test ./internal/app/httpapi -run TestListingKitSupportDoesNotOwnRepositoryAndHookBundleBuilders -count=1
```

Expected: FAIL until the old builders are fully moved out.

- [ ] **Step 3: Update ListingKit side-entry tests to reflect the new ownership path**

Make sure the E2E-style ListingKit side-entry tests still pass while using the narrowed support path. The goal is to prove:

- HTTP runtime
- side-entry runtime
- Temporal standalone entry

all consume the same feature-owned runtime-support boundary.

- [ ] **Step 4: Run full verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/... -count=1
go test ./internal/app/runtime -count=1
go test ./cmd/listingkit-temporal-worker -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/httpapi/phase3a_listingkit_boundary_test.go internal/app/httpapi/listingkit_support.go internal/listingkit/httpapi/runtime_support_test.go internal/app/httpapi/e2e_listingkit_sds_test.go internal/app/httpapi/e2e_listingkit_sds_live_test.go
git commit -m "test: lock listingkit runtime ownership boundary"
```

---

## Self-Review

### Spec coverage

- The `Phase 3` scope recommendation says the next hotspot should be ListingKit runtime input ownership rather than more Temporal abstraction. This plan follows that recommendation exactly.
- The plan moves the remaining app-owned ListingKit runtime-input shaping into feature-owned builders without reopening unrelated framework layers.

### Reuse check

- The plan reuses existing `BuildRuntimeModule(...)`, `BuildTemporalRuntime(...)`, and `BuildService(...)` seams instead of inventing a new bootstrap system.
- The plan keeps using the existing app/runtime and ListingKit support collaborators rather than redesigning adapters prematurely.

### Root-cause check

- The issue is not just that `listingkit_support.go` is long.
- The issue is that app-layer code still owns the full feature runtime contract for ListingKit.
- This plan fixes that by moving builder-bundle shaping and runtime-support ownership into `internal/listingkit/httpapi`.

### Placeholder scan

- No `TODO` or vague “figure out later” items remain.
- Each task includes concrete files, verification commands, and atomic commit boundaries.

### Scope discipline

- This slice does not attempt a generic workflow framework.
- This slice does not redesign ListingKit service internals.
- This slice only transfers runtime-input ownership to the feature package and locks that boundary in tests.
