# Image Agent Workspace Authorization and Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the manual image-agent flow reachable from the ListingKit workspace while durably binding each run to one task-owned source image and, for target-keyed tasks, one immutable target platform; close the execution-timeout and recovery-redrive review defects without weakening existing ownership, idempotency, or provider-effect guarantees.

**Architecture:** Keep ListingKit responsible for task ownership, canonical asset selection, target-bundle publication, and the workspace-facing preflight/create API. Keep imageagent responsible for validating and persisting an immutable run/plan/catalog snapshot, and Temporal responsible for deterministic orchestration only. Add an application-owned pre-deadline path that writes a terminal blocked projection before the server’s hard execution timeout. Store the pre-blocked external-effect phase so explicit redrive can restore the exact effect state and reconcile it without submitting a new provider generation.

**Tech Stack:** Go, Gin, GORM (SQLite/PostgreSQL), Temporal Go SDK, React/Next.js, TypeScript, Vitest, Go `testing`/`testify`.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-workspace-entry-and-style-authorization-design.md`

## Guardrails

- Browser requests never supply a tenant, user, arbitrary source URL, server identity, or a provider authorization snapshot.
- The workspace request contains exactly one `source_asset_id`; styles are optional and remain separate from source material.
- A target-keyed task requires an owned, normalized `target_platform`; that value is persisted on `imageagent.Run` and never changed by plan replacement or retry.
- Runs with an empty target remain compatible with scalar tasks. They must fail closed rather than infer a target for a target-keyed task.
- Existing generic `/api/v1/image-agent/runs` clients and their plan-validation contract stay compatible. The new ListingKit entrypoint is an additive, task-scoped adapter.
- A recovery redrive is scoped to tenant, owner, run, revision, slot, attempt, and action ID. It may reconcile an existing external effect but must never quote or generate again.
- Do not modify or stage `docs/superpowers/plans/2026-08-29-image-agent-recovery-lifecycle-closure.md`; it is unrelated user work.

## File map

- `internal/imageagent/model.go`, `service.go`, `repository.go`: immutable run contract, creation validation, projection persistence port.
- `internal/imageagent/store/records.go`, `gorm.go`, `memory.go`: GORM schema/mapping and parity in the in-memory repository.
- `internal/imageagent/temporal/types.go`, `worker.go`, `workflow.go`, `activities.go`, `effect_recovery_workflow.go`: V3 deadline clock, durable blocked transition, and effect recovery orchestration.
- `internal/imageagent/tools/productimage_executor.go`: provider request construction; must reject ambiguous multiple sources even if a caller bypasses the new entrypoint.
- `internal/listingkit/httpapi/image_agent_asset_catalog.go`, `image_agent_approved_publisher.go`, `routes_descriptor_task.go`, `runtime_module.go`: task-owned asset catalog, target-scoped publication, route wiring and composition.
- `internal/imageagent/httpapi/{handler.go,dto.go,routes.go}`: existing generic API DTO boundary; expose `target_platform` in projections without changing legacy request semantics.
- `web/listingkit-ui/src/lib/api/image-agent.ts`, `lib/types/image-agent.ts`, `app/api/listing-kits/proxy-url.ts`, and `components/listingkit/workspace/workspace-screen.tsx`: BFF allowlist, task preflight/create client, target-aware workspace panel, and tests.

## Implementation tasks

### 1. Add immutable target and exact-one-source domain validation

**Files:**
- Modify: `internal/imageagent/model.go`
- Modify: `internal/imageagent/service.go`
- Modify: `internal/imageagent/validation.go` (or the existing plan-validation file containing `ValidateInitialSubmittedPlan`)
- Modify tests: `internal/imageagent/model_test.go`, `internal/imageagent/service_test.go`, existing plan-validation tests

- [ ] Write failing domain tests that reject a submitted manual plan with zero or more than one `Plan.SourceAssetIDs`, reject any slot whose `SourceAssetIDs` does not contain that same single source, and preserve multi-slot plans that all reference it.
- [ ] Add `TargetPlatform string` to `imageagent.Run`; normalize/validate it at run initialization and include it in the existing-run idempotency equality check so the same run ID cannot be replayed with a different target.
- [ ] Add the narrowly scoped exact-one-source rule to initial manually submitted plans only; leave plan replacement semantics intact unless it attempts to alter the immutable source snapshot.
- [ ] Extend `StartRunInput` with the target selected by the ListingKit adapter, persist it into `Run`, and ensure generic legacy callers default to the scalar-compatible empty value.
- [ ] Run `go test ./internal/imageagent -count=1` and keep all existing run modes/legacy tests green.

### 2. Persist and expose target platform across repositories and HTTP projection DTOs

**Files:**
- Modify: `internal/imageagent/store/records.go`
- Modify: `internal/imageagent/store/gorm.go`
- Modify: `internal/imageagent/store/memory.go`
- Modify tests: `internal/imageagent/store/repository_contract_test.go`, `internal/imageagent/store/schema_v2_test.go`, `internal/imageagent/store/gorm_concurrency_test.go`
- Modify: `internal/imageagent/httpapi/dto.go`
- Modify tests: `internal/imageagent/httpapi/handler_test.go`

- [ ] Add a non-null/default-empty `target_platform` column to `image_agent_v2_runs`, map it both directions in GORM, and preserve it through projection snapshots, readback, idempotent commits, and memory-repository clones.
- [ ] Add repository contract tests for scalar (`""`) and target-keyed (`"shein"`) runs, including a GORM AutoMigrate/readback case so PostgreSQL-compatible column mapping is exercised.
- [ ] Add `target_platform` to the run response DTO and TypeScript-compatible JSON shape, while retaining omission/empty-value compatibility for older scalar runs.
- [ ] Run `go test ./internal/imageagent/store ./internal/imageagent/httpapi -count=1`.

### 3. Make ListingKit resolve task-owned source/style assets for a selected target

**Files:**
- Modify: `internal/listingkit/httpapi/image_agent_asset_catalog.go`
- Modify tests: `internal/listingkit/httpapi/image_agent_asset_catalog_test.go`
- Modify: `internal/imageagent/catalog.go` or the existing `AssetCatalogScope` declaration

- [ ] Write failing catalog tests for scalar assets, a selected `shein` bundle, missing target, unknown target, mismatched owner, source-as-style, and styles selected only from the selected bundle.
- [ ] Carry `TargetPlatform` through `AssetCatalogScope`. Continue to authenticate tenant/user from context and verify `BusinessTaskID`; then select the scalar snapshot when the task has no target bundles, or select exactly `AssetBundlesByTarget[normalizedTarget]` when it does.
- [ ] Return only canonical, safe-URL assets; classify source images as sources and only caller-selected non-source bundle assets as styles. Do not copy arbitrary task metadata into the provider snapshot.
- [ ] Keep legacy target-keyed calls without an explicit target fail-closed; never choose the first map entry.
- [ ] Run `go test ./internal/listingkit/httpapi -run 'ImageAgent.*Catalog|.*ImageAgent.*' -count=1`.

### 4. Add the task-scoped ListingKit preflight/create adapter and target-scoped publication

**Files:**
- Modify: `internal/listingkit/httpapi/routes_descriptor_task.go`
- Modify: `internal/listingkit/httpapi/routes_handler.go` and the concrete task route handler
- Add: `internal/listingkit/httpapi/image_agent_workspace_handler.go`
- Add tests: `internal/listingkit/httpapi/image_agent_workspace_handler_test.go`, `internal/listingkit/httpapi/routes_interface_test.go`
- Modify: `internal/listingkit/httpapi/image_agent_approved_publisher.go`
- Modify tests: `internal/listingkit/httpapi/image_agent_approved_publisher_test.go`

- [ ] Define `GET /api/v1/listing-kits/tasks/:task_id/image-agent-assets?target_platform=...`; authorize with the existing image-agent read permission, resolve identity/task ownership, and return source and optionally selectable style DTOs plus the normalized target context.
- [ ] Define `POST /api/v1/listing-kits/tasks/:task_id/image-agent-runs` with strict JSON: `target_platform`, one `source_asset_id`, optional `style_asset_ids`, and the existing client idempotency/action fields. Reject duplicate IDs, missing source, multiple/unknown sources, bad target selection, and unowned tasks before calling imageagent.
- [ ] Build the initial `imageagent.Plan` server-side: revision 1, one main slot, the selected source, selected styles, stable server-generated idempotency keys, and a `MaxImages=1` budget. Call `imageagent.Service.Start` with the persisted task target, never a browser-supplied plan.
- [ ] Keep the generic image-agent route unchanged; make the new task route additive and wire it through `runtime_module.go`/handler interfaces with verified identity and explicit permissions.
- [ ] Update approved-asset publication so a run with `TargetPlatform` writes only its target bundle and matching inventory summary. Keep scalar publication behavior for empty targets and fail closed for target-keyed tasks with legacy empty-target runs. Preserve user/non-image-agent records and remove stale image-agent records only in the selected bundle.
- [ ] Add regression tests proving a `shein` run neither reads nor overwrites `amazon`, a scalar run remains compatible, and no publication can infer a target.
- [ ] Run `go test ./internal/listingkit/httpapi -count=1`.

### 5. Close ambiguous provider-source handling at the execution boundary

**Files:**
- Modify: `internal/imageagent/tools/productimage_executor.go`
- Modify tests: `internal/imageagent/tools/productimage_executor_test.go`

- [ ] Add a red test submitting a slot with two nonempty `SourceAssetIDs` directly to the executor and assert a validation error before any provider/publisher call.
- [ ] Replace first-nonempty selection with exact-one normalization: reject zero, reject duplicates/multiple IDs, and use the sole source only after it is found in the immutable authorized catalog.
- [ ] Preserve any valid style-reference handling and existing single-source result identity.
- [ ] Run `go test ./internal/imageagent/tools -count=1`.

### 6. Add a durable application-owned lifecycle deadline before Temporal’s hard timeout

**Files:**
- Modify: `internal/imageagent/temporal/types.go`
- Modify: `internal/imageagent/temporal/worker.go`
- Modify: `internal/imageagent/temporal/workflow.go`
- Modify: `internal/imageagent/temporal/activities.go`
- Modify: `internal/imageagent/temporal/slot_workflow.go`
- Modify tests: `internal/imageagent/temporal/workflow_test.go`, `internal/imageagent/temporal/manual_acceptance_test.go`, `internal/imageagent/temporal/worker_test.go`

- [ ] Define a V3 lifecycle deadline exactly one day before `V3WorkflowExecutionTimeout`; derive and pass it for all new V3 manual starts separately from user-configured budget `MaxElapsed`.
- [ ] Add deterministic workflow timer/select logic that stops dispatch before the lifecycle deadline, waits only for bounded in-flight settlement, and calls a versioned activity to persist a blocked run projection with a distinct lifecycle-deadline code/message.
- [ ] Make the persisted deadline block permit recovery/cancel only and ensure it cannot be overwritten by late completion or command processing. Existing time/budget behavior remains compatible for old histories through a Temporal version marker.
- [ ] Bound child activity windows to the earlier of lifecycle settlement and existing budget deadline so a provider finalization cannot run past the durable stop point.
- [ ] Test the timer path with Temporal test time: no new slot dispatch after deadline, durable `RunStatusBlocked` projection survives, and server `WorkflowExecutionTimeout` remains 30 days. Test an in-flight slot and approval-waiting state to prove commands are not silently lost at the server timeout.
- [ ] Run `go test ./internal/imageagent/temporal -count=1`.

### 7. Make recovery-blocked effects explicitly redrivable without regeneration

**Files:**
- Modify: `internal/imageagent/model.go` / `recoverable_effect.go`
- Modify: `internal/imageagent/store/records.go`, `slot_effect_v3.go`, `memory.go`
- Modify: `internal/imageagent/temporal/activities.go`
- Modify: `internal/imageagent/temporal/effect_recovery_workflow.go`
- Modify: `internal/imageagent/temporal/worker.go`
- Modify tests: `internal/imageagent/store/slot_effect_v3_repository_test.go`, `internal/imageagent/temporal/effect_recovery_workflow_test.go`, `internal/imageagent/temporal/worker_test.go`, `internal/imageagent/temporal/manual_acceptance_test.go`

- [ ] Add a failing persistence test showing `PersistRecoveryBlockedEffectV3` records the pre-blocked safe phase (provider/staging/publication unknown) alongside the current `recovery_blocked` phase.
- [ ] Add an atomic repository operation keyed by effect identity and action ID that restores only a recovery-blocked effect to that saved phase, clears the recovery block metadata, and records idempotent redrive state. Reject missing/corrupt prior phases and all cross-owner/revision/attempt mismatches.
- [ ] Change `RecoverEffectV3` to invoke that restore operation before reconciliation. The recovery workflow must use the existing deterministic effect-recovery ID/action key and must not call quote/generate/publish for the provider-generation path.
- [ ] Ensure stale/repeated redrive action IDs are idempotent and do not start a second external recovery workflow; allow a new action only after the prior recovery workflow outcome is durable.
- [ ] Add Temporal acceptance tests starting with `recovery_blocked`, verifying the phase is restored, reconciliation completes or blocks with a meaningful outcome, and provider `Generate`/`Quote` call counts remain zero.
- [ ] Run `go test ./internal/imageagent/store ./internal/imageagent/temporal -count=1`.

### 8. Add workspace controls through the BFF and render the run workbench

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/image-agent.ts`
- Modify: `web/listingkit-ui/src/lib/api/image-agent.ts`
- Modify: `web/listingkit-ui/src/app/api/listing-kits/proxy-url.ts`
- Modify tests: `web/listingkit-ui/src/app/api/listing-kits/proxy-url.test.ts`, `web/listingkit-ui/src/lib/api/image-agent.test.ts`
- Add: `web/listingkit-ui/src/components/listingkit/image-agent/image-agent-launch-panel.tsx`
- Add tests: `web/listingkit-ui/src/components/listingkit/image-agent/image-agent-launch-panel.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen.tsx`
- Modify tests: `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen*.test.tsx`

