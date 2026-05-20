# Contract Decoupling Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop business/platform packages from importing `internal/app/*` for shared contracts by introducing top-level neutral contract packages and migrating callers.

**Architecture:** This phase does not move heavy implementations like `BaseProcessor` or `state.MemoryManager`. It introduces top-level compatibility packages for fetcher, scheduler, ports, and taskstatus so business code depends on neutral paths first; old `internal/app/*` packages remain in place as implementation detail or compatibility layer.

**Tech Stack:** Go, existing task/scheduler/fetcher/task-status abstractions

---

### Task 1: Add neutral top-level contract packages

**Files:**
- Create: `internal/ports/contracts.go`
- Create: `internal/scheduler/contracts.go`
- Create: `internal/taskstatus/service.go`
- Create: `internal/crawler/fetcher/contracts.go`

- [ ] **Step 1: Add top-level `ports` aliases**
- [ ] **Step 2: Add top-level `scheduler` aliases for task types/config**
- [ ] **Step 3: Add top-level `taskstatus` wrapper package**
- [ ] **Step 4: Add top-level `crawler/fetcher` compatibility package**

### Task 2: Migrate business and platform packages to neutral imports

**Files:**
- Modify: `internal/product/product_fetcher.go`
- Modify: `internal/platformbase/*.go`
- Modify: `internal/platformtask/*.go`
- Modify: `internal/shein/scheduler/*.go`
- Modify: `internal/temu/scheduler/*.go`
- Modify: `internal/shein/productdata/*.go`
- Modify: `internal/temu/sku/*.go`
- Modify: `internal/temu/product/*.go`
- Modify: `internal/temu/sync/*.go`
- Modify: `internal/temu/pricing/*.go`
- Modify: `internal/shein/inventory/sync.go`
- Modify: `internal/shein/pipeline/status.go`
- Modify: `internal/temu/task_handler.go`
- Modify: `internal/amazon/task_status.go`

- [ ] **Step 1: Replace `internal/app/ports` imports with `internal/ports`**
- [ ] **Step 2: Replace `internal/app/scheduler` imports with `internal/scheduler`**
- [ ] **Step 3: Replace `internal/app/taskstatus` imports with `internal/taskstatus`**
- [ ] **Step 4: Replace `internal/app/crawler/fetcher` imports with `internal/crawler/fetcher` where only contract/factory access is needed**

### Task 3: Verify compile and targeted package behavior

**Files:**
- Test: `internal/platformbase`
- Test: `internal/platformtask`
- Test: `internal/shein/scheduler`
- Test: `internal/temu/scheduler`
- Test: `internal/taskstatus`

- [ ] **Step 1: Format changed files**

Run:

```powershell
gofmt -w <changed-files>
```

- [ ] **Step 2: Run targeted tests**

Run:

```powershell
go test ./internal/platformbase ./internal/platformtask ./internal/taskstatus ./internal/shein/scheduler ./internal/temu/scheduler -count=1
```

Expected: PASS

- [ ] **Step 3: Run broader compile-sensitive packages**

Run:

```powershell
go test ./internal/amazon ./internal/shein/pipeline ./internal/temu ./internal/temu/sync ./internal/product -count=1
```

Expected: PASS
