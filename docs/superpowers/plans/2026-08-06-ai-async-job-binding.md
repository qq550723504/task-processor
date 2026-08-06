# AI Async Job Provider Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist the Provider route selected for each active asynchronous Studio image job and reuse that route for every later query.

**Architecture:** Add a provider-neutral `AsyncJobBindingStore` contract and a GORM implementation backed by a new `ai_async_jobs` table. The Studio capability adapter writes bindings after active submissions and resolves them before active queries. The existing ListingKit routed image client receives an optional route-aware query seam; concrete Provider construction remains inside `internal/listingkit/httpapi`.

**Tech Stack:** Go, GORM, PostgreSQL, existing ListingKit AI interfaces, existing `aicapability` contracts, Go tests.

## Global Constraints

- Do not store prompts, images, raw Provider responses, API keys, cookies, or Authorization headers in `ai_async_jobs`.
- Preserve `legacy` and `shadow` behavior; only `active` uses persisted route bindings for submit/query.
- Use the existing shared database connection and `scope=all` migration path; do not add a second database or migration binary.
- Keep Provider-specific code below the existing ListingKit HTTP API seam; `internal/aicapability` remains Provider-neutral.
- Follow TDD: every production behavior change starts with a failing test and the targeted package test must pass before the next task.

---

### Task 1: Define the async binding contract and error semantics

**Files:**
- Create: `internal/aicapability/async_job.go`
- Modify: `internal/aicapability/errors.go`
- Test: `internal/aicapability/async_job_test.go`

**Interfaces:**
- Produces `aicapability.AsyncJobBinding`, `aicapability.AsyncJobBindingStore`, `ErrAsyncJobBindingNotFound`, and `ErrAsyncJobBindingConflict` for the persistence and adapter tasks.

- [ ] **Step 1: Write the failing tests**

Add tests that assert the exported binding contains only routing/lifecycle metadata, that the not-found and conflict errors are discoverable with `errors.Is`, and that an empty `JobID` is rejected by a small validation helper.

- [ ] **Step 2: Run the contract tests and verify the expected failure**

Run:

```powershell
go test ./internal/aicapability -run 'TestAsyncJobBinding'
```

Expected: FAIL because the binding type, store interface, errors, and validation helper do not exist.

- [ ] **Step 3: Implement the minimal contract**

Define:

```go
type AsyncJobBinding struct {
    JobID, TenantID, UserID, BusinessTaskID, TraceID string
    Capability Capability
    Operation Operation
    ProviderID, ModelID, RoutingKey, CredentialReference string
    PolicyVersion, ConfigurationVersion string
    SubmittedAt, UpdatedAt, ExpiresAt time.Time
    Status string
    LastErrorCategory ErrorCategory
}

type AsyncJobBindingStore interface {
    PutAsyncJobBinding(context.Context, AsyncJobBinding) error
    GetAsyncJobBinding(context.Context, string) (AsyncJobBinding, error)
    UpdateAsyncJobBindingStatus(context.Context, string, string, ErrorCategory) error
}
```

Add sentinel errors and `ValidateAsyncJobBinding` so callers can reject empty IDs before persistence.

- [ ] **Step 4: Run the contract tests and verify they pass**

