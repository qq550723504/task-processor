# Runtime Decoupling Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove remaining business imports of `internal/app/state` and `internal/app/processor` by introducing neutral runtime package paths.

**Architecture:** This phase adds top-level compatibility packages for state and processor contracts/constructors. Existing implementations stay in `internal/app/*` for now, but platform and business packages stop depending on the `app` namespace directly.

**Tech Stack:** Go, existing processor/state runtime implementations

---

### Task 1: Add top-level runtime compatibility packages

**Files:**
- Create: `internal/state/compat.go`
- Create: `internal/processor/compat.go`

- [ ] **Step 1: Re-export state types and constructors through `internal/state`**
- [ ] **Step 2: Re-export processor types and constructors through `internal/processor`**

### Task 2: Migrate remaining business imports

**Files:**
- Modify: `internal/amazon/*.go`
- Modify: `internal/shein/**/*.go`
- Modify: `internal/temu/**/*.go`

- [ ] **Step 1: Replace `internal/app/state` with `internal/state`**
- [ ] **Step 2: Replace `internal/app/processor` with `internal/processor`**

### Task 3: Verify migrated packages

**Files:**
- Test: `internal/amazon`
- Test: `internal/shein/pipeline`
- Test: `internal/temu`

- [ ] **Step 1: Format changed files**
- [ ] **Step 2: Run targeted tests**

```powershell
go test ./internal/amazon ./internal/shein/pipeline ./internal/temu -count=1
```

Expected: PASS
