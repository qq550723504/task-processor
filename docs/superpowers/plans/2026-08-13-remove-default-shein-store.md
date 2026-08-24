# Remove ListingKit Default SHEIN Store Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans (recommended) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove implicit SHEIN store selection from ListingKit and require every SHEIN task to carry an explicit, tenant-authorized `shein_store_id`.

**Architecture:** Delete the ListingKit default-store configuration path at the service/config/settings boundaries. Keep store profiles and store access validation as per-store capabilities, while task creation becomes the only source of the selected SHEIN store. Settings health will validate non-store SHEIN configuration only.

**Tech Stack:** Go, Gin, existing ListingKit service tests, TypeScript/React settings tests, PowerShell/Git verification.

## Global Constraints

- Do not remove SHEIN store records, store profiles, explicit `shein_store_id` fields, or tenant/platform ownership checks.
- Do not add a global selected-store, current-store, or fallback-store replacement.
- A SHEIN task without a positive `shein_store_id` must fail before persistence or dispatch with `shein_store_id is required for SHEIN tasks`.
- Non-SHEIN task behavior and 1688 `source_account_id` behavior remain unchanged.
- Do not run deployment, K8s mutations, or real-provider task creation.
- Follow TDD for each behavior change: add the failing test, run it, implement the smallest change, then rerun focused tests.

---

### Task 1: Remove the default-store service and configuration contract

**Files:**
- Modify: `internal/listingkit/service_types.go`
- Modify: `internal/listingkit/service_config.go`
- Modify: `internal/listingkit/service_config_groups.go`
- Modify: `internal/listingkit/service_defaults.go`
- Modify: `internal/listingkit/service_task_wiring.go`
- Modify: `internal/listingkit/service_task_wiring_support.go`
- Modify: `internal/listingkit/task_dependencies.go`
- Modify: `internal/listingkit/task_lifecycle_service.go`
- Modify: `internal/listingkit/task_lifecycle_service_support.go`
- Modify: `internal/listingkit/request_defaults.go` (delete when no references remain)
- Modify: `internal/listingkit/httpapi/bootstrap_submit_module.go`
- Modify: `internal/listingkit/httpapi/bootstrap_service_config.go`
- Test: `internal/listingkit/service_test.go`
- Test: `internal/listingkit/httpapi/bootstrap_test.go`

**Interfaces:**
- Consume: existing `GenerateRequest`, `ServiceConfig`, and task lifecycle wiring.
- Produce: a ListingKit service with no `SheinDefaultStoreID`, no `generateRequestDefaults`, and no request-default callback.

- [ ] **Step 1: Write the failing contract tests**

Add a boundary assertion that `ServiceSheinDependencies` has no `SheinDefaultStoreID` field and that task lifecycle wiring does not contain a request-default callback. Replace the old tests that invoke `applyGenerateRequestDefaults` with a test demonstrating that request preparation leaves an omitted `shein_store_id` at zero before validation.

- [ ] **Step 2: Run the focused tests and verify the expected failure**

Run:

```powershell
go test ./internal/listingkit -run 'TestApplyGenerateRequestDefaults|TestBuildListingKit|TestBuildTaskModule' -count=1
```

Expected result: the new boundary/explicit-selection assertions fail against the existing default-store fields or helper.

- [ ] **Step 3: Remove the configuration and wiring path**

Remove `SheinDefaultStoreID` from `ServiceSheinDependencies`, remove `defaultStoreID` from the HTTP API submit module, stop passing it through `buildListingKitSheinDependencies`, and change `defaultSheinSettings` to accept only the pricing policy. Remove `generateRequestDefaults`, its builder/resolver, the task dependency field, and the lifecycle callback. Delete `applyGenerateRequestDefaults` and its now-unused file after all references are gone.

- [ ] **Step 4: Run focused tests and compile the affected packages**

Run:

```powershell
gofmt -w (rg --files internal/listingkit internal/listingkit/httpapi -g '*.go')
go test ./internal/listingkit ./internal/listingkit/httpapi -count=1
```

Expected result: the service/config packages compile and all remaining failures are stale assertions about the removed field, which are fixed in the next task.

- [ ] **Step 5: Commit the isolated contract change**

```powershell
git add internal/listingkit internal/listingkit/httpapi
git commit -m "refactor: remove default SHEIN store wiring"
```

### Task 2: Require explicit SHEIN store selection at task creation

**Files:**
- Modify: `internal/listingkit/task_lifecycle_service_support.go`
- Modify: `internal/listingkit/service_task_wiring.go`
- Test: `internal/listingkit/service_task_lifecycle_test.go`
- Test: `internal/listingkit/service_test.go`
- Test: `internal/listingkit/workflow_requests_test.go`

**Interfaces:**
- Consume: normalized `GenerateRequest` and existing `validateRequestedSheinStoreAccess`.
- Produce: validation that rejects a SHEIN request with `SheinStoreID <= 0` before `Repository.CreateTask` or dispatch.

- [ ] **Step 1: Add the failing lifecycle test**

