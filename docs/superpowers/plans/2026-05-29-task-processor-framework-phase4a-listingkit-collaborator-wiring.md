# Task Processor Framework Phase 4A ListingKit Collaborator Wiring Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the remaining collaborator-assembly pressure inside `internal/listingkit/service.go` by moving admin, submit, and temporal collaborator wiring behind explicit feature-owned builder seams, while keeping ListingKit business behavior unchanged.

**Architecture:** Treat `internal/listingkit/service.go` as a stateful service root, not a long-term home for dependency shaping. Reuse the existing `new*Service(...)` and `*ServiceConfig` seams that already exist in `settings_admin_service.go`, `shein_admin_service.go`, `service_submit.go`, and `service_submit_temporal_adapter.go`. This slice is about explicit collaborator wiring and ownership, not a domain rewrite.

**Tech Stack:** Go, existing ListingKit collaborator services, existing `service_wiring_test.go`, existing ListingKit package tests, CodeGraph-informed refactor boundaries

**Out of Scope For This Slice:**

- redesigning `internal/listingkit/service.go` field layout
- changing ListingKit HTTP/bootstrap ownership again
- rewriting workflow/process logic
- changing Temporal runtime registration
- moving every `task*OrDefault()` helper in one pass

---

## Root Cause This Slice Addresses

After `Phase 3B`, the major framework-facing bootstrap hotspots are much healthier, but `internal/listingkit/service.go` still concentrates a different kind of complexity:

1. it stores long-lived service state
2. it shapes collaborator config closures
3. it initializes collaborator groups
4. it hides dependency ownership behind `*OrDefault()` helpers spread across multiple files

The root problem is not that `service.go` is still large by itself.

The deeper problem is that collaborator ownership is still partially implicit:

- admin collaborators capture store/profile/settings state directly from `service`
- submit and temporal collaborators shape locks, workflow toggles, and remote submission behavior inline
- group initialization tests prove existence, but not explicit ownership of each collaborator wiring seam

That makes future refactors harder because the service root still acts as both:

- state holder
- collaborator wiring container

This slice separates those two roles more cleanly.

---

## Target Outcome

At the end of `Phase 4A`:

- `internal/listingkit/service.go` keeps service state and high-level collaborator initialization only
- admin collaborator wiring is expressed through explicit builder helpers owned near admin services
- submit and temporal collaborator wiring is expressed through explicit builder helpers owned near submit services
- collaborator group initialization tests lock the new seams
- business behavior stays unchanged

---

## Task 1: Split collaborator-group initialization out of `service.go`

**Files:**
- Create: `internal/listingkit/service_collaborators.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/service_wiring_test.go`

- [ ] **Step 1: Write failing boundary tests for collaborator-group ownership**

Add a narrow source-level guard in `service_wiring_test.go` that proves `service.go` keeps the root `NewService(...)` path but does not keep the full collaborator-group initialization bodies inline.

Lock at least these seams:

- `initializeTaskCollaborators`
- `initializeAdminCollaborators`
- `initializeSubmitCollaborators`
- `initializeTemporalCollaborators`

- [ ] **Step 2: Run focused collaborator wiring tests**

Run:

```powershell
go test ./internal/listingkit -run "Test(NewServiceInitializesCollaborators|ServiceInitializeCollaboratorGroups)" -count=1
```

Expected: PASS before the move, giving a safe baseline for the file split.

- [ ] **Step 3: Move collaborator-group initialization methods into a dedicated wiring file**

Create `service_collaborators.go` and move these methods there unchanged in behavior:

- `initializeCollaborators`
- `initializeTaskCollaborators`
- `initializeAdminCollaborators`
- `initializeSubmitCollaborators`
- `initializeTemporalCollaborators`

Keep `service.go` responsible for:

- the `service` struct
- `ServiceConfig`
- `NewService(...)`
- `newServiceWithConfig(...)`
- `applyDefaults()`

- [ ] **Step 4: Re-run ListingKit package verification**

Run:

```powershell
go test ./internal/listingkit -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service.go internal/listingkit/service_collaborators.go internal/listingkit/service_wiring_test.go
git commit -m "refactor: split listingkit collaborator initialization"
```

---

## Task 2: Make admin collaborator wiring explicit and feature-owned

**Files:**
- Create: `internal/listingkit/service_admin_wiring.go`
- Modify: `internal/listingkit/settings_admin_service.go`
- Modify: `internal/listingkit/shein_admin_service.go`
- Modify: `internal/listingkit/service_wiring_test.go`
- Modify: `internal/listingkit/service_shein_categories_test.go`
- Modify: `internal/listingkit/store_profile_service_test.go`

- [ ] **Step 1: Write failing tests for admin collaborator wiring seams**

Add tests that prove:

1. `settingsAdminOrDefault()` delegates config shaping to a dedicated builder
2. `sheinAdminOrDefault()` delegates config shaping to a dedicated builder
3. the resulting collaborators still expose the same runtime behavior through existing public methods

Use existing public entrypoints as guardrails:

- `ListSheinStoreProfiles(...)`
- `GetSheinSettings(...)`
- `SearchSheinCategories(...)`

- [ ] **Step 2: Run focused admin tests for a baseline**

Run:

```powershell
go test ./internal/listingkit -run "Test(ServiceInitializeCollaboratorGroups|SearchSheinCategories|ListSheinStoreProfiles)" -count=1
```

Expected: PASS before the refactor.

- [ ] **Step 3: Introduce explicit admin wiring helpers**

Create `service_admin_wiring.go` with narrow helpers such as:

