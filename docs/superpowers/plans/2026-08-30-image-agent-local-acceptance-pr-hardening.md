# Image Agent Local Acceptance PR Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Image Agent Manual Runtime local acceptance chain isolated, fail-closed, repeatable, and safe enough for a Draft PR.

**Architecture:** Keep product runtime schema and browser sessions free of acceptance-only concerns. Put developer entrypoints under their owning business modules, make acceptance launchers explicitly opt into an isolated mode, and prove identity/database/process ownership at every boundary before allowing seed or runtime operations.

**Tech Stack:** Go, PowerShell/Pester, Next.js/Auth.js, Docker Compose, PostgreSQL, ZITADEL, Temporal.

**Spec:** `docs/development/image-agent-local-acceptance.md`

## Global Constraints

- Preserve default developer launch behavior outside explicit acceptance mode.
- Never print or return access tokens, client secrets, PATs, or password values.
- Bind acceptance services to loopback only and refuse unknown port owners.
- Do not add acceptance-only tables to canonical product migrations.
- Do not weaken repository architecture tests or production identity checks.
- Keep `web/listingkit-ui/AGENTS.md` and `web/listingkit-ui/CLAUDE.md` out of the change.

---

## Task 1: Move development CLIs behind the repository command boundary

- [x] Move the ZITADEL provisioner entrypoint from production `cmd/` into its owning internal module.
- [x] Move the acceptance seed entrypoint from production `cmd/` into its owning internal module.
- [x] Update PowerShell scripts, tests, and documentation to invoke the new paths.
- [x] Run focused tool tests and repository architecture-boundary tests.

## Task 2: Add explicit isolated launcher behavior

- [x] Add an acceptance-isolated switch to the API and UI launchers.
- [x] In isolated mode, skip repository `.env`, disable kubeconfig discovery, use acceptance-specific state/PID directories, and refuse occupied ports.
- [x] Add behavioral tests with a hostile `.env`, mocked Kubernetes discovery, and unrelated port owners.
- [x] Keep existing non-acceptance launcher behavior compatible.

## Task 3: Harden Compose networking and lifecycle ownership

- [x] Bind PostgreSQL, Redis, Temporal, MinIO, ZITADEL, API, and UI exposure to `127.0.0.1` only.
- [x] Verify rendered Compose configuration contains no wildcard published address.
- [x] Track and validate process ownership before stop operations.
- [x] Stop all acceptance services, including MinIO and ZITADEL, in both normal stop and reset paths.

## Task 4: Prove database and issuer isolation

- [x] Validate the exact Compose project, service, published host port, loopback host, and database name against the effective DSN.
- [x] Require a loopback ZITADEL issuer for acceptance operations.
- [x] Add negative tests for mismatched service, port, database, and non-loopback issuer.

## Task 5: Keep the acceptance marker acceptance-only

- [x] Remove the acceptance marker from canonical product runtime migrations.
- [x] Initialize and validate the marker explicitly in the acceptance workflow.
- [x] Update schema and acceptance tests to prove production migrations do not create the marker.

## Task 6: Remove tokens from browser-visible Auth.js sessions

- [x] Persist the local acceptance access token only from the encrypted server-side JWT callback path.
- [x] Remove access and ID tokens from session objects and custom session responses.
- [x] Restore the default ZITADEL audience and role scopes while retaining explicit local overrides.
- [x] Add tests proving browser-visible sessions are token-free and server-side persistence remains functional.

## Task 7: Harden ZITADEL provisioning identity

- [x] Validate reused API applications use the exact Basic authentication method.
- [x] Reject placeholder or missing client secrets and provide a safe local recreate/rotate path.
- [x] Persist the bootstrap user ID and require authorization introspection subject equality.
- [x] Add focused provisioner and script tests for mismatched auth methods, secrets, and subjects.

## Task 8: Make seed idempotency concurrency-safe

- [x] Recover uniqueness conflicts by rereading the canonical seed rows and checking equivalence.
- [x] Add a concurrent database-backed seed test.
- [x] Preserve deterministic tenant, user, task, and workspace identities.

## Task 9: Verify and prepare the Draft PR

- [x] Run focused Go, Pester, and UI tests after each slice.
- [x] Run serial full Go tests plus UI test, lint, and typecheck suites.
- [x] Request an independent final code review and address all critical/important findings.
- [x] Stage only intended paths, commit, merge current `origin/main`, resolve conflicts, and rerun verification.
- [ ] Push the `codex/image-agent-local-acceptance` branch and create a Draft PR without merging or deploying.
