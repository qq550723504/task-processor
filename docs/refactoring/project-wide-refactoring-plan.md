# Task Processor Project-wide Refactoring Plan

> Scope: this document captures a project-level refactoring direction for `task-processor`, beyond the existing ListingKit-only restructuring notes. It is intended to guide incremental refactoring without forcing a high-risk rewrite.

## 1. Current Diagnosis

The project is no longer a small task processor. It has grown into a product listing automation platform with several runtime and business capabilities:

- HTTP API runtime based on Gin.
- Background and workflow execution through Temporal.
- Persistence through GORM with Postgres / SQLite support.
- Redis, RabbitMQ, S3/object storage, OpenAI, Playwright, metrics, and authorization integrations.
- A dedicated ListingKit UI under `web/listingkit-ui`.
- Platform-specific listing, publishing, workspace, and asset workflows for marketplaces such as SHEIN, Amazon, TEMU, and Walmart.

The most important architecture issue is not simply directory naming. The main issue is that several boundaries have become blurred:

- `internal/listingkit` is still the largest complexity center.
- Platform-specific marketplace rules can drift back into ListingKit facade code.
- Runtime assembly, HTTP API wiring, workflow orchestration, marketplace logic, product facts, asset logic, and external integrations are not consistently separated.
- Some parts of the project already moved in the right direction, such as `catalog`, `asset`, `internal/marketplace/shein/publishing`, and `internal/marketplace/shein/workspace`, but the pattern is not yet applied consistently across the whole codebase.

## 2. Target Architecture Style

Use a modular monolith architecture first. Do not split into microservices until package boundaries are stable.

The target shape should be:

```text
cmd/
  product-listing-api/
  product-listing-worker/
  listingkit-temporal-worker/
  tools/

internal/
  app/
    httpapi/
    worker/
    runtime/

  platform/
    config/
    logging/
    metrics/
    authz/
    database/
    redis/
    queue/
    temporal/
    objectstore/

  integration/
    openai/
    s3/
    playwright/
    shein/
    amazon/
    temu/
    walmart/

  product/
    catalog/
    asset/
    image/
    ai/

  listing/
    task/
    workflow/
    preview/
    export/
    revision/
    submission/
    studio/
    settings/

  marketplace/
    shein/
      publishing/
      workspace/
      model/
      api/
    amazon/
      publishing/
      workspace/
      model/
      api/
    temu/
      publishing/
      workspace/
      model/
      api/
    walmart/
      publishing/
      workspace/
      model/
      api/

  shared/
    errors/
    timeutil/
    pagination/
    validation/

web/
  listingkit-ui/

docs/
  architecture/
  refactoring/
  product/
  api/

scripts/
deployments/
```

This is a destination map, not a first-step migration plan. The codebase should move toward it incrementally.

## 3. Boundary Rules

### 3.1 Dependency Direction

Allowed high-level dependency direction:

```text
cmd
  -> app
  -> business modules
  -> domain modules
  -> platform / integration interfaces
```

More concretely:

```text
cmd/product-listing-api
  -> internal/app/httpapi
  -> internal/listingkit or internal/listing
  -> internal/catalog / internal/asset / internal/marketplace/*
  -> internal/platform / internal/integration through interfaces
```

Forbidden dependency direction:

```text
catalog -> listingkit
asset -> listingkit
marketplace/shein -> listingkit
marketplace/shein/publishing -> listingkit
marketplace/shein/workspace -> listingkit
infra/openai -> listingkit
repository -> httpapi
domain -> gin
domain -> gorm
domain -> temporal
```

These rules are intended to reduce Go import cycles and prevent business logic from drifting into runtime or facade layers.

### 3.2 App Layer Rule

`internal/app/*` should own runtime assembly only:

- Config loading.
- Dependency construction.
- Route registration.
- Worker registration.
- Wiring services to handlers.

It should not own business rules.

### 3.3 ListingKit Rule

