# ListingKit Mandatory Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every ListingKit authentication-disable switch while keeping public health probes available.

**Architecture:** Protected ListingKit routes always construct and run ZITADEL identity middleware. Authorization allowlists remain optional after identity verification. Configuration and local tooling no longer expose an authentication-disabled mode; standalone crawler probes bypass only the HTTP adapter.

**Tech Stack:** Go, Viper configuration, Gin/net/http, PowerShell, Kubernetes YAML, Go tests.

## Global Constraints

- Do not restore trusted `X-User-*` identity headers or anonymous ListingKit APIs.
- `/health`, `/readyz`, crawler `/health`, and crawler `/ready` remain public.
- Missing ZITADEL issuer/client fails closed for protected routes.
- `authorizationRequired` remains an optional post-identity allowlist, not an authentication switch.

---

### Task 1: Remove the authentication configuration switch

**Files:**
- Modify: `internal/core/config/type_listingkit.go`, `config/config-dev.yaml`, `config/config-test.yaml`, `config/config-prod.yaml`
- Modify: `internal/core/config/config.go`, `internal/core/config/defaults.go`, `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/config_test.go`, `internal/core/config/config_env_test.go`

- [x] Write failing tests proving `authRequired` YAML and `TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED` do not configure ListingKit authentication.
- [x] Run `go test ./internal/core/config -run 'Test.*ListingKit.*Auth' -count=1` and confirm RED.
- [x] Remove `AuthRequired`, its YAML/default/env bindings, and compatibility coercion; retain `AuthorizationRequired`.
- [x] Run `go test ./internal/core/config -count=1` and `go vet ./internal/core/config`.
- [x] Commit `refactor: remove ListingKit auth disable configuration`.

### Task 2: Remove local Disabled mode and preserve fail-closed routing

**Files:**
- Modify: `internal/listingkit/httpapi/zitadel_auth_runtime.go`, `internal/listingkit/httpapi/zitadel_auth_test.go`
- Modify: `scripts/start-listingkit-local-api.ps1`, `scripts/start-listingkit-local-dev.ps1`, `scripts/start-listingkit-local-ui.ps1`, and their tests
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`, `web/listingkit-ui/src/proxy.ts`, and ListingKit proxy/session tests
- Modify: `internal/app/httpapi/server_test.go`

- [x] Add failing route tests: false/absent legacy flags cannot yield an unauthenticated protected route; empty issuer/client returns 503.
- [x] Add failing script test or static assertion that `ZitadelAuthMode Disabled` is unavailable.
- [x] Simplify middleware initialization so identity authentication is unconditional and remove Disabled-script branches plus the UI local-debug identity/header bypass.
- [x] Run affected Go tests, PowerShell script tests, and targeted UI tests.
- [x] Commit `refactor: remove ListingKit local auth bypass`.

### Task 3: Verify public probes and deployment documentation

**Files:**
- Modify: `internal/listingkit/httpapi/zitadel_auth_http_adapter_test.go`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`, local-debug documentation, and every auth environment example

- [x] Add failing tests proving `/health`, `/readyz`, crawler `/health`, and crawler `/ready` bypass middleware while protected routes do not.
- [x] Remove obsolete auth-disable documentation and describe fail-closed local/deployment setup.
- [x] Run `go test ./internal/listingkit/httpapi ./internal/crawler/alibaba1688 ./internal/app/httpapi -count=1`, `kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod`, and `git diff --check`.
- [x] Commit `docs: require ListingKit authentication in every environment`.

### Task 4: Final verification and review

- [x] Run `go test ./... -count=1`.
- [x] Run ListingKit UI typecheck/lint/build because auth configuration changes affect UI integration.
- [x] Inspect `rg -n 'AuthRequired|TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED|ZitadelAuthMode|LISTINGKIT_UI_BYPASS_AUTH_GATE|LOCAL_DEBUG' internal config deployments scripts web` and require zero ListingKit disable controls.
- [x] Request independent code review; fix all Critical and Important findings before push.