- `buildSettingsAdminServiceConfig(s *service) settingsAdminServiceConfig`
- `buildSheinAdminServiceConfig(s *service) sheinAdminServiceConfig`

Then update:

- `settingsAdminOrDefault()`
- `sheinAdminOrDefault()`

to consume those builders instead of shaping closures inline.

Important:

- keep the mutation and locking behavior unchanged
- do not move domain logic out of `settings_admin_service.go` or `shein_admin_service.go`

- [ ] **Step 4: Re-run ListingKit admin verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(ServiceInitializeCollaboratorGroups|SearchSheinCategories|ListSheinStoreProfiles)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service_admin_wiring.go internal/listingkit/settings_admin_service.go internal/listingkit/shein_admin_service.go internal/listingkit/service_wiring_test.go internal/listingkit/service_shein_categories_test.go internal/listingkit/store_profile_service_test.go
git commit -m "refactor: extract listingkit admin collaborator wiring"
```

---

## Task 3: Make submit and temporal collaborator wiring explicit

**Files:**
- Create: `internal/listingkit/service_submit_wiring.go`
- Modify: `internal/listingkit/service_submit.go`
- Modify: `internal/listingkit/service_submit_temporal_adapter.go`
- Modify: `internal/listingkit/service_wiring_test.go`
- Modify: `internal/listingkit/service_submit_test.go`
- Modify: `internal/listingkit/service_submit_temporal_adapter_test.go`

- [ ] **Step 1: Write failing tests for submit/temporal wiring boundaries**

Add narrow tests that prove:

1. `taskSubmissionOrDefault()` delegates config shaping to a builder helper
2. `taskSubmissionExecutionOrDefault()` delegates config shaping to a builder helper
3. `taskTemporalSubmissionAdapterOrDefault()` delegates config shaping to a builder helper

Also keep one behavioral guardrail around:

- `SubmitTask(...)`
- one Temporal adapter entrypoint such as `BeginSheinPublishAttempt(...)`

- [ ] **Step 2: Run focused submit runtime tests**

Run:

```powershell
go test ./internal/listingkit -run "Test(SubmitTask|BeginSheinPublishAttempt|ServiceInitializeCollaboratorGroups)" -count=1
```

Expected: PASS before the refactor.

- [ ] **Step 3: Extract submit and temporal config builders**

Create `service_submit_wiring.go` with focused helpers such as:

- `buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig`
- `buildTaskSubmissionExecutionServiceConfig(s *service) taskSubmissionExecutionServiceConfig`
- `buildTaskTemporalSubmissionAdapterConfig(s *service) taskTemporalSubmissionAdapterConfig`

Then rewire:

- `taskSubmissionOrDefault()`
- `taskSubmissionExecutionOrDefault()`
- `taskTemporalSubmissionAdapterOrDefault()`

to use these helpers.

Keep the current lock acquisition, workflow toggle checks, and pricing/store-resolution behavior unchanged.

- [ ] **Step 4: Re-run submit and temporal verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(SubmitTask|BeginSheinPublishAttempt|ServiceInitializeCollaboratorGroups)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service_submit_wiring.go internal/listingkit/service_submit.go internal/listingkit/service_submit_temporal_adapter.go internal/listingkit/service_wiring_test.go internal/listingkit/service_submit_test.go internal/listingkit/service_submit_temporal_adapter_test.go
git commit -m "refactor: extract listingkit submit collaborator wiring"
```

---

## Task 4: Lock the new `listingkit` collaborator wiring boundary

**Files:**
- Create: `internal/listingkit/phase4a_collaborator_boundary_test.go`
- Modify: `internal/listingkit/service_wiring_test.go`
- Modify: `internal/listingkit/service_config_test.go`

- [ ] **Step 1: Add source and behavior guardrails**

Lock two things:

1. `service.go` remains a service root and does not regrow inline admin/submit/temporal config shaping
2. the explicit wiring helpers stay the place where collaborator config ownership lives

Suggested source-level checks:

- `service.go` should not contain `newSettingsAdminService(`
- `service.go` should not contain `newSheinAdminService(`
- `service.go` should not contain `newTaskSubmissionService(`
- `service.go` should not contain `newTaskTemporalSubmissionAdapter(`

- [ ] **Step 2: Run full ListingKit verification**

Run:

```powershell
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/phase4a_collaborator_boundary_test.go internal/listingkit/service_wiring_test.go internal/listingkit/service_config_test.go
git commit -m "test: lock listingkit collaborator wiring boundary"
```

---

## Self-Review

### Spec coverage

This plan intentionally covers only the collaborator-wiring hotspot that still remains after `Phase 3B`:

- service root vs collaborator wiring ownership
- admin collaborator config shaping
- submit/temporal collaborator config shaping
- regression guardrails

It does not expand into workflow phase modeling or service-domain redesign.

### Reuse check

This plan reuses seams that already exist in the codebase:

- `new*Service(...)` constructors
- `*ServiceConfig` structs
- existing collaborator group tests

It does not invent a new container or a generic collaborator framework.

### Root-cause check

The problem being addressed is implicit collaborator ownership, not file length alone.

The plan therefore focuses on:

- explicit config builders
- moving wiring near the collaborator services that consume it
- keeping the root service file focused on state and top-level initialization

### Scope discipline

This is a bounded slice:

- no service behavior rewrite
- no bootstrap-layer revisit
- no speculative generalization across unrelated features

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-29-task-processor-framework-phase4a-listingkit-collaborator-wiring.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