`internal/listingkit` should become a compatibility facade and orchestration surface, not the home for all business rules.

Allowed responsibilities:

- Task lifecycle and orchestration.
- Workflow entrypoints.
- Request normalization.
- Persistence coordination.
- Preview/export aggregation.
- Revision/history facade.
- API-facing shell models.
- Cross-platform listing task concepts.

Avoid adding:

- New SHEIN category, attribute, pricing, or publishing rules.
- New SHEIN editor, repair, revision, or workspace rules.
- New platform-specific business rules that can live under marketplace-specific packages.

### 3.4 Marketplace Rule

Platform-specific behavior belongs in marketplace packages.

Recommended placement:

| Change type | Preferred home |
| --- | --- |
| SHEIN category / attribute / sale-attribute publishing | `internal/marketplace/shein/publishing` |
| SHEIN inspection / editor / repair / revision UX | `internal/marketplace/shein/workspace` |
| Amazon publishing behavior | `internal/amazon` now; later `internal/marketplace/amazon/publishing` |
| TEMU publishing behavior | `internal/temu` now; later `internal/marketplace/temu/publishing` |
| Walmart publishing behavior | current Walmart package now; later `internal/marketplace/walmart/publishing` |
| Product facts | `internal/catalog` or later `internal/product/catalog` |
| Visual assets | `internal/asset` or later `internal/product/asset` |
| Listing task flow | `internal/listingkit` now; later `internal/listing/*` |

## 4. Refactoring Phases

## Phase 0: Baseline and Safety Net

Goal: make refactoring measurable and reversible.

Deliverables:

- `docs/refactoring/dependency-baseline.md`
- `docs/refactoring/package-map.md`
- `docs/refactoring/test-baseline.txt`
- `docs/refactoring/coverage-baseline.out`

Suggested commands:

```bash
go test ./...
go test ./... -coverprofile=docs/refactoring/coverage-baseline.out
go list ./... > docs/refactoring/packages-baseline.txt
go mod graph > docs/refactoring/mod-graph-baseline.txt
```

Also capture:

- Package count.
- Largest packages by file count.
- Largest files by line count.
- Largest structs and constructors.
- Packages importing `internal/listingkit`.
- Packages imported by `internal/listingkit`.
- Known unstable tests.

Do not start large file moves before this baseline exists.

## Phase 1: Define and Enforce Project Boundaries

Goal: document the target dependency direction and create guardrails.

Deliverables:

- `docs/architecture/project-boundaries.md`
- Optional dependency check script under `scripts/`.

The boundary document should answer:

- What belongs in `app`?
- What belongs in `listingkit`?
- What belongs in `catalog` and `asset`?
- What belongs in platform-specific packages?
- What belongs in `platform` / `integration`?
- Which import directions are forbidden?

A lightweight script can fail CI or produce warnings when forbidden imports are introduced.

## Phase 2: Modularize `internal/listingkit`

Goal: reduce the root `internal/listingkit` surface and split by business capability.

Preferred module shape:

```text
internal/listingkit/
  service.go
  model.go
  errors.go

  task/
  workflow/
  preview/
  export/
  revision/
  submission/
  studio/
  settings/
  store/
  api/
```

Recommended order:

1. `preview`
2. `submission`
3. `revision/history`
4. `studio`
5. `workflow`
6. `task lifecycle`

Do not start by moving everything. Move one bounded capability at a time and run tests after each migration.

### 2.1 Preview Refactor

Current issue: preview building is an aggregation hotspot and contains platform-specific branching.

Target:

```text
internal/listingkit/preview/
  service.go
  base.go
  platform.go
  amazon.go
  shein.go
  temu.go
  walmart.go
```

Introduce a platform preview adapter pattern:

```go
type PlatformPreviewBuilder interface {
    Platform() string
    Build(ctx context.Context, input PlatformPreviewInput) (PlatformPreviewOutput, error)
}
```

The base preview service should own:

