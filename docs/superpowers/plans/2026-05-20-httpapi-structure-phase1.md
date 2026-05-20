# HTTP API Structure Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce `internal/app/httpapi` centralization and remove duplicate API entrypoint drift without changing API behavior.

**Architecture:** Keep the existing `httpapi` package boundary for this phase, but split route registration and module-facing handler contracts by bounded context so `server.go` stops being the single expansion point. Keep runtime behavior unchanged and make `cmd/productenrich-api` explicitly delegate to the shared HTTP API startup path instead of pretending to be an independent service shape.

**Tech Stack:** Go, Gin, Logrus, existing package-local route descriptors and command entrypoints

---

### Task 1: Split HTTP route composition by module

**Files:**
- Modify: `internal/app/httpapi/server.go`
- Create: `internal/app/httpapi/routes_core.go`
- Create: `internal/app/httpapi/routes_products.go`
- Create: `internal/app/httpapi/routes_amazonlisting.go`
- Create: `internal/app/httpapi/routes_listingkit.go`
- Test: `internal/app/httpapi/server_test.go`

- [ ] **Step 1: Extract route descriptor builders into focused files**

Move route assembly out of `server.go` into focused helpers:
- core routes: `/health`
- product/image routes
- amazon listing routes
- listing kit routes, prompt routes, studio/session/login/task-rpc/sds routes

- [ ] **Step 2: Keep the public server bootstrap unchanged**

Retain the existing exported helpers and signatures:
- `RegisterRoutes`
- `RegisterRoutesWithPrompt`
- `buildHTTPServerBundleWithStudio`

They should now call small internal helper functions instead of owning all route declarations inline.

- [ ] **Step 3: Run focused route tests**

Run:

```powershell
go test ./internal/app/httpapi -run "Test(RegisterRoutes|BuildRouteDescriptors|Shein|SDS)" -count=1
```

Expected: PASS

### Task 2: Reduce handler contract sprawl in `types.go`

**Files:**
- Modify: `internal/app/httpapi/types.go`
- Test: `internal/app/httpapi/server_test.go`

- [ ] **Step 1: Break the oversized route handler interface into smaller embedded contracts**

Keep behavior unchanged, but replace the monolithic `listingKitRouteHandler` surface with smaller embedded interfaces grouped by responsibility, such as:
- task routes
- studio routes
- settings/store routes
- admin routes
- subscription/platform admin routes

- [ ] **Step 2: Preserve compatibility for existing handler implementations**

Keep `listingKitRouteHandler` as a composed interface so current handler implementations still satisfy the type without business-layer edits.

- [ ] **Step 3: Re-run focused package tests**

Run:

```powershell
go test ./internal/app/httpapi -run "Test(RegisterRoutes|BuildRouteDescriptors|ListingKit)" -count=1
```

Expected: PASS

### Task 3: Make duplicate API entrypoint explicit

**Files:**
- Modify: `cmd/productenrich-api/main.go`
- Optionally modify: `cmd/productenrich-api/README.md`
- Test: `cmd/productenrich-api`

- [ ] **Step 1: Convert the command into an explicit shared HTTP API alias**

Make the startup path mirror `cmd/product-listing-api` through a clearly named helper so future drift is obvious and minimized.

- [ ] **Step 2: Clarify command role**

If README wording is misleading, update it so the command is documented as a compatibility/alias entrypoint rather than an independently assembled API.

- [ ] **Step 3: Run targeted command/package tests**

Run:

```powershell
go test ./cmd/productenrich-api ./cmd/product-listing-api ./internal/app/httpapi -count=1
```

Expected: PASS
