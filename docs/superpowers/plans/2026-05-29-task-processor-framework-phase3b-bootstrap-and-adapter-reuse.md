# Task Processor Framework Phase 3B Bootstrap Decomposition And Adapter Reuse Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Execute both next hotspots from the `Phase 3A` checkpoint in one ordered slice:

1. decompose `internal/listingkit/httpapi/bootstrap.go`
2. extract only the adapter-oriented bootstrap reuse points that now have real evidence, without inventing a speculative generic runtime framework

**Architecture:** Treat `internal/listingkit/httpapi/bootstrap.go` as the primary decomposition target and use that refactor to reveal stable bootstrap seams. Reuse only the parts that already show repeated or clearly transferable bootstrap behavior:

- Redis-backed cookie-store bootstrap
- SDS client/bootstrap wiring
- SDS sync-service bootstrap

Do **not** jump straight to a global runtime manifest or a giant “bootstrap platform” package.

**Tech Stack:** Go, ListingKit HTTP runtime builders, app/httpapi runtime bootstrap, existing SDS and SHEIN integrations, existing runtime and E2E tests

**Out of Scope For This Slice:**

- redesigning `internal/listingkit/service.go`
- making every feature adopt a unified support manifest
- genericizing Temporal runtime just for symmetry
- moving all config/bootstrap logic out of app layer in one pass
- changing business behavior of ListingKit workflows

---

## Why These Two Should Be Done Together

`Phase 3A` solved the ownership problem around ListingKit runtime inputs, but it left two adjacent complexity hotspots:

1. [internal/listingkit/httpapi/bootstrap.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap.go:1) is still a 989-line mixed assembly file
2. [internal/app/httpapi/listingkit_support.go](/D:/code/task-processor/internal/app/httpapi/listingkit_support.go:1) still contains app-owned bootstrap helpers that now look reusable at the adapter boundary

Doing only the bootstrap split would leave the newly clarified bootstrap seams trapped inside ListingKit.

Doing only adapter reuse now would be too speculative, because the stable seam boundaries are still obscured by the size and mixture inside `bootstrap.go`.

The right sequence is:

1. split ListingKit bootstrap by responsibility
2. extract only the app/bootstrap helpers whose reuse boundary becomes obvious after the split

---

## Root Cause This Slice Addresses

The root problem is not just that `bootstrap.go` is large.

The real problem is that ListingKit runtime assembly still mixes several layers of concern in one file:

1. repository assembly
2. service-config assembly
3. module/runtime assembly
4. Temporal workflow client wiring
5. handler dependency shaping

Because those concerns are mixed, it is hard to tell which bootstrap logic is:

- feature-owned and should stay in ListingKit
- app-owned and should stay in runtime bootstrap
- adapter-owned and should be reused elsewhere

This plan fixes that in the correct order.

---

## Target Outcome

At the end of `Phase 3B`:

- `internal/listingkit/httpapi/bootstrap.go` is materially smaller and focused on orchestration
- repository assembly, service-config assembly, and runtime/module assembly live in separate feature-owned files
- app-layer helper reuse candidates around SDS and Redis bootstrap become explicit and small
- adapter-oriented bootstrap helpers are extracted only where the boundary is already proven
- no business behavior changes

---

## Task 1: Split repository assembly out of ListingKit bootstrap

