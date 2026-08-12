# ProductImage Scene Tenant Allowlist Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restrict the ProductImage scene-governance canary to explicitly allowlisted tenant IDs before any real model call is enabled.

**Architecture:** Keep the existing process-wide `productImageSceneEnabled` switch, but require a non-empty tenant allowlist whenever it is enabled. Pass the normalized allowlist into the ProductImage policy resolver; tenants outside it receive `policy_denied` before credential resolution or provider invocation. Production configuration will add only the verified `zone` tenant ID after the code is deployed.

**Tech Stack:** Go, Viper configuration, existing `internal/aicapability` router, YAML/Kubernetes ConfigMap, Go tests.

## Global Constraints

- Default behavior remains unchanged while `productImageSceneEnabled=false`.
- An enabled governance switch with an empty allowlist must fail closed at configuration/bootstrap validation.
- No API key, prompt, image bytes, or raw provider response may enter route decisions or logs.
- No real model call is made by automated tests.
- ProductImage must not import ListingKit facade packages.

---

### Task 1: Add configuration and fail-closed validation

**Files:**
- Modify: `internal/core/config/type_ai_capability.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/defaults.go`
- Modify: `internal/core/config/validator_ai_capability.go` (or the existing AI capability validator file)
- Modify: `internal/core/config/ai_capability_test.go`
- Modify: `config/config-dev.yaml`
- Modify: `config/config-test.yaml`
- Modify: `config/config-prod.yaml`

**Interfaces:**
- Add `ProductImageSceneAllowedTenantIDs []string` to `config.AICapabilityConfig`.
- Bind `aiCapability.productImageSceneAllowedTenantIDs` to `TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS` using the repository’s existing string-slice environment parsing.

- [ ] **Step 1: Write failing tests** for default empty allowlist, comma-separated environment parsing, and enabled-with-empty-list rejection.
- [ ] **Step 2: Run** `go test ./internal/core/config -run 'Test.*ProductImageScene|Test.*AICapability' -count=1` and confirm the new assertions fail.
- [ ] **Step 3: Implement** the field, default, env binding, normalization, and fail-closed validation.
- [ ] **Step 4: Run** the focused config tests and `go test ./internal/core/config -count=1`.
- [ ] **Step 5: Update** all three YAML examples with an empty allowlist and keep the governance flag false.

### Task 2: Enforce tenant allowlist in ProductImage routing

**Files:**
- Modify: `internal/productimage/httpapi/ai_capability_scene_catalog.go`
- Modify: `internal/productimage/httpapi/ai_capability_scene_catalog_test.go`
- Modify: `internal/productimage/httpapi/scene_governance_builder.go`
- Modify: `internal/productimage/httpapi/scene_governance_builder_test.go`

**Interfaces:**
- Change the router construction to accept `allowedTenantIDs []string`.
- `productImageScenePolicyResolver` stores a normalized set and returns `ErrorPolicyDenied` for a tenant not in that set.

- [ ] **Step 1: Write failing tests** proving an allowlisted tenant routes, a non-allowlisted tenant is denied before resolver lookup, and an empty list denies every tenant.
- [ ] **Step 2: Run** `go test ./internal/productimage/httpapi -run 'Test.*Scene.*Catalog|Test.*Governed.*Builder' -count=1` and confirm failure.
- [ ] **Step 3: Implement** normalized allowlist matching and pass the config list from the governed builder.
- [ ] **Step 4: Run** the focused ProductImage HTTP API tests.

### Task 3: Verify bootstrap and boundary behavior

**Files:**
- Modify: `internal/productimage/httpapi/bootstrap.go`
- Modify: `internal/productimage/httpapi/bootstrap_test.go` or the existing scene bootstrap test file
- Modify: `internal/app/httpapi/runtime_ai_capability_test.go`
- Modify: `tests/import_boundaries_test.go` only if a new import is introduced and needs an explicit boundary assertion

- [ ] **Step 1: Add tests** that enabled governance requires a non-empty allowlist and that disabled governance preserves the legacy provider path.
- [ ] **Step 2: Run** `go test ./internal/productimage/httpapi ./internal/app/httpapi ./tests -count=1` and confirm the intended failure before implementation.
- [ ] **Step 3: Implement** the bootstrap wiring without adding any ProductImage-to-ListingKit facade dependency.
- [ ] **Step 4: Run** the same packages and `git diff --check`.

### Task 4: Prepare the production canary configuration

**Files:**
- Modify: `deployments/kubernetes/listingkit-workbench/base/configmap.yaml`
- Modify: `internal/productimage/README.md`

- [ ] **Step 1: Add** `TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ALLOWED_TENANT_IDS` as a documented, comma-separated setting while leaving the governance flag `false` in the committed baseline.
- [ ] **Step 2: Document** that only the verified `zone` tenant ID `373211199677923496` may be added for the canary, and that enabling without an allowlist fails closed.
- [ ] **Step 3: Render/dry-run** the Kubernetes manifests and verify no secret values are introduced.

### Task 5: Full verification and PR handoff

- [ ] **Step 1:** Run `go test ./internal/aicapability ./internal/core/config ./internal/productimage ./internal/productimage/httpapi ./internal/app/httpapi ./tests -count=1`.
- [ ] **Step 2:** Run `go test ./...`.
- [ ] **Step 3:** Run `git diff --check` and inspect the exact staged paths.
- [ ] **Step 4:** Create a dedicated branch, commit only this plan’s implementation files, push, and open a Draft PR.
- [ ] **Step 5:** Do not enable production until CI passes and the PR is merged; then set the production allowlist to the zone tenant and run the explicitly authorized canary.