- [ ] Add typed parsers and client calls for task preflight/create; include `target_platform` in run DTO types and parse no server identity fields.
- [ ] Extend the BFF’s explicit route allowlist only for the two task-scoped paths, preserving path encoding and rejecting arbitrary image-agent/admin forwarding.
- [ ] Render a workspace panel in the task context that loads the selected platform’s preflight catalog, requires one source via radio selection, exposes style references as optional checkboxes, and disables creation until all task/target/source requirements are met.
- [ ] Submit only target, source, styles, and client idempotency to the task route. On accepted creation, show the existing `ImageAgentWorkbench` with the returned run ID; do not construct a Plan in the browser.
- [ ] Render a distinct deadline-block guidance state that offers only the supported recovery/cancel actions, and preserve existing run-link/workspace routing behavior.
- [ ] Add tests for target selection propagation, source-required behavior, multiple source selections being impossible, optional styles, success handoff to the workbench, API errors, and no image-agent launch affordance outside an eligible workspace task.
- [ ] Run `npm.cmd --prefix web/listingkit-ui test -- --run` (or the repository’s focused Vitest command if package scripts require it).

### 9. Run focused-to-broad verification and close review threads only after evidence

**Files:**
- Modify only if formatting/checks demand it; do not alter unrelated user files.