**Files:**
- Create: `internal/listingkit/httpapi/bootstrap_repositories.go`
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`

- [ ] **Step 1: Write failing boundary tests for repository assembly extraction**

Lock the intended decomposition boundary with a narrow source-level test or function-level test:

1. repository assembly helpers still exist
2. `bootstrap.go` no longer owns the repository assembly types and builder functions directly

Focus on these symbols moving out:

- `builtRepositories`
- `builtCoreRepositories`
- `builtLateCoreRepositories`
- `buildCoreRepositories`
- `buildLateCoreRepositories`
- `buildAdminRepositories`
- `assembleRepositories`
- `buildRepositories`

- [ ] **Step 2: Extract repository assembly types and helpers**

Move the repository-assembly data structures and functions into `bootstrap_repositories.go`.

Keep the API unchanged for callers:

- `buildRepositories(...)`
- repository assembly behavior

This is a file-level decomposition only, not a logic rewrite.

- [ ] **Step 3: Re-run focused ListingKit tests**

Run:

```powershell
go test ./internal/listingkit/httpapi -count=1
```

- [ ] **Step 4: Commit**

```bash
git add internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_repositories.go internal/listingkit/httpapi/bootstrap_test.go
git commit -m "refactor: split listingkit repository assembly"
```

---

## Task 2: Split service-config and runtime assembly out of ListingKit bootstrap

**Files:**
- Create: `internal/listingkit/httpapi/bootstrap_service_config.go`
- Create: `internal/listingkit/httpapi/bootstrap_runtime.go`
- Modify: `internal/listingkit/httpapi/bootstrap.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Modify: `internal/listingkit/httpapi/temporal_runtime_test.go`

- [ ] **Step 1: Write failing tests for service/runtime assembly boundaries**

Cover the feature-owned seams that should survive extraction:

1. `BuildService(...)` still returns the same runtime bundle shape
2. `BuildModule(...)` still returns the same module runtime shape
3. Temporal runtime still builds through the same feature-owned service/runtime path

- [ ] **Step 2: Extract service-config assembly**

Move these shapes and helpers into `bootstrap_service_config.go`:

- `buildListingKitServiceConfigInput`
- `buildListingKitServiceConfig`
- `buildListingKitCoreDependencies`
- `buildListingKitAssetDependencies`
- `buildListingKitSheinDependencies`
- `buildListingKitWorkflowDependencies`

- [ ] **Step 3: Extract runtime/module assembly**

Move these shapes and helpers into `bootstrap_runtime.go`:

- `serviceRuntimeModules`
- `serviceRuntimeAssembly`
- `moduleRuntimeAssembly`
- `assembleModuleRuntime`
- `createModuleRuntime`
- `buildModuleRuntime`
- `buildServiceRuntimeModules`
- `assembleServiceRuntime`
- `buildServiceRuntime`

- [ ] **Step 4: Re-run ListingKit runtime verification**

Run:

```powershell
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/httpapi/bootstrap.go internal/listingkit/httpapi/bootstrap_service_config.go internal/listingkit/httpapi/bootstrap_runtime.go internal/listingkit/httpapi/bootstrap_test.go internal/listingkit/httpapi/temporal_runtime_test.go
git commit -m "refactor: split listingkit runtime assembly"
```

---

## Task 3: Extract adapter-oriented SDS bootstrap reuse

**Files:**
- Create: `internal/sds/httpbootstrap/support.go`
- Create: `internal/sds/httpbootstrap/support_test.go`
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/app/httpapi/modules_sds_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_live_test.go`

- [ ] **Step 1: Write failing tests for reusable SDS bootstrap helpers**

Target the parts that already look adapter-like:

1. building SDS client config from app config
2. bootstrapping SDS sync service with auth-state reporting
3. bootstrapping baseline remote provider

The goal is to move helper logic without changing behavior.

- [ ] **Step 2: Extract SDS bootstrap helpers into an adapter-facing support package**

Create a narrow package such as:

- `internal/sds/httpbootstrap`

It should own things like:

- bootstrapping sync service
- bootstrapping baseline remote provider
- any glue that is no longer ListingKit-specific after `Phase 3A`

Keep app-layer code responsible only for choosing when to call those helpers.

- [ ] **Step 3: Rewire ListingKit support to consume the extracted SDS helpers**

Update `listingkit_support.go` so its SDS path becomes:

- app/runtime chooses bootstrap timing
- SDS helper package performs bootstrap mechanics
- ListingKit still consumes results through `RuntimeSupport`

- [ ] **Step 4: Re-run app-layer and ListingKit SDS verification**

Run:

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/... -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/sds/httpbootstrap/support.go internal/sds/httpbootstrap/support_test.go internal/app/httpapi/listingkit_support.go internal/app/httpapi/modules_sds_test.go internal/app/httpapi/e2e_listingkit_sds_test.go internal/app/httpapi/e2e_listingkit_sds_live_test.go
git commit -m "refactor: extract sds bootstrap support helpers"
```

