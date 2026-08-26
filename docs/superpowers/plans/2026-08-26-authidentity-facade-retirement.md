# Retire ListingKit AuthenticatedIdentity Compatibility Facade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (\`- [ ]\`) syntax.

**Goal:** Remove the now-unused \`internal/listingkit.AuthenticatedIdentity\` compatibility facade after all production callers have moved to \`internal/authidentity\`.

**Architecture:** \`internal/authidentity\` remains the sole owner of authenticated identity context values. ListingKit tests and compatibility fixtures will use that neutral package directly; no production behavior or route authorization changes are included.

**Tech Stack:** Go, AST-based import-boundary tests.

**Spec:** \`docs/architecture/auth-and-tenancy.md\` and the merged Zitadel auth runtime extraction.

## Global Constraints

- Do not change authentication behavior, tenant parsing, role authorization, Legacy Tenant Bridge behavior, or HTTP status/error contracts.
- Do not modify SHEIN Login, Local Agent, 1688 business logic, database Builder, deployment manifests, or runtime configuration.
- Do not retain a second identity-context implementation or root \`listingkit\` identity aliases after this slice.
- Preserve \`internal/authidentity\` tests and context validation semantics.

---

### Task 1: Prove the compatibility facade can be retired

**Files:**
- Modify: \`tests/import_boundaries_test.go\`
- Test: \`internal/listingkit/authenticated_identity_test.go\` or a focused boundary test

**Interfaces:**
- Consumes the existing AST/index helpers in \`tests/import_boundaries_test.go\`.
- Produces a failing guard when any Go source still calls the retired root ListingKit identity API.

- [ ] **Step 1: Add a failing root-facade guard**

Add \`TestListingKitAuthenticatedIdentityCompatibilityFacadeIsRetired\` that fails if \`internal/listingkit/authenticated_identity.go\` exists or if non-test Go files import/use the root identity API. The failure must include the source path.

- [ ] **Step 2: Run the focused guard**

Run:

\`\`\`powershell
go test ./tests -run 'TestListingKitAuthenticatedIdentityCompatibilityFacadeIsRetired' -count=1
\`\`\`

Expected: FAIL because the compatibility facade still exists.

### Task 2: Migrate remaining test-only callers and remove the facade

**Files:**
- Modify: remaining Go test files reported by the guard under \`internal/compatibility/listingkit/sourcehandoff/a1688/httpapi\`, \`internal/listingkit/api\`, \`internal/listingkit/httpapi\`, \`internal/localagent/httpapi\`, \`internal/productimage/httpapi\`, and \`tests\`.
- Delete: \`internal/listingkit/authenticated_identity.go\`
- Delete: \`internal/listingkit/authenticated_identity_test.go\` if its duplicate coverage is already provided by \`internal/authidentity\`.

**Interfaces:**
- Consumes \`authidentity.AuthenticatedIdentity\`, \`authidentity.WithAuthenticatedIdentity\`, and \`authidentity.AuthenticatedIdentityFromContext\`.
- Produces no root ListingKit identity API references.

- [ ] **Step 1: Replace only test/fixture imports and selectors**

Change test helpers and fixtures to import \`task-processor/internal/authidentity\` and use its identity type/context functions. Do not alter production request handling or compatibility endpoint behavior.

- [ ] **Step 2: Run focused package tests**

Run:

\`\`\`powershell
go test ./internal/authidentity ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/compatibility/listingkit/sourcehandoff/a1688/httpapi ./internal/localagent/httpapi ./internal/productimage/httpapi ./tests -run 'Test(AuthenticatedIdentity|RequestContext|CreateListingKitTask|ZitadelAuth|LocalAgent|ProductImage)' -count=1
\`\`\`

Expected: PASS with no root ListingKit identity facade required.

### Task 3: Final boundary and repository verification

**Files:**
- Modify: \`docs/architecture/architecture-review-checklist.md\` only if the new guard is not already listed.
- Test: \`tests/import_boundaries_test.go\`

- [ ] **Step 1: Run architecture and compatibility guards**

\`\`\`powershell
go test ./tests -run 'TestListingKitAuthenticatedIdentityCompatibilityFacadeIsRetired|TestAuthenticatedIdentityRootImportsStayRestricted|TestZitadelAuthRuntimeDoesNotImportListingKit' -count=1
git diff --check
\`\`\`

- [ ] **Step 2: Run the repository suite**

\`\`\`powershell
go test ./...
\`\`\`

Expected: exit code 0 and no changes outside the scoped tests/facade removal.

- [ ] **Step 3: Review final scope and commit**

\`\`\`powershell
git status --short --branch
git diff --stat origin/main...HEAD
git add -- <scoped files>
git commit -m "refactor: retire ListingKit identity compatibility facade"
\`\`\`

No push, PR, merge, or deployment is part of this plan unless separately authorized.
