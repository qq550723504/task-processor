# SHEIN Platform Normalization and Store 986 Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent platform-case mismatches from hiding SHEIN import tasks and safely resume the 200 confirmed pending tasks for store 986.

**Architecture:** Normalize source, target, and legacy platform fields at the import-task write boundary, add case-insensitive defensive matching to dispatch lifecycle queries, and add regression tests for mixed-case source/target values. Backfill only audited pending store-986 rows in a transaction; let the existing control-plane claim and publish them.

**Tech Stack:** Go, GORM, PostgreSQL, RabbitMQ, Kubernetes, Go tests.

## Global Constraints

- Preserve `platform/source_platform=amazon` and `target_platform=shein`; do not collapse source and target into one field.
- Keep the control-plane platform configuration canonical as lowercase `shein`.
- Do not manually set tasks to `queued` or publish RabbitMQ messages outside the control-plane workflow.
- Do not modify historical published, draft, paused, or terminated tasks during the store-986 recovery.
- Stage only files related to this repair if Git staging is requested later.

### Task 1: Add failing tests for canonical platform routing

**Files:**
- Modify: `internal/listingadmin/import_task_handler_test.go`
- Modify: `internal/listingadmin/import_task_repository_test.go`

- [ ] Add a test proving batch creation canonicalizes `Amazon` and `SHEIN` to lowercase while preserving source and target semantics.
- [ ] Add a repository candidate-selection test proving a stored `target_platform=SHEIN` row is selected for request platform `shein`.
- [ ] Run the focused tests and confirm they fail because the current code only trims strings and compares platforms case-sensitively.

### Task 2: Implement write-boundary normalization

**Files:**
- Create or modify the shared platform normalization helper under `internal/domain/task/`.
- Modify: `internal/listingadmin/import_task_handler.go`
- Modify: `internal/listingadmin/import_task_model.go`
- Modify: `internal/listingadmin/import_task_repository.go`

- [ ] Normalize and validate platform values at API validation and repository persistence boundaries.
- [ ] Use lowercase canonical values for `platform`, `source_platform`, and `target_platform` without changing their meanings.
- [ ] Preserve legacy behavior by treating a missing target platform as the legacy platform only where the existing contract permits it.
- [ ] Run the focused tests and confirm they pass.

### Task 3: Add defensive dispatch matching and regression coverage

**Files:**
- Modify: `internal/listingadmin/import_task_repository.go`
- Modify: `internal/listingcontrol/scheduler_test.go` if scheduler-level coverage is needed.

- [ ] Make dispatch candidate, paused-task, daily-usage, and queued-count platform predicates normalize both database values and request values.
- [ ] Keep unrelated platform query behavior unchanged unless it consumes the same task-routing fields.
- [ ] Run focused listing-control tests, then the repository package tests.

### Task 4: Verify and backfill store 986 safely

**Files:**
- No production code files.
- Optional operational evidence: `docs/product/validation/` only if the user requests durable runbook documentation.

- [ ] Re-query store 986 configuration, Redis owner/mode, shard-8 queue, and the exact 200-row scope.
- [ ] Confirm the 200 product IDs remain unique and have no existing published, draft, or active duplicate task.
- [ ] In one PostgreSQL transaction, normalize only the audited pending rows with `platform=Amazon`, `source_platform=Amazon`, `target_platform=SHEIN`, `store_id=986`.
- [ ] Verify the control-plane claims/publishes tasks and shard-8 consumes them; do not update task status manually.
- [ ] Record final counts, queue depth, control-plane dispatch evidence, and any task failures.

### Task 5: Full verification and handoff

- [ ] Run `gofmt` on modified Go files.
- [ ] Run focused tests and the relevant broader Go test packages.
- [ ] Run `git diff --check` and inspect the final diff.
- [ ] Report code changes separately from production data and runtime evidence.