---

## Task 4: Extract adapter-oriented Redis cookie-store bootstrap reuse

**Files:**
- Create: `internal/sheinlogin/bootstrap/store_support.go`
- Create: `internal/sheinlogin/bootstrap/store_support_test.go`
- Modify: `internal/app/httpapi/listingkit_support.go`
- Modify: `internal/sheinlogin/bootstrap/build.go`
- Modify: `internal/app/httpapi/phase3a_listingkit_boundary_test.go`

- [ ] **Step 1: Write failing tests for Redis cookie-store bootstrap reuse**

Cover:

1. lazy Redis store creation from config
2. no-op behavior when Redis config is absent
3. graceful degradation with logging on Redis init failure

- [ ] **Step 2: Extract cookie-store bootstrap helper**

Create a narrow helper in `internal/sheinlogin/bootstrap` that owns:

- building a `RedisStore` from config
- nil-safe behavior for missing config
- closer wiring output

Do **not** move ListingKit support-state caching into that package. The helper should only own bootstrap mechanics.

- [ ] **Step 3: Rewire ListingKit support and any existing SHEIN login bootstrap callsites**

Update app-layer ListingKit support to use the extracted helper.

If `sheinlogin/bootstrap/build.go` can use the same helper cleanly, move it too. That is the concrete evidence-based reuse case we already have.

- [ ] **Step 4: Re-run targeted verification**

Run:

```powershell
go test ./internal/sheinlogin/... -count=1
go test ./internal/app/httpapi -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/sheinlogin/bootstrap/store_support.go internal/sheinlogin/bootstrap/store_support_test.go internal/app/httpapi/listingkit_support.go internal/sheinlogin/bootstrap/build.go internal/app/httpapi/phase3a_listingkit_boundary_test.go
git commit -m "refactor: extract shein redis bootstrap support"
```

---

## Task 5: Lock the new decomposition and reuse boundaries

**Files:**
- Create: `internal/listingkit/httpapi/phase3b_bootstrap_boundary_test.go`
- Modify: `internal/app/httpapi/phase3a_listingkit_boundary_test.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Modify: `internal/app/httpapi/runtime_deps_test.go`

- [ ] **Step 1: Add narrow source and behavior guardrails**

Lock two things:

1. `bootstrap.go` no longer regresses into a giant mixed assembly file for the extracted seams
2. app-layer ListingKit support still consumes adapter/bootstrap helpers instead of re-growing direct bootstrap mechanics

- [ ] **Step 2: Run full verification**

Run:

```powershell
go test ./internal/listingkit/httpapi ./internal/listingkit/... -count=1
go test ./internal/app/httpapi -count=1
go test ./internal/app/runtime -count=1
go test ./internal/sheinlogin/... -count=1
go test ./cmd/listingkit-temporal-worker -count=1
```

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/httpapi/phase3b_bootstrap_boundary_test.go internal/app/httpapi/phase3a_listingkit_boundary_test.go internal/listingkit/httpapi/bootstrap_test.go internal/app/httpapi/runtime_deps_test.go
git commit -m "test: lock bootstrap decomposition and reuse boundaries"
```

---

## Self-Review

### Spec coverage

- This plan follows both recommended next hotspots from the `Phase 3A` checkpoint.
- It keeps the order disciplined: decompose ListingKit first, then extract only proven reuse.

### Reuse check

- It avoids inventing a generic runtime framework.
- It extracts only two reuse candidates that already show real pressure:
  - SDS bootstrap
  - Redis cookie-store bootstrap

### Root-cause check

- The problem is not only file length.
- The problem is mixed ownership and hidden seam boundaries.
- This plan exposes those seams first, then extracts reuse from the seams that stabilize.

### Scope discipline

- No service rewrite
- No speculative global manifest
- No forced generalization without a second real caller
