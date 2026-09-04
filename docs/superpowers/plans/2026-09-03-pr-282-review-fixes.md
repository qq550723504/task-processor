# PR #282 Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Resolve the verified PR #282 review findings without reviving retired ListingKit Studio runtime paths or breaking frozen Temporal V2 histories.

**Architecture:** Catalog owns the canonical ProductSnapshot and its atomic/idempotent publication port. Source ingestion publishes the normalized snapshot before task dispatch, while ImageAgent consumes an authorized, dimension-complete catalog. V2 and V3 ImageAgent execution remain separate wires; V3 preserves final artifact metadata through effect, projection, recovery, and approval publication.

**Tech Stack:** Go, GORM, Temporal, React/Next.js, TanStack Query, Vitest, existing `internal/integration/httpimage` and product image capability ports.

**Spec:** `docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md` and the current PR #282 review threads.

## Global Constraints

- Preserve unrelated existing changes in the main worktree.
- Do not restore retired Studio endpoints or reintroduce legacy runtime dependencies.
- Catalog publication must be idempotent for the same tenant/product/publication identity and reject the same identity with a different payload.
- A size slot must fail closed when source image dimensions are unknown or non-positive.
- Historical V2 Temporal activity input remains frozen; V3-only fields must not be required by the V2 executor.
- Every production behavior change starts with a failing focused test and is followed by targeted and package-level verification.

### Task 1: Persist 1688 source snapshots before task dispatch

**Files:**
- Modify: `internal/app/httpapi/product_catalog_composition.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/compatibility/listingkit/sourcehandoff/a1688/command.go`
- Modify: `internal/compatibility/listingkit/sourcehandoff/a1688/listingkit_task.go`
- Modify: `internal/product/sourcing/catalog_asset_handoff.go`
- Test: `internal/app/httpapi/product_catalog_composition_test.go`
- Test: `internal/compatibility/listingkit/sourcehandoff/a1688/*_test.go`
- Test: `internal/product/sourcing/*_test.go`

**Interfaces:** `catalog.Repository` is constructed once at the application composition boundary and passed as `catalog.Publisher` to the 1688 command service. `CreateTask` prepares one envelope, converts it to a snapshot including image candidates, publishes with a deterministic publication ID, then calls `CreateGenerateTask`.

- [ ] Write a test proving a source asset candidate is present in the published snapshot and that task creation is not called when publication fails.
- [ ] Run the focused tests and confirm the new test fails because only the reader is wired and candidates are dropped.
- [ ] Add the writer/publisher dependency, deterministic publication identity, publication-before-dispatch ordering, and idempotent retry behavior using existing Catalog ports.
- [ ] Map image candidates into `catalog.ProductSnapshot.Images`, preserving URL, role, media type filtering, checksum, and source trace without persisting video candidates as images.
- [ ] Run the focused Catalog, sourcehandoff, and app composition tests.

### Task 2: Carry reliable source image dimensions into authorization

**Files:**
- Modify: `internal/product/sourcing/source_envelope.go`
- Modify: `internal/product/catalog/snapshot.go`
- Modify: `internal/app/httpapi/image_agent_asset_catalog.go`
- Modify: existing bounded image metadata adapter under `internal/integration/httpimage` if required by its current API
- Test: `internal/app/httpapi/image_agent_asset_catalog_test.go`
- Test: `internal/product/sourcing/*_test.go`

**Interfaces:** `catalog.Image` and the source asset handoff carry positive pixel `Width` and `Height`. The authorized catalog converter copies them into `imageagent.AuthorizedAsset`. Missing dimensions are resolved through a bounded existing image adapter; failures remain missing and are rejected by size-slot validation.

- [ ] Write tests proving dimensions survive source-to-catalog-to-authorized-catalog conversion and that an unknown dimension is not replaced with a default.
- [ ] Run the focused tests and confirm failure.
- [ ] Implement the smallest model and adapter changes, enforcing URL safety, timeout, response-size limits, and image decode validation at the integration boundary.
- [ ] Run the Catalog and ImageAgent asset-catalog tests.

### Task 3: Execute configured image review before acceptance

**Files:**
- Modify: `internal/imageagent/tools/product_image_executor.go`
- Modify: `internal/app/worker/imageagent/dependencies.go`
- Modify: `internal/imageagent/temporal/types.go` only if review evidence needs an additive V3 field
- Test: `internal/imageagent/tools/product_image_executor_contract_test.go`
- Test: `internal/app/worker/imageagent/dependencies_test.go`

**Interfaces:** `imageagenttools.Dependencies` receives `productimage.Reviewer`. Quoting includes the `review` operation. Generated candidates are submitted to the reviewer with the resolved product/profile context; the profile’s role-specific threshold determines accepted versus human-review/blocked outcome. Reviewer errors retain provider-dispatch semantics.