- [ ] Run `gofmt` on every touched Go file and `git diff --check`.
- [ ] Run the focused Go suites from tasks 1–7, then `go test ./internal/imageagent/... ./internal/listingkit/httpapi -count=1`.
- [ ] Run the focused UI tests from task 8 and the project’s lint/typecheck scripts documented in `web/listingkit-ui/package.json`.
- [ ] Inspect `git status --short`, stage explicit implementation/test/doc paths only, and commit on `codex/image-agent-core-manual`; leave the untracked recovery plan untouched.
- [ ] Push the branch, re-query PR #239 review threads and CI, reply to each comment with the exact behavioral evidence, and resolve only the threads addressed by verified code/CI.

## Final acceptance criteria

1. A verified workspace user can start one manual image run from an owned task with one source, optional styles, and an immutable selected platform.
2. Target-keyed asset reads and writes are isolated to the selected target; scalar behavior stays unchanged and ambiguous legacy runs fail closed.
3. Direct API/executor callers cannot silently provide multiple source assets.
4. Every pre-timeout expiry produces a durable blocked projection before Temporal can terminate the workflow, with no new provider dispatch after expiry.
5. A recovery-blocked external effect can be redriven through its saved phase without creating a new provider generation.
6. Existing generic image-agent API users, authorization checks, and unrelated workspace paths retain their current contracts.