Run the same `go test` command and expect PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/aicapability/async_job.go internal/aicapability/errors.go internal/aicapability/async_job_test.go
git commit -m "feat(ai): define async job route binding contract"
```

### Task 2: Persist bindings in GORM and include them in schema migration

**Files:**
- Create: `internal/aicapability/store/gorm_async_job_store.go`
- Test: `internal/aicapability/store/gorm_async_job_store_test.go`
- Modify: `internal/listingkit/httpapi/builders_repository_schema.go`
- Modify: `internal/app/runtime/listingkitschemamigrate/runtime.go`
- Modify: `internal/app/httpapi/adapters_schema_migration.go`

**Interfaces:**
- Consumes `aicapability.AsyncJobBindingStore` from Task 1.
- Produces `store.NewGormAsyncJobBindingStore`, `store.AutoMigrateAsyncJobBindings`, and the `ai_async_jobs` table.

- [ ] **Step 1: Write failing GORM tests**

Using the existing SQLite test pattern, cover table creation, trimming and round-trip metadata, idempotent writes with identical route metadata, conflict rejection without overwrite, missing lookup, status updates, and empty job ID rejection.

- [ ] **Step 2: Run the persistence tests and verify the expected failure**

Run:

```powershell
go test ./internal/aicapability/store -run 'TestGormAsyncJob'
```

Expected: FAIL because the store and row model do not exist.

- [ ] **Step 3: Implement the GORM store**

Create a private `asyncJobRow` with primary key `job_id`, indexes for tenant/time, capability/time, and provider/model, and no payload columns. Implement `PutAsyncJobBinding` with a read-before-create conflict check so an identical binding is idempotent and a different route returns `ErrAsyncJobBindingConflict`. Implement lookup and status update with UTC timestamps and normalized strings.

- [ ] **Step 4: Wire migration functions**

Add `AutoMigrateAsyncJobBindings(db)` and call it from the existing ListingKit repository schema migration and standalone ListingKit schema migration after the invocation ledger migration.

- [ ] **Step 5: Run the persistence and migration tests**

Run:

```powershell
go test ./internal/aicapability/store ./internal/listingkit/httpapi ./internal/app/runtime/listingkitschemamigrate
```

Expected: PASS, including assertions that `ai_async_jobs` exists after migration.

- [ ] **Step 6: Commit**

```powershell
git add internal/aicapability/store internal/listingkit/httpapi/builders_repository_schema.go internal/app/runtime/listingkitschemamigrate/runtime.go internal/app/httpapi/adapters_schema_migration.go
git commit -m "feat(ai): persist async job route bindings"
```

### Task 3: Add the route-aware query seam to the existing image clients

**Files:**
- Modify: `internal/listingkit/ai_contracts.go`
- Modify: `internal/listingkit/httpapi/ai_client_image_routing.go`
- Modify: `internal/listingkit/httpapi/ai_client_strict_image.go`
- Modify: `internal/listingkit/httpapi/ai_image_generator_adapter.go`
- Test: `internal/listingkit/httpapi/ai_clients_test.go`

**Interfaces:**
- Produces optional `listingkit.AIAsyncImageQueryByRoutingKey`:

```go
QueryImageGenerationForRoutingKey(context.Context, string, string) (*AIImageAsyncResult, error)
```

- The concrete routed client resolves the supplied routing key through its existing selector map; strict configured clients ignore the key because they are already bound to one credential reference.

- [ ] **Step 1: Write failing route-aware client tests**

Add a test to the routed client that asserts a query with `routingKey="nano"` reaches the nanobanana configured client, while a query with `routingKey="gpt-image-2"` reaches the GPT client. Add an adapter test proving the optional ListingKit interface forwards the key to the underlying routed client.

- [ ] **Step 2: Run the tests and verify the expected failure**

Run:

```powershell
go test ./internal/listingkit/httpapi -run 'Test.*RoutingKey.*Query|Test.*Query.*RoutingKey'
```

Expected: FAIL because the optional interface and routed query method do not exist.

- [ ] **Step 3: Implement the minimal seam**

Add the optional interface to `ai_contracts.go`. Implement `QueryImageGenerationForRoutingKey` on `listingKitRoutedImageClient`, resolving the requested route and calling that concrete client. Implement the adapter forwarding method with a typed optional interface assertion so legacy image generators that do not expose the seam remain valid.

- [ ] **Step 4: Run the route-aware tests and existing client tests**

Run:

```powershell
go test ./internal/listingkit/httpapi
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/ai_contracts.go internal/listingkit/httpapi/ai_client_image_routing.go internal/listingkit/httpapi/ai_client_strict_image.go internal/listingkit/httpapi/ai_image_generator_adapter.go internal/listingkit/httpapi/ai_clients_test.go
git commit -m "feat(ai): expose route-aware async image queries"
```

### Task 4: Integrate binding persistence and route recovery in the capability adapter

**Files:**
- Modify: `internal/listingkit/studio_ai_capability_adapter.go`
- Test: `internal/listingkit/studio_ai_capability_adapter_test.go`

**Interfaces:**
- Consumes `aicapability.AsyncJobBindingStore` and `listingkit.AIAsyncImageQueryByRoutingKey`.
- Produces active-mode submit/query behavior that uses one persisted route.

- [ ] **Step 1: Write failing adapter tests**

Extend the existing adapter test stubs and add tests for active submit writing a binding containing the route decision and returned job ID; active query loading that binding and invoking the route-aware seam with the same routing key; legacy and shadow queries not accessing the binding store; missing active binding falling back to legacy query and recording `ErrorUnknownRemoteState`; and conflicting or failed binding persistence returning an explicit unknown-state error.

- [ ] **Step 2: Run the adapter tests and verify the expected failure**

Run:

```powershell
go test ./internal/listingkit -run 'TestStudioAIImageCapabilityAdapter.*Async|TestStudioAIImageCapabilityAdapter.*Query'
```

Expected: FAIL because the adapter has no binding store or route-aware query path.

- [ ] **Step 3: Implement binding-aware submit/query**

Add `AsyncJobStore aicapability.AsyncJobBindingStore` to the adapter config and require it only for active mode. After a successful active submit with a non-empty job ID, persist the binding from the route decision and request identity. For active query, load the binding, convert it to a route decision for the invocation record, and call `QueryImageGenerationForRoutingKey` when available. On missing binding, use the legacy query and classify the record as `unknown_remote_state`; on store or route-aware seam failure, return the explicit error while preserving the existing recorder timeout behavior.

- [ ] **Step 4: Run adapter tests and the complete ListingKit package tests**

Run:

```powershell
go test ./internal/listingkit
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/studio_ai_capability_adapter.go internal/listingkit/studio_ai_capability_adapter_test.go
git commit -m "feat(ai): bind active async queries to submit route"
```

### Task 5: Wire the store through runtime dependencies

**Files:**
- Modify: `internal/app/httpapi/runtime_ai_capability.go`
- Modify: `internal/app/httpapi/adapters_ai_capability.go`
- Modify: `internal/app/httpapi/runtime_shared_deps.go`
- Modify: `internal/app/httpapi/runtime.go`
- Modify: `internal/listingkit/httpapi/runtime_builder.go`
- Modify: `internal/listingkit/httpapi/bootstrap_contracts.go`
- Modify: `internal/listingkit/httpapi/bootstrap_submit_module.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Test: `internal/app/httpapi/runtime_ai_capability_test.go`

