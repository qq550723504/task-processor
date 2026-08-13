# Platform-aware Asset Refactoring Implementation Plan

> **Authority:** Historical implementation record. The platform-aware asset refactor is complete. This file no longer defines the active execution queue; use `docs/refactoring/current-refactoring-status.md` and GitHub issue #137 for current order.
>
> **For agentic workers:** Retain the checklist as execution evidence. Do not re-execute it as a current plan; use the active current-state document and backlog authority above.

**Goal:** Make ListingKit's image and asset flow explicitly target-platform-aware, then use that vertical slice to establish reusable configuration, event, and HTTP-contract boundaries.

**Architecture:** Preserve the modular monolith and existing Product Sourcing/SHEIN boundaries. `internal/listingkit` remains an orchestration and compatibility facade; product-image behavior moves behind narrow product/asset-facing contracts, while marketplace-specific validation receives an explicit target rather than inferring or defaulting one. Runtime configuration is constructed in application/bootstrap packages and injected into feature packages.

**Tech Stack:** Go 1.26, Gin, GORM, Temporal, RabbitMQ, Next.js/TypeScript, Vitest, Go standard testing.

## Global constraints

- Do not create a new service, scheduler, queue owner, or workflow engine.
- Do not combine behavior changes with broad package renames or cosmetic file moves.
- `internal/app/*` remains runtime assembly; it may read the global configuration, domain packages may not add new direct dependencies on `internal/core/config`.
- A missing target platform is an input error. It must never silently select Amazon, SHEIN, or the first platform in an array.
- Preserve tenant/user AI identity and fail-closed resolver behavior; never restore a static default credential/model fallback.
- Preserve Temporal determinism and existing RabbitMQ acknowledgement/recovery semantics unless a task explicitly changes and tests them.
- Do not begin another product-source integration or a TEMU/Amazon workbench expansion in these PRs.
- Every temporary import-boundary exception must name an owner, reason, and retirement condition.

---

> Status: implementation complete; operational compatibility and product-flow gates remain, re-baselined on 2026-08-12.
>
> Calibrated against: `master` at `a463998b53ebb628a51b13db4292f463363e376a`.
>
> Supersedes: the 2026-07 queue that made next-source selection the next structural milestone. Product Sourcing closeout remains required product work, but it is not a reason to defer the contained asset-boundary correction below.

## 1. Current position

- The repository is a ListingKit-centred modular-monolith migration, not a candidate for a broad microservice split.
- `internal/listingkit` remains the complexity sink and still owns API, HTTP assembly, persistence, orchestration, and compatibility concerns.
- Product Sourcing foundations (`SourceEnvelope`, catalog/asset facts, and the ListingKit bridge) exist; the controlled 1688 path still needs business-flow acceptance evidence.
- SHEIN is the production-critical target path. TEMU/Amazon runtime assets remain maintained but their full workbenches are deferred.
- Image processing now requires explicit supported targets; multi-target requests execute and project results independently by target. Legacy scalar fields remain explicitly selected compatibility projections and no longer derive from input-array order or an Amazon fallback.
- ProductImage runtime configuration is now interpreted at the HTTP composition boundary and passed inward as typed options. Product-image business packages have an import-boundary guard against new `internal/core/config` dependencies.
- Existing import-boundary tests pass, but many exception allowlists show that structural migration must reduce real dependencies instead of adding further exemptions.

## 2. Sequencing decision (completed)

The completed structural sequence was **not** a large ListingKit extraction. It was the platform-aware asset slice:

```text
ListingKit GenerateRequest (one or more requested targets)
  -> explicit per-target image/asset request
  -> productimage processing and reusable asset persistence
  -> marketplace-specific validation for that exact target
  -> ListingKit aggregation of target-keyed results
```

This ordering removed the incorrect fallback before it could spread into new sources or platforms, and gave the configuration and API-contract work one stable, narrow interface to adopt. The remaining work is the operational gate in Task 3 and the independent product-flow gates below.

## 3. Work that remains parallel, not coupled

| Track | Required outcome | Boundary |
| --- | --- | --- |
| SHEIN stabilization | Focused evidence for pricing, readiness, publish/recovery, and production-sensitive changes | No package migration bundled with business-policy fixes |
| Product Sourcing MVP | One controlled `1688 -> SourceEnvelope -> facts -> ListingKit task -> preview/readiness` flow, with lineage and warnings | No new source until the result is recorded as closed or blocked |
| Platform-aware asset refactor | Explicit target selection, target-keyed image/asset outputs, and no Amazon/order fallback | The next structural PR sequence below |
| Baseline evidence | Exact command/CI output recorded for the commit under review | A failing environment gate is classified, never silently ignored |