Create a task lifecycle fixture whose repository records whether `CreateTask` was called. Submit a request such as:

```go
&GenerateRequest{
    ProductURL: "https://example.test/product",
    Platforms:  []string{"shein"},
}
```

Assert that the error is exactly `invalid request: shein_store_id is required for SHEIN tasks` and that the repository was not called. Add a companion test proving a non-SHEIN request with no SHEIN store remains valid.

- [ ] **Step 2: Run the lifecycle tests and verify they fail**

Run:

```powershell
go test ./internal/listingkit -run 'Test.*(Generate|TaskLifecycle|Request)' -count=1
```

Expected result: the missing-store SHEIN request currently proceeds to later validation or repository setup, so the new assertion fails.

- [ ] **Step 3: Implement the explicit-store validation**

After request normalization and before `validateRequestedSheinStoreAccess`, add a helper in `task_lifecycle_service_support.go` that returns the exact error when `generateRequestTargetsPlatform(req, "shein")` is true and `req.SheinStoreID <= 0`. Call it from `prepareGenerateTask`; leave non-SHEIN requests unchanged.

- [ ] **Step 4: Run the focused lifecycle and API request tests**

Run:

```powershell
gofmt -w internal/listingkit/task_lifecycle_service_support.go
go test ./internal/listingkit -run 'Test.*(Generate|TaskLifecycle|Request)' -count=1
go test ./internal/listingkit/api -run 'Test.*Generate' -count=1
```

Expected result: missing-store SHEIN creation is rejected before persistence, explicit store requests still reach existing tenant/platform access validation, and non-SHEIN requests are unaffected.

- [ ] **Step 5: Commit the task boundary change**

```powershell
git add internal/listingkit
git commit -m "feat: require explicit SHEIN store selection"
```

### Task 3: Remove default store from settings, health, and HTTP schema

**Files:**
- Modify: `internal/listingkit/model_request_submit_support.go`
- Modify: `internal/listingkit/settings_admin_shein_settings_service.go`
- Modify: `internal/listingkit/settings_health.go`
- Modify: `internal/listingkit/api/settings_service.go`
- Test: `internal/listingkit/shein_settings_test.go`
- Test: `internal/listingkit/settings_health_test.go`
- Test: `internal/listingkit/api/settings_namespace_handler_test.go`
- Test: `internal/listingkit/api/handler_dependencies_test.go`

**Interfaces:**
- Consume: existing tenant SHEIN settings and settings-health endpoint.
- Produce: settings without `DefaultStoreID`; health checks for site, stock, submit mode, and pricing only; schema without `default_store_id`.

- [ ] **Step 1: Add failing settings and health assertions**

Update the settings tests to construct `SheinSettings` without a store and assert:

```go
health := BuildSettingsHealth(SettingsHealthInputs{
    Shein: &SheinSettings{Site: "US", DefaultStock: 20, DefaultSubmitMode: "publish"},
})
```

The `shein.account` item must be `ready`. Add assertions that the item message and action contain neither `默认店铺` nor `default store`. Add an HTTP schema assertion that the SHEIN namespace fields do not include `default_store_id`.

- [ ] **Step 2: Run the settings tests and verify they fail**

Run:

```powershell
go test ./internal/listingkit -run 'Test.*(Settings|Health)' -count=1
go test ./internal/listingkit/api -run 'Test.*Settings' -count=1
```

Expected result: compilation fails on `DefaultStoreID` after the model assertion changes, and the existing health implementation still reports the default-store blocker.

- [ ] **Step 3: Implement the settings and health change**

Remove `DefaultStoreID` from `SheinSettings`. Remove the update assignment in `settings_admin_shein_settings_service.go`. Remove the field from the SHEIN settings namespace schema and change its description to describe site, warehouse, inventory, submit mode, and pricing without a default store. Update `sheinAccountHealthItem` to omit store validation and use explicit-store wording for its impact/action.

- [ ] **Step 4: Verify unknown legacy input cannot mutate state**

Keep the existing JSON decoder behavior: a request containing `default_store_id` must not alter any setting because the model no longer has that field. Add a handler test that sends `{"default_store_id": 9, "site": "GB"}` and asserts the response has no `default_store_id` property and the updated settings contain `Site == "GB"` without any store selection field.

- [ ] **Step 5: Run focused settings and health tests**

Run:

```powershell
gofmt -w internal/listingkit/model_request_submit_support.go internal/listingkit/settings_admin_shein_settings_service.go internal/listingkit/settings_health.go internal/listingkit/api/settings_service.go
go test ./internal/listingkit -run 'Test.*(Settings|Health)' -count=1
go test ./internal/listingkit/api -run 'Test.*(Settings|Health)' -count=1
```

Expected result: the account health item is not blocked by missing store selection, and the HTTP settings contract no longer exposes the removed field.

- [ ] **Step 6: Commit the settings boundary change**

```powershell
git add internal/listingkit
git commit -m "refactor: remove default SHEIN store setting"
```

