# Runtime Decoupling Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the runtime implementations for processor and state out of `internal/app/*` into top-level packages so the `app` namespace stops owning shared runtime code.

**Architecture:** `internal/state` and `internal/processor` become the canonical home for runtime implementations. `internal/app/state` and `internal/app/processor` are reduced to thin compatibility layers for legacy imports.

**Tech Stack:** Go, existing runtime implementations

---

### Task 1: Move state implementation ownership

**Files:**
- Create: `internal/state/*.go`
- Create: `internal/state/daily_count_manager_test.go`
- Modify: `internal/app/state/*`

- [ ] **Step 1: Copy runtime implementation into `internal/state`**
- [ ] **Step 2: Move runtime test ownership into `internal/state`**
- [ ] **Step 3: Replace `internal/app/state` implementation with compatibility aliases**

### Task 2: Move processor implementation ownership

**Files:**
- Create: `internal/processor/*.go`
- Modify: `internal/app/processor/*`

- [ ] **Step 1: Copy processor/runtime implementation into `internal/processor`**
- [ ] **Step 2: Replace `internal/app/processor` implementation with compatibility aliases**

### Task 3: Update remaining callers and verify

**Files:**
- Test: `internal/state`
- Test: `internal/processor`
- Test: `internal/app/consumer`
- Test: `internal/amazon`
- Test: `internal/shein/pipeline`
- Test: `internal/temu`

- [ ] **Step 1: Point remaining callers at top-level runtime packages**
- [ ] **Step 2: Format changed files**
- [ ] **Step 3: Run targeted tests**

```powershell
go test ./internal/state ./internal/processor ./internal/app/consumer ./internal/amazon ./internal/shein/pipeline ./internal/temu -count=1
```

Expected: PASS
