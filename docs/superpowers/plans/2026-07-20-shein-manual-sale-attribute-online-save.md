# SHEIN Manual Sale Attribute Online Save Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every manual SHEIN sale-attribute save validate the live template, resolve hand-entered values online, and preserve custom text after refresh.

**Architecture:** The ListingKit revision service owns the online validation boundary. It always fetches the live SHEIN template, rejects stale selected IDs, and resolves custom text through the existing validation/create-value workflow before applying a revision. The review-card initialization distinguishes a template value from a persisted custom value so it can restore the correct control state.

**Tech Stack:** Go, existing SHEIN attribute client, React/TypeScript, Vitest, Go test.

## Global Constraints

- Never persist a manual sale-attribute revision when a SHEIN template, validation, or custom-value creation call fails.
- Every manual save fetches the current SHEIN attribute template.
- Existing selected `attribute_value_id` values must be validated against that live template.
- Persisted custom values outside the template must rehydrate into text inputs.

---

### Task 1: Enforce live SHEIN validation in the revision service

**Files:**
- Modify: `internal/listingkit/service_revision_manual_sale_attributes.go`
- Modify: `internal/listingkit/service_revision_manual_sale_attributes_assignment_support.go`
- Test: `internal/listingkit/service_revision_test.go`

**Interfaces:**
- Consumes: `AttributeAPI.GetAttributeTemplates`, `ValidateCustomAttributeValue`, and `AddCustomAttributeValue`.
- Produces: `resolveManualSheinSaleAttributeValueIDs` either returns fully SHEIN-confirmed IDs or an error before `applyListingKitRevision` persists a change.

- [ ] **Step 1: Write failing Go tests**

Add tests that construct a manual revision with already-populated value IDs and assert the attribute API template method is called once. Add a stale-ID case where the live template excludes the submitted ID and assert `ApplyTaskRevision` returns an error without repository update. Add a custom-text case with an old local assignment and assert the validation and add-custom callbacks receive the submitted text rather than the cached assignment.

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/listingkit -run 'Test.*Manual.*Sale.*(Live|Stale|Custom)' -count=1 -vet=off`

Expected: FAIL because selected IDs are not checked against a current template and cached assignments can bypass online resolution.

- [ ] **Step 3: Implement the smallest online-validation boundary**

Make `resolveManualSheinSaleAttributeValueIDs` always build the store-scoped attribute API and call `GetAttributeTemplates(categoryID)`. Validate every non-nil submitted value ID against the matching live attribute's `AttributeValueInfoList`. Remove the cached-assignment backfill from the manual-save path so text values proceed through `ResolveSingleSaleAttributeValue`, which invokes validate/create as required. Keep source-value fallback only for genuinely absent optional SKU attributes.

- [ ] **Step 4: Run focused Go tests and verify GREEN**

Run: `go test ./internal/listingkit -run 'Test.*Manual.*Sale.*(Live|Stale|Custom)' -count=1 -vet=off`

Expected: PASS with one live template read per manual save, stale IDs rejected, and custom text resolved online.

- [ ] **Step 5: Commit the backend slice**

Run: `git add internal/listingkit/service_revision_manual_sale_attributes.go internal/listingkit/service_revision_manual_sale_attributes_assignment_support.go internal/listingkit/service_revision_test.go && git commit -m "fix: validate manual SHEIN sale attributes online"`

### Task 2: Rehydrate persisted custom sale-attribute text

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`

**Interfaces:**
- Consumes: `patch.sale_attribute` and `skuPatch.sale_attributes` from `SheinEditorContext`.
- Produces: `ManualSaleAttributeSelection` with either `{ valueId, textValue: "" }` for template values or `{ textValue }` for persisted custom values.

- [ ] **Step 1: Write a failing Vitest case**

Create a review-card fixture whose source Size is `1PCS`, whose persisted SKU sale attribute is custom Size `11`, and whose live template has no value ID for `11`. Assert the text input has value `11`, and clicking save passes `textValue: "11"` rather than `1PCS`.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `web/listingkit-ui/node_modules/.bin/vitest.cmd run src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`

Expected: FAIL because initialization derives text only from the source attribute.

- [ ] **Step 3: Implement persisted-value-aware initialization**

Pass each existing resolved attribute value into `buildInitialManualSelection`. Prefer a value ID only when it is still present in the option's template values; otherwise initialize `textValue` from the persisted resolved value, falling back to the source value only when no saved value exists.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `web/listingkit-ui/node_modules/.bin/vitest.cmd run src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`

Expected: PASS with custom `11` visible and resubmitted as custom text.

- [ ] **Step 5: Commit the frontend slice**

Run: `git add web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.tsx web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx && git commit -m "fix: preserve manual SHEIN sale attribute text"`

### Task 3: Verify the integrated online-save contract

**Files:**
- Verify: `internal/listingkit/service_revision_test.go`
- Verify: `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`

- [ ] **Step 1: Format and run focused checks**

Run: `gofmt -w internal/listingkit/service_revision_manual_sale_attributes.go internal/listingkit/service_revision_manual_sale_attributes_assignment_support.go; go test ./internal/listingkit -count=1 -vet=off -timeout=240s; web/listingkit-ui/node_modules/.bin/vitest.cmd run src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx; npm run typecheck`

Expected: all commands exit zero.

- [ ] **Step 2: Inspect final scope**

Run: `git status --short; git diff --check HEAD~2..HEAD`

Expected: only the two intentional implementation commits and no whitespace errors.