### Task 4: Preserve explicit store profile and submission behavior

**Files:**
- Modify: `internal/listingkit/service_submit_settings_resolution.go`
- Test: `internal/listingkit/service_submit_store_context_test.go`
- Test: `internal/listingkit/store_profile_service_test.go`
- Test: `internal/listingkit/service_shein_store_client_test.go`
- Test: `internal/listingkit/task_submission_execution_service_test.go`

**Interfaces:**
- Consume: explicit task `SheinStoreID`, persisted store-resolution snapshot, and per-store profile lookup.
- Produce: profile settings that configure the selected store without assigning a removed default-store field; missing store selection still fails at runtime for already persisted invalid tasks.

- [ ] **Step 1: Add the failing profile-resolution assertion**

Change the profile test fixture to use base `SheinSettings` without a default store and an explicit task/profile store. Assert the resolved profile contributes site, warehouse, stock, submit mode, and pricing, while the store ID comes only from the task or snapshot resolver.

- [ ] **Step 2: Run the focused submission tests and verify the stale field failure**

Run:

```powershell
go test ./internal/listingkit -run 'Test.*(Submit|Store|Profile|Client)' -count=1
```

Expected result: the old `settings.DefaultStoreID` assertions fail because the store identity must now be asserted through `resolveSheinStoreID` or the task snapshot.

- [ ] **Step 3: Remove profile-to-default-store assignment**

Delete only the `settings.DefaultStoreID = profile.StoreID` assignment from `applySubmitSettingsProfile`. Keep profile lookup and all other profile overlays. Do not alter `resolveStoreID`, which already prefers the persisted snapshot and explicit task request.

- [ ] **Step 4: Run submission and store tests**

Run:

```powershell
gofmt -w internal/listingkit/service_submit_settings_resolution.go
go test ./internal/listingkit -run 'Test.*(Submit|Store|Profile|Client)' -count=1
```

Expected result: explicit task/snapshot store resolution remains intact and no profile can reintroduce a global store selection.

- [ ] **Step 5: Commit the explicit profile behavior**

```powershell
git add internal/listingkit
git commit -m "refactor: keep SHEIN profiles store-scoped"
```

### Task 5: Update boundary tests, documentation, and run full verification

**Files:**
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Modify: `internal/listingkit/httpapi/settings_health_probes_test.go`
- Modify: `internal/listingkit/phase102_model_request_support_boundary_test.go`
- Modify: `internal/listingkit/service_test.go`
- Modify: `internal/listingkit/service_submit_store_context_test.go`
- Modify: `internal/listingkit/shein_settings_test.go`
- Modify: `internal/listingkit/settings_health_test.go`
- Modify: `internal/listingkit/api/handler_dependencies_test.go`
- Modify: `internal/listingkit/api/settings_namespace_handler_test.go`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Test: no UI production file is expected to change; the current UI settings components contain no SHEIN default-store field or wording and will be verified by the scoped search

**Interfaces:**
- Consume: all changed Go and UI contracts.
- Produce: a clean repository-wide contract with no ListingKit default-store symbol or user-facing wording, while unrelated infrastructure defaults remain untouched.

- [ ] **Step 1: Add the boundary/search regression test**

Add a focused test or update the existing phase boundary test so ListingKit source files do not contain `SheinDefaultStoreID`, `DefaultStoreID`, `default_store_id`, or the default-store health wording. Scope the assertion to `internal/listingkit` and its HTTP API; do not scan Temporal or unrelated platform code. Separately run the same search over `web/listingkit-ui/src` to verify the already-audited settings UI has no obsolete control or copy.

- [ ] **Step 2: Run the boundary test and inspect remaining matches**

Run:

```powershell
go test ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi -run 'Test.*Boundary|Test.*Settings|Test.*Health' -count=1
rg -n -i "SheinDefaultStoreID|DefaultStoreID|default_store_id|默认店铺|default store" internal/listingkit web/listingkit-ui/src
```

Expected result: the scoped search returns no ListingKit production references; any remaining test-only references are removed rather than suppressed.

- [ ] **Step 3: Run Go formatting, static checks, and the full suite**

Run:

```powershell
gofmt -w (rg --files internal/listingkit internal/listingkit/httpapi -g '*.go')
git diff --check
GOWORK=off go test ./... -count=1 -timeout=30m
```

Expected result: all Go packages pass, with no formatting or whitespace errors. If an unrelated baseline failure appears, record its package and prove it is outside the changed paths before reporting it.

- [ ] **Step 4: Review the final diff and commit verification updates**

Run:

```powershell
git diff --stat origin/master...HEAD
git diff --name-only origin/master...HEAD
git status --short --branch
```

Confirm only the explicit-store change, its tests, and directly necessary documentation are present, then commit:

```powershell
git add internal/listingkit web/listingkit-ui docs
git commit -m "test: verify explicit SHEIN store selection"
```

After this plan is complete, request code review before any push or pull request. Deployment and live 1688/SHEIN acceptance remain separate, explicitly authorized steps.
