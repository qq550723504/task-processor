# Standard Product and SHEIN Draft Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `canonical.Product` the explicit cross-platform product fact source and make `DraftPayload` the only new SHEIN draft write source while preserving historical JSON compatibility.

**Architecture:** Add a small SHEIN model-contract layer around the existing `Package` aliases. New production code reads and writes the semantic fields through helpers, validates draft structure separately from platform resolution state, and keeps `SkcList` as a compatibility/display summary for now. Do not introduce a second generic product model or remove legacy JSON fields in this phase.

**Tech Stack:** Go, standard library, existing Go test suite, JSON compatibility tests.

## Global Constraints

- Preserve all unrelated Studio working-tree changes.
- `canonical.Product` remains platform-neutral; SHEIN IDs, sites, warehouses, and submission state stay outside it.
- `DraftPayload` is the canonical SHEIN draft payload for new code; `RequestDraft` remains a read/write JSON compatibility alias only at the model boundary.
- `PreviewPayload` and `SubmissionState` remain separate from the draft payload.
- Do not change remote SHEIN API payload semantics in this phase.

### Task 1: Add the SHEIN draft contract helpers and structural validation

**Files:**
- Create: `internal/publishing/shein/model_contract.go`
- Modify: `internal/publishing/shein/semantic_fields.go`
- Test: `internal/publishing/shein/model_contract_test.go`
- Test: `internal/publishing/shein/semantic_fields_test.go`

**Interfaces:**
- `DraftPayloadOf(pkg *Package) *RequestDraft` returns the normalized semantic draft pointer or nil.
- `EnsureDraftPayload(pkg *Package) *RequestDraft` creates and aliases an empty semantic draft when the package exists.
- `SetDraftPayload(pkg *Package, draft *RequestDraft) *Package` updates semantic and legacy draft pointers together.
- `ValidateDraftPayload(draft *RequestDraft) []DraftPayloadIssue` reports structural issues with `Path`, `Code`, and `Message`.
- Structural validation checks only draft hierarchy shape: at least one SKC, each SKC at least one SKU, and each SKU supplier SKU. Title completeness can come from the package/preview compatibility fields and is not treated as a hierarchy check. Platform template readiness remains in the workspace layer.

- [x] **Step 1: Write failing tests for semantic access and structural validation.**
- [x] **Step 2: Run `go test ./internal/publishing/shein -run 'Test(DraftPayload|ValidateDraftPayload|PackageSemantic)' -count=1` and confirm the new tests fail because the helpers do not exist.**
- [x] **Step 3: Implement the helpers and use `SetDraftPayload`/`EnsureDraftPayload` inside semantic normalization without changing JSON compatibility behavior.**
- [x] **Step 4: Run the focused tests and confirm they pass.**
- [x] **Step 5: Add a regression test proving a legacy-only `request_draft` package is normalized to the semantic pointer and can be validated.**
- [x] **Step 6: Run the focused package tests again.**

### Task 2: Migrate core SHEIN workspace and revision reads/writes to the semantic draft contract

**Files:**
- Modify: `internal/marketplace/shein/workspace/state.go`
- Modify: `internal/marketplace/shein/workspace/editor_progress.go`
- Modify: `internal/marketplace/shein/workspace/editor_dirty_hints.go`
- Modify: `internal/marketplace/shein/workspace/editor_context_builder.go`
- Modify: `internal/marketplace/shein/workspace/inspection.go`
- Modify: `internal/marketplace/shein/workspace/submit_payload_readiness_checks.go`
- Modify: `internal/listingkit/revision_apply_shein_helpers.go`
- Modify: `internal/listingkit/service_revision_recompute.go`
- Test: existing workspace and revision tests plus focused new tests where needed

**Interfaces:**
- All migrated reads call `sheinpub.DraftPayloadOf(pkg)`.
- All migrated creation/assignment paths call `sheinpub.EnsureDraftPayload(pkg)` or `sheinpub.SetDraftPayload(pkg, draft)`.
- `BuildSubmitPayloadReadinessChecks` reports the first structural `DraftPayload` issue instead of treating a non-nil empty draft as ready.
- Legacy-only packages continue to work through boundary normalization.

- [x] **Step 1: Add failing readiness coverage for a non-nil but structurally empty draft.**
- [x] **Step 2: Run the focused workspace test and confirm it fails because readiness currently checks only pointer existence.**
- [x] **Step 3: Migrate readiness and the selected workspace/revision paths to the semantic helpers.**
- [x] **Step 4: Run focused workspace, revision, and publishing tests.**
- [x] **Step 5: Add/adjust compatibility assertions for legacy `RequestDraft` inputs.**
- [x] **Step 6: Re-run the same focused test set.**

### Task 3: Document the model boundary and prevent accidental regressions

**Files:**
- Modify: `internal/publishing/shein/model.go`
- Modify: `internal/publishing/shein/doc.go`
- Modify: `docs/architecture/project-boundaries.md` only if the existing boundary text lacks the explicit `DraftPayload` rule
- Test: `internal/publishing/shein/semantic_fields_test.go`

**Interfaces:**
- Comments state that `canonical.Product` is input fact data, `Package` is workflow state, and `DraftPayload` is the SHEIN draft contract.
- Comments state that `SkcList` is a compatibility/display summary in this phase, not the new draft write source.
- Tests assert semantic/legacy alias consistency and separation of preview/submission state.

- [x] **Step 1: Add the boundary comments and a test that semantic aliases remain synchronized after JSON marshal/unmarshal.**
- [x] **Step 2: Run the package tests.**
- [x] **Step 3: Review the diff for unrelated changes and remove any accidental scope expansion.**

### Task 4: Verify the implementation

**Files:**
- No production files; verification only.

- [x] **Step 1: Run `gofmt` on only the changed Go files.**
- [x] **Step 2: Run focused tests for `internal/publishing/shein` and `internal/marketplace/shein/workspace`.**
- [x] **Step 3: Run `go test ./internal/listingkit/... ./internal/publishing/shein/... ./internal/marketplace/shein/...`.**
- [x] **Step 4: Run `git diff --check`.**
- [x] **Step 5: Run `git status --short` and verify the unrelated Studio files remain unchanged and unstaged.**
