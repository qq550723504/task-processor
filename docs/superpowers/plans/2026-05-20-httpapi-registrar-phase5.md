# HTTPAPI Registrar Phase 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move business route registrars out of `internal/app/httpapi` so the package focuses on bootstrap and route mounting.

**Architecture:** Introduce a neutral route descriptor package, let business domains own their route descriptor builders, and keep `internal/app/httpapi` responsible only for composing descriptors plus auth/mount middleware.

**Tech Stack:** Go, Gin

---

### Task 1: Extract shared route descriptor

**Files:**
- Create: `internal/httproute/descriptor.go`
- Modify: `internal/app/httpapi/types.go`

- [x] **Step 1: Add neutral route descriptor type**
- [x] **Step 2: Convert app/httpapi to use the shared descriptor alias**

### Task 2: Move domain registrars

**Files:**
- Create: `internal/productenrich/httpapi/routes.go`
- Create: `internal/amazonlisting/httpapi/routes.go`
- Create: `internal/listingkit/httpapi/routes.go`
- Delete: `internal/app/httpapi/routes_products.go`
- Delete: `internal/app/httpapi/routes_amazonlisting.go`
- Delete: `internal/app/httpapi/routes_listingkit.go`

- [x] **Step 1: Move product and image route registration**
- [x] **Step 2: Move Amazon listing route registration**
- [x] **Step 3: Move ListingKit route registration**

### Task 3: Rewire bootstrap and verify

**Files:**
- Modify: `internal/app/httpapi/server.go`
- Test: `internal/app/httpapi`
- Test: `internal/listingkit/...`
- Test: `internal/amazonlisting/...`
- Test: `internal/productenrich/...`
- Test: `internal/productimage/...`

- [x] **Step 1: Rewire app/httpapi to call domain registrars**
- [x] **Step 2: Format changed files**
- [x] **Step 3: Run targeted tests**

```powershell
go test ./internal/app/httpapi -count=1
go test ./internal/listingkit/... ./internal/amazonlisting/... ./internal/productenrich/... ./internal/productimage/... -count=1
```

Expected: PASS
