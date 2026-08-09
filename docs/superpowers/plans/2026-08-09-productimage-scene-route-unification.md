# ProductImage Scene Route Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make ProductImage scene governance resolve and invoke the existing `image_gpt_image_2` tenant/user client consistently.

**Architecture:** Keep the provider-neutral router in `internal/productimage/httpapi`, use one canonical credential reference (`image_gpt_image_2`), and bind the governed scene generator to the same logical OpenAI Manager client. The Manager may expose an unpreconfigured logical client only when a resolver exists; actual calls remain fail-closed on missing resolved credentials.

**Tech Stack:** Go, GORM-backed credential resolver, OpenAI-compatible image client, table-driven Go tests.

## Global Constraints

- Default `productImageSceneEnabled=false` remains unchanged.
- Do not migrate, copy, print, or expose API keys.
- Legacy Studio routing remains unchanged when scene governance is disabled.
- Unsupported API styles fail closed.

---

### Task 1: Resolver-backed logical image client

**Files:**
- Modify: `internal/infra/clients/openai/client_manager.go`
- Test: `internal/infra/clients/openai/tenant_manager_test.go`

**Interfaces:**
- Consumes: `Manager.configResolver`, `GetImageClient`, `resolveClientWithSelection`.
- Produces: `GetImageClient(name)` accepts an unregistered logical name only when a resolver is configured; resolver-backed calls still require a resolved config.

- [ ] **Step 1: Add a failing test** asserting `GetImageClient("image_gpt_image_2")` succeeds with a resolver and returns an image client that honors `ImageRouteSelection{CredentialReference: "image_gpt_image_2"}`.
- [ ] **Step 2: Run `go test ./internal/infra/clients/openai -run 'Test.*Logical.*Image|Test.*Route' -count=1` and verify the new test fails because `GetImageClient` rejects the unregistered name.
- [ ] **Step 3: Change `GetImageClient` so unknown names are rejected only when `configResolver == nil`; keep the existing static-client rejection for managers without a resolver.
- [ ] **Step 4: Run the focused package tests and verify they pass.

### Task 2: Canonical ProductImage scene credential

**Files:**
- Modify: `internal/productimage/httpapi/ai_capability_scene_catalog.go`
- Test: `internal/productimage/httpapi/ai_capability_scene_catalog_test.go`

**Interfaces:**
- Consumes: `openaiclient.ClientConfigResolver.ResolveClientConfig`.
- Produces: `BuildProductImageSceneCapabilityRouter` resolves `image_gpt_image_2` and returns that exact credential reference in `RouteDecision`.

- [ ] **Step 1: Update the catalog test fixture to expect `image_gpt_image_2` as the resolver lookup and `CredentialReference`.
- [ ] **Step 2: Run `go test ./internal/productimage/httpapi -run 'Test.*Capability.*Scene|Test.*Scene.*Catalog' -count=1` and verify the old `image` expectation fails.
- [ ] **Step 3: Add a package constant `productImageSceneClientName = "image_gpt_image_2"` and use it for resolver lookup and route decision output; keep the existing OpenAI-compatible API-style allowlist.
- [ ] **Step 4: Add coverage for a missing resolver-backed credential and an unsupported API style, then run the focused tests.

### Task 3: Bind the governed provider to the canonical client

**Files:**
- Modify: `internal/productimage/httpapi/model_provider_builder.go`
- Test: `internal/productimage/httpapi/model_provider_defaults_test.go`

**Interfaces:**
- Consumes: `config.AICapability.ProductImageSceneEnabled`, `openaiclient.Manager.GetImageClient`.
- Produces: When governance is enabled, the scene generator is built from `image_gpt_image_2`; when disabled, existing legacy client selection remains unchanged.

- [ ] **Step 1: Add a recording Manager test that captures the requested image client name and asserts governance-enabled construction requests `image_gpt_image_2`.
- [ ] **Step 2: Run the focused builder test and verify it fails against the current hard-coded `image` selection/static Nano Banana branch.
- [ ] **Step 3: Gate the legacy static Nano Banana branch behind `!cfg.AICapability.ProductImageSceneEnabled`; for governance use the resolver-backed `image_gpt_image_2` logical client and build the OpenAI-compatible editor/generator from it.
- [ ] **Step 4: Keep the existing fail-closed behavior when the resolver-backed client cannot be built, and run the ProductImage HTTP API tests.

### Task 4: Verification and handoff

**Files:**
- No additional source files.

- [ ] **Step 1: Run `go test ./internal/infra/clients/openai ./internal/productimage ./internal/productimage/httpapi ./internal/app/httpapi -count=1`.
- [ ] **Step 2: Run `go test ./... -count=1` with the repository’s normal timeout budget.
- [ ] **Step 3: Run `git diff --check` and inspect the diff for credential values, feature-flag changes, or unrelated edits.
- [ ] **Step 4: Commit only the implementation and test files with message `fix: unify product image scene route credential`.