## 4. Implementation queue

### Task 1: Make image-target selection explicit

**Status:** Complete — merged in [PR #138](https://github.com/qq550723504/task-processor/pull/138) (`8b212af53`). CI backend and frontend checks passed. Follow-up target-tagging and inventory-isolation fixes are included in the merged result.

**Files:**

- Modify: `internal/listingkit/model_request.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/workflow_requests.go`
- Modify: `internal/listingkit/workflow_standard_media_phase.go`
- Modify: `internal/listingkit/model_result.go`
- Modify: `internal/listingkit/assembler.go`
- Modify: `internal/listingkit/platform_payload_result_context.go` and every production consumer found by `rg -l 'result\.(ImageAssets|AssetBundle|AssetInventorySummary)' internal/listingkit -g '*.go'`
- Modify: `internal/productimage/domain/model.go`
- Modify: `internal/productimage/service_task.go`
- Modify: `internal/listingkit/workflow_assets_test.go`
- Modify: `internal/listingkit/workflow_scene_options_test.go`
- Modify: `internal/productimage/service_task_test.go`
- Create: `internal/listingkit/workflow_requests_test.go`

**Consumes:** `GenerateRequest.Platforms`, `listing/platform.NormalizeSupportedPlatforms`, and `productimage.ImageProcessRequest`.

**Produces:** an explicit target-platform value for every image-processing request and a target-keyed `ListingKitResult` projection for a multi-target listing request. The legacy scalar asset fields remain compatibility projections only; they are populated for a single target or for an explicitly selected compatibility target, never from array order.

- [x] Write failing tests that assert all of the following:
  - a request that enables image processing with an empty or unsupported target list is rejected before an image task is created;
  - a request for `["shein", "temu"]` creates independent requests whose targets are `shein` and `temu`, regardless of input order;
  - neither `nil` nor an empty target list produces an `amazon` request;
  - image validation receives the same explicit target that was selected for the corresponding asset output.
  - a multi-target result exposes both target-keyed outputs, while legacy scalar fields are populated only for one target or for an explicitly requested compatibility target.
- [x] Run the focused tests and confirm that the current fallback/first-element behavior fails those assertions:

  ```powershell
  go test ./internal/listingkit -run "Test.*Image.*Target|Test.*Workflow.*Request" -count=1
  go test ./internal/productimage -run "TestCreateProcessTask" -count=1
  ```

- [x] Add a target-specific request field or value object in `productimage/domain/model.go`. Keep the existing serialized `Marketplace` field readable only as a compatibility input during this PR; normalize it at the ingress boundary and reject disagreement with the explicit target.
- [x] Replace `detectImageMarketplace` in `workflow_requests.go` with a helper that returns one validated request per normalized target. It must return an error for missing targets and must not choose the first item as the sole target.
- [x] Add target-keyed result containers and lookup helpers in `model_result.go`. Update platform-payload consumers to ask for their own target rather than read the scalar compatibility fields.
- [x] Update `workflow_standard_media_phase.go` to process the returned requests independently and aggregate results by target. Record target-specific child-task/stage identity; do not overwrite one target's validation or asset result with another's.
- [x] Update `service.go` validation so an image-processing request cannot reach the workflow without at least one supported target. Requests that do not process images retain their existing non-image behavior.
- [x] Re-run the focused tests, then run:

  ```powershell
  go test ./internal/listingkit ./internal/productimage -count=1
  go test ./tests -count=1
  ```

- [x] Update `docs/architecture/listingkit-refactor-status.md` so it describes the implemented explicit-target behavior, not the retired fixed-Amazon implementation.
- [x] Commit only this behavior/documentation slice:

  ```text
  refactor(asset): require explicit image target platforms
  ```

### Task 2: Establish a product-image configuration seam

**Status:** Complete — merged in [PR #138](https://github.com/qq550723504/task-processor/pull/138) (`8b212af53`). CI backend and frontend checks passed.

**Files:**

- Modify: `internal/productimage/asset_publisher.go`
- Modify: `internal/productimage/httpapi/asset_publisher_builder.go`
- Modify: `internal/productimage/httpapi/image_pipeline_component_builder.go`
- Modify: `internal/productimage/httpapi/model_provider_builder.go`
- Modify: `internal/productimage/httpapi/runtime_builder.go`
- Modify: `internal/productimage/httpapi/runtime_module.go`
- Modify: `internal/productimage/*_test.go` tests covering the touched constructors
- Modify: `tests/import_boundaries_test.go`

**Consumes:** the explicit target produced by Task 1 and existing resolver-only provider interfaces.

**Produces:** typed product-image constructor options assembled in `productimage/httpapi`, with no new configuration reads from product-image business behavior.

- [x] Write failing constructor tests that build the product-image runtime from a small options struct and prove that missing tenant-bound model routing fails closed.
- [x] Run only those tests and confirm that the desired constructor cannot yet be used without a broad `*config.Config` dependency.
- [x] Define focused options in the HTTP/runtime builder for object storage, model/provider resolution, scene policy, and identity requirements. Do not pass the entire global config through a new wrapper.
- [x] Move configuration interpretation into the existing `productimage/httpapi/*builder.go` files. `asset_publisher.go` and other product-image behavior must receive interfaces or typed values, not read global configuration.
- [x] Add an import-boundary rule preventing new production imports of `internal/core/config` below `internal/productimage`, while allowing the existing HTTP/runtime composition package to construct options.
- [x] Re-run product-image tests, targeted architecture tests, and the existing resolver-only regression test:

  ```powershell
  go test ./internal/productimage/... -count=1
  go test ./internal/infra/clients/openai -run TestManagerFailsClosedForResolverOnlyImageWithoutIdentityConfiguration -count=1
  go test ./tests -count=1
  ```

- [x] Commit only the configuration seam:

  ```text
  refactor(productimage): inject runtime options at the HTTP boundary
  ```

### Task 3: Version the task-routing event at the messaging boundary

**Files:**

- Modify: `internal/domain/task/message.go`
- Modify: `internal/domain/task/normalize.go`
- Modify: `internal/domain/task/normalize_test.go`
- Modify: `internal/app/task/message_adapter.go`
- Modify: `internal/app/task/message_adapter_test.go`
- Modify: `internal/app/task/message_types.go`
- Modify: `internal/app/task/message_types_test.go`
- Modify: message producers/consumers found by `rg -l 'TaskMessage' internal -g '*.go'`
- Create: `docs/architecture/task-event-v2-migration.md`

**Consumes:** canonical source/target values from Task 1 and the current RabbitMQ task message path.

**Produces:** `TaskEventV2` with string task ID, explicit source/target platforms, schema version, and trace metadata; a bounded adapter for legacy messages.

**Boundary:** this task covers only RabbitMQ messages that carry a complete task payload. The Listing Control Plane's ID-only dispatch remains a separate `ListingDispatchSignal`: it causes the receiver to load a task from persistent state and does not carry source/target routing fields, so it is outside the V2 producer and compatibility window.

- [x] Write failing normalization tests for a V2 event with explicit source and target, and for rejection of missing/contradictory values.
- [x] Keep current legacy-message tests, but change them to assert conversion at the adapter boundary rather than allowing domain consumers to depend on the ambiguous `platform` field.
- [x] Introduce `TaskEventV2`; require `schemaVersion`, `taskId` as a string, `sourcePlatform`, and `targetPlatform` for new producer output.
- [x] Keep legacy decoding only in `app/task/message_adapter.go`. Remove the unknown-queue fallback to `amazon.crawler`; unknown routes must return a classified error/metric.
- [x] New producers publish only V2 events. Consumers retain legacy decoding for two release cycles; remove it only after zero legacy consumers and zero legacy-event observations for 14 consecutive days. Record the measurement and exit condition in `task-event-v2-migration.md`.
- [x] Re-run task/domain/consumer tests and one RabbitMQ integration path without changing acknowledgement ordering:

  ```powershell
  go test ./internal/domain/task ./internal/app/task ./internal/app/consumer -count=1
  go test ./tests -count=1
  ```

- [x] Commit the event migration separately:

  ```text
  refactor(task): introduce explicit platform routing event v2
  ```

### Task 4: Pilot a generated HTTP contract for the asset slice

**Status:** Complete — merged in [PR #138](https://github.com/qq550723504/task-processor/pull/138) (`8b212af53`). CI backend and frontend checks passed.

**Files:**

- Create: `docs/api/listingkit-asset.openapi.yaml`
- Modify: `internal/productimage/httpapi/handler.go`
- Modify: `internal/listingkit/api/handler_tasks.go` only if the public request/response changes
- Modify: `web/listingkit-ui/src/lib/api/client.ts`
- Modify: the affected files under `web/listingkit-ui/src/lib/api/`
- Create: generated artifacts only in `web/listingkit-ui/src/lib/api/generated/`; `openapi-typescript` is a frontend dev dependency and the deterministic generator command is documented in the OpenAPI header and Makefile
- Modify: `Makefile`

**Consumes:** stable request/response and error semantics from Tasks 1 and 2.

**Produces:** one machine-readable contract and generated TypeScript types/client bindings for the asset endpoints; this is a pilot, not a repository-wide API migration.

- [x] Write contract tests that validate a representative valid request, a missing-target 4xx response, and a target-specific result against the OpenAPI schema.
- [x] Add the smallest OpenAPI document that describes only the selected asset endpoints, error envelope, target-platform enum, and async result shape. Do not hand-maintain a second Go or TypeScript model.
- [x] Use `openapi-typescript` to generate deterministic TypeScript types; commit the output only under `web/listingkit-ui/src/lib/api/generated/`.
- [x] Replace the corresponding hand-written response cast in the UI preview client with a generated contract type while retaining runtime Zod validation and the existing proxy/auth boundary.
- [x] Add `make api-contract-check` to regenerate the pilot contract and fail on drift.
- [x] Run backend focused tests plus frontend checks:

  ```powershell
  go test ./internal/productimage/... ./internal/listingkit/... -count=1
  Set-Location web/listingkit-ui
  npm.cmd run typecheck
  npm.cmd test
  ```

- [x] Commit the contract pilot separately:

  ```text
  build(api): add generated asset endpoint contract
  ```

## 5. Baseline and product-flow gates

These are required evidence tracks, but they must not be bundled with Tasks 1-4 unless they expose a direct regression in that task.

```powershell
go test ./tests -count=1
go test ./internal/listingkit ./internal/productimage -count=1
go test ./internal/product/sourcing/... ./internal/catalog/... ./internal/asset/... ./internal/product/sourcehandoff/... -count=1
Set-Location web/listingkit-ui
npm.cmd run typecheck
```

Before calling the Product Sourcing MVP closed, run and record one controlled flow:

```text
1688 source request
  -> SourceEnvelope
  -> catalog and asset facts
  -> ListingKit task
  -> preview/readiness
```

The validation note must name the commit, describe source lineage, list missing-fact warnings/errors, and distinguish controlled evidence from a real tenant/store acceptance run.

## 6. Stop conditions

Pause the affected task and update this plan if it would:

- require a new global configuration read in a product, listing, marketplace, or integration package;
- make an unspecified target silently choose a platform;
- change Temporal activity retry/determinism or RabbitMQ acknowledgement ordering without a dedicated design and test plan;
- add marketplace-specific policy to root `internal/listingkit`;
- require broad import allowlist expansion instead of removing a dependency;
- couple the asset refactor to a new source integration, a platform workbench, or a database rewrite;
- expose tenant credentials, browser state, provider responses, or assets across tenant boundaries.

## 7. Definition of done

The next refactoring phase is complete when:

- all newly created image-processing requests have an explicit target platform;
- a multi-target listing has independent target-keyed image/asset processing and validation results;
- no image request path defaults to Amazon or relies on the first input platform;
- product-image business behavior no longer acquires global configuration directly;
- TaskEventV2 is the only new task-event output and legacy conversion has a measured retirement path;
- the asset HTTP pilot has a machine-readable contract and a deterministic drift check;
- targeted tests, `go test ./tests -count=1`, and UI typecheck pass for the exact commit under review;
- SHEIN stabilization and controlled 1688 acceptance remain separately recorded rather than implicitly claimed by refactoring tests.

## 8. Explicitly deferred

- Broad `internal/listingkit` renames or a one-shot compatibility-package migration.
- New scheduler/watchdog/worker ownership.
- A LangGraph or microservice replacement for Go/Temporal/RabbitMQ orchestration.
- Full TEMU, Amazon, or Walmart workbench expansion.
- A second product-source adapter before the controlled 1688 loop is closed.
