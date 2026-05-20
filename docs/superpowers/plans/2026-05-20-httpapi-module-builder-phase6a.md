# HTTPAPI Module Builder Phase 6A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce `internal/app/httpapi/modules.go` by moving the independent module builders out of the centralized bootstrap package.

**Architecture:** Extract self-contained module builders into the owning business packages first (`productenrich`, `amazonlisting`), add a tiny shared worker-pool helper, and keep `image` / `listingKit` for a follow-up slice because they still depend on broader shared assembly helpers.

**Tech Stack:** Go, Gin, existing worker pool and repository patterns

---

### Task 1: Extract shared worker-pool helper

**Files:**
- Create: `internal/httpbootstrap/pool.go`

- [x] **Step 1: Move reusable worker pool config and submitter into a neutral helper package**

### Task 2: Move independent business module builders

**Files:**
- Create: `internal/productenrich/httpapi/bootstrap.go`
- Create: `internal/amazonlisting/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/modules.go`

- [x] **Step 1: Move productenrich module builder and repository/bootstrap helpers**
- [x] **Step 2: Move amazonlisting module builder and repository/bootstrap helpers**
- [x] **Step 3: Rewire app/httpapi to delegate to the new builders**

### Task 3: Collapse dead centralized wrappers

**Files:**
- Modify: `internal/app/httpapi/modules.go`
- Modify: `internal/app/httpapi/types.go`

- [x] **Step 1: Inline prompt/template and task RPC construction into bootstrap**
- [x] **Step 2: Remove unused prompt module type and dead centralized helper functions**

### Task 4: Verify extracted builders

**Files:**
- Test: `internal/app/httpapi`
- Test: `internal/productenrich/...`
- Test: `internal/amazonlisting/...`

- [x] **Step 1: Format changed files**
- [x] **Step 2: Run targeted tests**

```powershell
go test ./internal/app/httpapi ./internal/productenrich/... ./internal/amazonlisting/... -count=1
```

Expected: PASS

**Follow-up:** Extract `productimage` and `listingKit` module builders in a separate slice once the remaining shared helper boundaries are clarified.