- Status and timestamp fields.
- Generic overview.
- Catalog and asset attachment.
- Revision history metadata.
- Selected platform filtering.

Platform builders should own only platform-specific preview payloads.

### 2.2 Submission Refactor

Current issue: submit, retry, recovery, execution state, direct submit, Temporal lifecycle/flow/persistence/refresh collaborators, and submit locks are related but spread across the larger ListingKit service object.

Target:

```text
internal/listing/submission/
  action_record.go
  attempt_finalize.go
  attempt_record.go
  confirm_remote_state.go
  event_history.go
  inflight_state.go
  refresh_guard.go
  refresh_selection.go
  remote_sync.go
  submit_error.go

internal/marketplace/shein/publishing/
  remote_record_policy.go
  submission_projection.go
  submission_remote.go

internal/listingkit/
  service_submit*.go
  task_submission_*.go
  task_temporal_submission_*_service.go
```

Expose a small service interface:

```go
type Service interface {
    Submit(ctx context.Context, taskID string, req SubmitRequest) (*SubmitResult, error)
    Recover(ctx context.Context, taskID string) error
    GetState(ctx context.Context, taskID string) (*SubmitState, error)
}
```

Direction note:

- generic listing submission mechanics should move toward `internal/listing/submission`
- marketplace-specific submission rules should move toward `internal/marketplace/*/publishing`
- `internal/listingkit` should shrink into orchestration, compatibility, and API-shell responsibilities rather than becoming the long-term home of a large generic `submission` package

### 2.3 Service Object Slimming

The root ListingKit service should not hold every dependency directly. Replace the large struct with grouped internal facades:

```go
type service struct {
    task       *taskFacade
    workflow   *workflowFacade
    preview    *previewFacade
    revision   *revisionFacade
    studio     *studioFacade
    submission *submissionFacade
    settings   *settingsFacade
    shein      *sheinFacade
}
```

This can be done before changing public APIs.

## Phase 3: Stabilize `app/httpapi` and Runtime Assembly

Goal: make `internal/app/httpapi` an assembly layer only.

Rules:

- Handlers should call service interfaces.
- Bootstrap files should assemble dependencies.
- Runtime support should build infrastructure adapters.
- Business rules should live in listing, product, or marketplace modules.

Recommended structure:

```text
internal/app/httpapi/
  bootstrap/
  routes/
  middleware/
  modules/
```

Avoid direct business logic inside route registration or runtime build files.

## Phase 4: Normalize Marketplace Boundaries

Goal: apply the already-emerging SHEIN pattern to all marketplaces.

Current SHEIN direction is good:

- Publishing rules live in `internal/marketplace/shein/publishing`.
- Workspace rules live in `internal/marketplace/shein/workspace`.
- ListingKit keeps only facade bridges.

Next steps:

1. Freeze SHEIN placement rules.
2. Prevent new SHEIN rules from being added to root `listingkit`.
3. Start aligning TEMU publishing/workspace logic to the same shape.
4. Then align Amazon and Walmart.

Do not rename all marketplace directories at once. First establish adapter interfaces and reduce imports from ListingKit into platform internals.

## Phase 5: Consolidate Infrastructure and Integrations

Goal: make external dependencies replaceable and keep domain code clean.

Recommended distinction:

```text
platform = runtime infrastructure used by the application
integration = concrete external system adapters
```

Examples:

| Concern | Target area |
| --- | --- |
| Config | `internal/platform/config` |
| Logging | `internal/platform/logging` |
| Metrics | `internal/platform/metrics` |
| Authorization | `internal/platform/authz` |
| Database connection | `internal/platform/database` |
| Redis connection | `internal/platform/redis` |
| Queue connection | `internal/platform/queue` |
| Temporal client/worker bootstrap | `internal/platform/temporal` |
| Object storage abstraction | `internal/platform/objectstore` |
| OpenAI client adapter | `internal/integration/openai` |
| S3 concrete adapter | `internal/integration/s3` |
| Playwright browser automation | `internal/integration/playwright` |
| Marketplace API clients | `internal/integration/{shein,amazon,temu,walmart}` |

