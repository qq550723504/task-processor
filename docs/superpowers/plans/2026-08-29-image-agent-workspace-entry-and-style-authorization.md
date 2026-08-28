# Image Agent Workspace Entry and Style Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create an eligible tenant’s manual image run from the workspace and snapshot only explicitly selected style assets.

**Architecture:** ListingKit owns owned-task preflight and server-side run construction; Image Agent owns lifecycle validation. The browser submits selected style IDs only and navigates with the returned run ID.

**Tech Stack:** Go, Gin, Next.js, React, Zod, Vitest, testify.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-workspace-entry-and-style-authorization-design.md`

## Global Constraints

- Browser never provides run IDs, plans, budgets, tenant/user IDs, or idempotency keys.
- Styles are selected safe non-source task assets; never inferred from kind or role.
- Initial budget enables only `max_images=1`; eligibility is checked before persistence.

---

### Task 1: Catalog translation and tenant admission

**Files:**
- Modify: `internal/listingkit/httpapi/image_agent_asset_catalog.go`
- Modify: `internal/listingkit/httpapi/image_agent_asset_catalog_test.go`
- Modify: `internal/imageagent/service.go`
- Test: `internal/imageagent/service_commands_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestImageAgentCatalogIncludesOnlyExplicitNonSourceStyles(t *testing.T) {}
func TestServiceStartRejectsProviderIneligibleTenantBeforeInitializeRun(t *testing.T) {}
func TestImageAgentCatalogTruncatesDisplayLabelAt256Runes(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/listingkit/httpapi ./internal/imageagent -run '^(TestImageAgentCatalogIncludesOnlyExplicitNonSourceStyles|TestServiceStartRejectsProviderIneligibleTenantBeforeInitializeRun|TestImageAgentCatalogTruncatesDisplayLabelAt256Runes)$' -count=1`

Expected: FAIL because selected style IDs, eligibility port, and label bound do not exist.

- [ ] **Step 3: Implement minimal contracts**

Add selected-style catalog input, 256-rune display-label normalization, and injected `RunEligibility` checked in `Service.Start` before catalog/projection initialization.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/listingkit/httpapi ./internal/imageagent -count=1`

Expected: PASS.

Run: `git add internal/listingkit/httpapi/image_agent_asset_catalog.go internal/listingkit/httpapi/image_agent_asset_catalog_test.go internal/imageagent/service.go internal/imageagent/service_commands_test.go` then `git commit -m "feat: authorize explicit image styles"`.

### Task 2: Task-scoped preflight and create API

**Files:**
- Modify: `internal/listingkit/httpapi/routes_descriptor_task.go`
- Modify: `internal/listingkit/httpapi/image_agent_*.go`
- Test: `internal/listingkit/httpapi/image_agent_*_test.go`

- [ ] **Step 1: Write failing endpoint tests**

```go
func TestCreateTaskImageAgentRunStartsServerOwnedDefault(t *testing.T) {}
func TestCreateTaskImageAgentRunRejectsUnknownOrSourceStyle(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/listingkit/httpapi -run '^(TestCreateTaskImageAgentRunStartsServerOwnedDefault|TestCreateTaskImageAgentRunRejectsUnknownOrSourceStyle)$' -count=1`

Expected: FAIL because task-scoped routes do not exist.

- [ ] **Step 3: Implement server orchestration**

Register the exact GET preflight and POST create routes, generate server identities, create revision-one single-main plan and one-image budget, then call existing `imageagent.Application.Start`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/listingkit/httpapi -count=1`

Expected: PASS.

Run: stage only named image-agent adapter/test files then `git commit -m "feat: create image runs from workspace tasks"`.

### Task 3: Workspace create-and-navigate UI

**Files:**
- Modify: `web/listingkit-ui/src/lib/api/image-agent.ts`
- Modify: `web/listingkit-ui/src/app/api/listing-kits/proxy-url.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen.tsx`
- Test: matching `*.test.ts` and `*.test.tsx`

- [ ] **Step 1: Write failing UI test**

```tsx
it("creates a run with selected styles and navigates to its run id", async () => {})
```

- [ ] **Step 2: Verify RED**

Run: `npm.cmd test -- workspace-screen-product-layout.test.tsx`

Expected: FAIL because no create action or task-scoped proxy route exists.

- [ ] **Step 3: Implement and verify**

Add typed preflight/create clients, exact BFF allow-list entries, pending-safe create dialog, and navigation that preserves query state while adding `image_agent_run_id`.

Run: `npm.cmd test -- workspace-screen-product-layout.test.tsx image-agent-workbench.test.tsx`

Expected: PASS.

- [ ] **Step 4: Commit**

Run: stage only named API/proxy/workspace/test paths then `git commit -m "feat: open image agent from workspace"`.