**Interfaces:**
- Adds `AIAsyncJobStore aicapability.AsyncJobBindingStore` alongside the existing invocation recorder through `RuntimeDependencies` and `BuildServiceInput`.

- [ ] **Step 1: Write failing runtime wiring tests**

Add tests that legacy mode returns no recorder or async store, while shadow/active runtime dependencies create both stores from one shared DB connection. Add a bootstrap test that active mode rejects a missing async store and that shadow mode remains constructible without one.

- [ ] **Step 2: Run the tests and verify the expected failure**

Run:

```powershell
go test ./internal/app/httpapi ./internal/listingkit/httpapi -run 'Test.*AICapability|Test.*Async.*Store|Test.*Routing.*Mode'
```

Expected: FAIL because runtime structs and bootstrap wiring do not expose the new store.

- [ ] **Step 3: Implement shared runtime wiring**

Construct the GORM binding store from the same database handle as the invocation recorder, pass it through shared runtime dependencies, and add it to submit-module input. Require it only in active mode, then pass it into `StudioAIImageCapabilityAdapterConfig`.

- [ ] **Step 4: Run runtime and bootstrap tests**

Run:

```powershell
go test ./internal/app/httpapi ./internal/listingkit/httpapi
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/app/httpapi internal/listingkit/httpapi
git commit -m "feat(ai): wire async job binding store into runtime"
```

### Task 6: Full verification and migration evidence

**Files:**
- Modify: `docs/superpowers/plans/2026-08-06-ai-async-job-binding.md` (check off completed steps only)
- Test: all package tests through the repository command

- [ ] **Step 1: Run formatting and static checks**

Run:

```powershell
gofmt -w internal/aicapability/async_job.go internal/aicapability/store/gorm_async_job_store.go internal/listingkit/ai_contracts.go internal/listingkit/studio_ai_capability_adapter.go internal/listingkit/httpapi/ai_client_image_routing.go internal/listingkit/httpapi/ai_client_strict_image.go internal/listingkit/httpapi/ai_image_generator_adapter.go
go vet ./internal/aicapability/... ./internal/listingkit/... ./internal/app/httpapi/...
```

Expected: exit code 0.

- [ ] **Step 2: Run the full Go suite**

Run:

```powershell
go test ./...
```

Expected: exit code 0 with no failed packages.

- [ ] **Step 3: Review the final diff and migration call sites**

Run:

```powershell
git diff --check
git diff --stat HEAD~5..HEAD
git grep -n "AutoMigrateAsyncJobBindings\|ai_async_jobs\|QueryImageGenerationForRoutingKey"
```

Confirm that all schema migration entry points include `ai_async_jobs`, all active query paths use the binding store, and no sensitive payload fields were added.

- [ ] **Step 4: Commit the plan checklist update**

```powershell
git add docs/superpowers/plans/2026-08-06-ai-async-job-binding.md
git commit -m "docs: mark async job binding verification complete"
```