- [ ] Write a test proving a configured reviewer is called and a below-threshold result cannot become accepted.
- [ ] Run the focused executor test and confirm failure because the reviewer is currently omitted and never invoked.
- [ ] Add reviewer injection, quote validation, review invocation, threshold evaluation, and durable review outcome handling without accepting unreviewed output.
- [ ] Run executor, capability-composition, and affected Temporal tests.

### Task 4: Keep V2 history execution compatible while selecting V3 explicitly

**Files:**
- Modify: `internal/app/worker/imageagent/dependencies.go`
- Modify: `internal/imageagent/temporal/activities.go` only if the compatibility adapter needs an explicit wire boundary
- Modify: `internal/imageagent/tools/product_image_executor.go` or add a focused V2 adapter beside it
- Test: `internal/app/worker/imageagent/dependencies_test.go`
- Test: `internal/imageagent/temporal/history_replay_test.go`

**Interfaces:** V2 dependencies expose a frozen-input executor that does not require `TargetPlatform` or `ImagePolicyContext`; V3 dependencies expose the policy-aware staged executor. No V2 activity input shape is expanded.

- [ ] Add a replay/compatibility test with a frozen V2 `ExecuteSlotActivityInput` and assert it does not fail only because V3 policy fields are absent.
- [ ] Run the replay test and confirm failure against the current shared executor.
- [ ] Wire separate V2 and V3 executors, reusing the pre-hard-cut behavior where necessary, while keeping reviewer/policy enforcement on the V3 path.
- [ ] Run history replay and worker dependency tests.

### Task 5: Preserve V3 final artifact metadata through approval

**Files:**
- Modify: `internal/imageagent/slot_effect_v3.go`
- Modify: `internal/imageagent/model.go`
- Modify: `internal/imageagent/temporal/activities.go`
- Modify: `internal/imageagent/temporal/workflow.go`
- Modify: `internal/imageagent/temporal/effect_recovery_workflow.go`
- Modify: `internal/imageagent/assetpublication/publisher.go`
- Test: `internal/imageagent/assetpublication/publisher_test.go`
- Test: `internal/imageagent/temporal/slot_effect_v3_activity_test.go`
- Test: `internal/imageagent/temporal/effect_recovery_workflow_test.go`

**Interfaces:** V3 candidate/effect projections carry the allowlisted `Width`, `Height`, and `Operations` from `FinalManifest`. All normalization, fingerprints, recovery comparisons, and approval commits use the same metadata. V2 URL-based approval remains unchanged.

- [ ] Write a failing approval test asserting dimensions and operations reach `productasset.ApprovedAsset`.
- [ ] Run the test and confirm the approval projection currently writes zero/nil metadata.
- [ ] Extend the V3 allowlist and every projection/recovery conversion, then copy the normalized metadata in `approvalCommit`.
- [ ] Run package tests covering publication, recovery, fingerprints, and approval idempotency.

### Task 6: Remove all production UI calls to retired routes

**Files:**
- Modify/remove: `web/listingkit-ui/src/components/listingkit/workspace/use-workspace-data.ts`
- Modify/remove: `web/listingkit-ui/src/components/listingkit/workspace/use-workspace-navigation-actions.ts`
- Modify/remove: `web/listingkit-ui/src/components/listingkit/workspace/use-shein-workspace-actions.ts`
- Modify/remove: `web/listingkit-ui/src/components/listingkit/queue/queue-screen.tsx`
- Modify/remove: corresponding `web/listingkit-ui/src/lib/api/*`, `src/lib/query/*`, proxy mocks, README, and tests
- Test: affected workspace and queue Vitest suites

**Interfaces:** The shipped UI uses only route descriptors that still exist after the hard cut. Retired generation queue/review/dispatch and SHEIN regeneration consumers are migrated to surviving preview/result/action/recovery flows or removed together with their unreachable screens.

- [ ] Add a static/source test that fails if retired endpoint strings or their query hooks remain in production UI imports.
- [ ] Run the focused frontend tests and confirm the current stale consumers are detected.
- [ ] Remove or migrate consumers atomically, update mocks and documentation, and keep workspace loading independent of retired review-session endpoints.
- [ ] Run typecheck and all affected Vitest suites.

### Task 7: Full verification and review handoff

**Files:** No new production files; update only tests or documentation if verification exposes an explicit contract gap.

- [ ] Run all focused Go and frontend suites from Tasks 1–6.
- [ ] Run `go test ./...` and the frontend typecheck/test commands defined by the repository.
- [ ] Inspect `git diff`, `git status`, and the final changed-file list in the isolated worktree.
- [ ] Re-check the PR review threads against each fix and report any environment-only failures separately.