Domain and listing modules should depend on small interfaces, not concrete clients.

## Phase 6: API Contract and Frontend Boundary

Goal: keep `web/listingkit-ui` independent but contract-driven.

Recommended additions:

```text
docs/api/listingkit-openapi.yaml
web/listingkit-ui/src/lib/api/generated/
```

Rules:

- Backend DTO changes should be reflected in API contract updates.
- Frontend should not guess backend response shapes.
- Generated types are preferred for stable API surfaces.
- Handwritten frontend API clients should be gradually reduced.

## 5. Migration Strategy

Use small PRs. Each PR should do one of the following:

- Move one bounded group of files.
- Introduce one interface and adapter.
- Slim one constructor.
- Extract one platform-specific builder.
- Add one boundary test or dependency check.

Avoid PRs that combine:

- Large file moves.
- Behavior changes.
- Directory renames.
- New features.
- Dependency updates.

Recommended PR sequence:

1. Add project boundary documentation.
2. Add dependency baseline script.
3. Extract ListingKit preview package.
4. Extract platform preview builders.
5. Slim ListingKit service constructor.
6. Extract submission service package.
7. Extract revision/history package.
8. Extract studio package.
9. Move runtime-only assembly code under clearer app/runtime grouping.
10. Add import boundary checks.
11. Normalize TEMU package placement.
12. Normalize Amazon/Walmart package placement.

## 6. What Not To Do Yet

Do not split into microservices yet.

Reason: package boundaries are not stable enough. Splitting now would turn code complexity into deployment complexity.

Do not rename every package at once.

Reason: this would create a huge PR, increase import churn, and make test failures hard to diagnose.

Do not use pure technical layering as the main design.

Avoid making everything look like:

```text
domain/
service/
repository/
handler/
```

The project complexity is mainly business workflow complexity, not CRUD layering complexity. Prefer business-capability packages with light internal layering.

Do not keep adding platform-specific rules to ListingKit.

Reason: this is the fastest path back to a large facade package and Go import cycles.

## 7. Success Metrics

Track these metrics over time:

- Root `internal/listingkit` file count.
- Largest package file count.
- Largest file line count.
- Number of packages importing `internal/listingkit`.
- Number of platform-specific files in root `internal/listingkit`.
- Test coverage for refactored packages.
- Number of direct imports from domain code to Gin/GORM/Temporal/OpenAI/S3 clients.
- Number of API DTOs covered by frontend generated types.

Suggested target milestones:

| Milestone | Target |
| --- | --- |
| M1 | Project boundaries documented and dependency baseline captured |
| M2 | `internal/listingkit` root file count reduced materially |
| M3 | Preview building moved to adapter-based package |
| M4 | Submission logic consolidated |
| M5 | App HTTP bootstrap free of business rules |
| M6 | SHEIN placement rules enforced |
| M7 | TEMU starts following the marketplace boundary model |
| M8 | Infrastructure clients mostly hidden behind interfaces |

## 8. Immediate Next Actions

Recommended next three actions:

1. Add `docs/architecture/project-boundaries.md` with allowed and forbidden dependencies.
2. Create a dependency baseline script under `scripts/` that reports package imports and `internal/listingkit` root file count.
3. Start with the ListingKit preview refactor because it is a bounded aggregation hotspot and a natural place to introduce platform adapters.

## 9. Guiding Principle

The project should become:

```text
A modular monolith with clear business boundaries, stable runtime assembly, marketplace-specific rule ownership, and infrastructure hidden behind interfaces.
```

The safest first cut is still `internal/listingkit`, but the project-level target is broader: make ListingKit an orchestration and compatibility surface, make marketplace packages own marketplace rules, make product packages own product and asset facts, and make app/platform packages own runtime concerns.
