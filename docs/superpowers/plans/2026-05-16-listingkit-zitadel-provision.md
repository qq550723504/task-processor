# ListingKit ZITADEL Provision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an idempotent operator tool that provisions ListingKit ZITADEL project roles and emits the runtime env values needed by ListingKit.

**Architecture:** Keep ZITADEL mutations out of application startup. Add a small Go package with an HTTP client for ZITADEL Management REST v1 project and role endpoints, a CLI wrapper under `cmd/`, and deployment docs that explain when to run it and how to configure scopes/allowlists.

**Tech Stack:** Go standard library, `httptest`, ZITADEL Management REST v1, existing repo scripts/docs conventions.

---

### Task 1: Provisioning Package

**Files:**
- Create: `internal/zitadelprovision/provisioner.go`
- Create: `internal/zitadelprovision/provisioner_test.go`

- [ ] **Step 1: Write failing package tests**

Add tests that use `httptest.Server` and verify:
- An existing project named `ListingKit` is found via `POST /projects/_search`.
- Existing roles from `POST /projects/{id}/roles/_search` are skipped.
- Missing roles are created with `POST /projects/{id}/roles`.
- A missing project fails unless `CreateProject` is true.
- A missing project is created with role assertions enabled when `CreateProject` is true.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/zitadelprovision`

Expected: FAIL because the package does not exist yet.

- [ ] **Step 3: Implement provisioning package**

Implement:
- `DefaultRoles() []ProjectRole`
- `RecommendedScopes() []string`
- `Provision(ctx context.Context, cfg Config) (Result, error)`
- HTTP helpers that attach `Authorization: Bearer <token>` and optional `x-zitadel-orgid`.

Use ZITADEL endpoints:
- `POST /projects/_search`
- `POST /projects`
- `POST /projects/{projectID}/roles/_search`
- `POST /projects/{projectID}/roles`

- [ ] **Step 4: Run package tests**

Run: `go test ./internal/zitadelprovision`

Expected: PASS.

### Task 2: CLI Wrapper

**Files:**
- Create: `cmd/listingkit-zitadel-provision/main.go`

- [ ] **Step 1: Write CLI around the package**

Add flags:
- `-issuer-url`
- `-token`
- `-org-id`
- `-project-id`
- `-project-name`
- `-create-project`

Read defaults from env:
- `ZITADEL_ISSUER_URL`
- `ZITADEL_MANAGEMENT_TOKEN`
- `ZITADEL_ORG_ID`
- `LISTINGKIT_ZITADEL_PROJECT_ID`
- `LISTINGKIT_ZITADEL_PROJECT_NAME`

Print project id, role statuses, recommended `ZITADEL_SCOPES`, and recommended `LISTINGKIT_ZITADEL_ALLOWED_ROLES`.

- [ ] **Step 2: Run CLI package tests/build**

Run: `go test ./cmd/listingkit-zitadel-provision ./internal/zitadelprovision`

Expected: PASS or no test files for the command package plus passing internal package tests.

### Task 3: Docs And Env Examples

**Files:**
- Modify: `.env.example`
- Modify: `deployments/kubernetes/zitadel/local/README.md`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`

- [ ] **Step 1: Document provisioning flow**

Document:
- Do not run provisioning from normal app startup.
- Required management token permissions.
- Example local and production commands.
- The roles created by the tool.
- The recommended role allowlist.

- [ ] **Step 2: Update env examples**

Add commented examples for:
- `ZITADEL_MANAGEMENT_TOKEN`
- `ZITADEL_ORG_ID`
- `LISTINGKIT_ZITADEL_PROJECT_ID`
- `LISTINGKIT_ZITADEL_PROJECT_NAME`
- `LISTINGKIT_ZITADEL_ALLOWED_ROLES=listingkit_admin,listingkit_operator,listingkit_viewer`

- [ ] **Step 3: Run final verification**

Run:
- `go test ./internal/zitadelprovision ./cmd/listingkit-zitadel-provision`
- `npm test -- src/components/listingkit/shared/listingkit-app-shell.test.tsx src/components/providers/zitadel-auth-gate.test.tsx`
- `npm run typecheck`

Expected: all pass.
