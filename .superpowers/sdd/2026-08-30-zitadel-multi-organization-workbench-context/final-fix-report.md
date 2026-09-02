# ZITADEL Multi-Organization Workbench — Final Fix Report

Date: 2026-08-31 (Asia/Singapore)

## Status and scope

This final wave closes all eight review findings in the branch at the code, test, and implementation-plan levels. It does **not** claim production readiness: no real or local ZITADEL instance was called or mutated, and the enabled-runtime, browser, revocation, and real-provider acceptance gates remain external follow-up work.

Source commits created by this wave:

- `f3a8ce853` — `fix: harden workbench identity boundaries`
- `cfbcff0a3` — `fix: enforce strict workbench browser contracts`
- `667c6bdfb` — `docs: make store center bff plan executable`

The implementation deliberately reuses maintained open-source and existing project infrastructure:

- bounded in-process caching uses `github.com/jellydator/ttlcache/v3` v3.4.1 rather than adding a custom cache/eviction engine;
- identity auditing adapts the existing Logrus structured logger rather than adding another logger or an unplanned audit database;
- browser contracts continue to use the project's Zod and existing bounded request/response readers.

## Finding-by-finding closure

### 1. Grant cache was unbounded and retained expired one-off subjects

Root cause: the cache was a process-lifetime `map` with lazy, key-local expiry. A subject that never returned could leave an expired entry indefinitely, and there was no capacity bound.

RED evidence: the new lifecycle/capacity tests did not compile against the map implementation because it had no bounded-cache constructor or cache `Len`/`Items` operations.

Fix:

- replaced the map with a capacity-bounded `ttlcache` instance (default capacity 1024);
- retained the security TTL as `min(60s, token expiry)` and disabled expiry extension on reads;
- performs global expired-entry cleanup on writes;
- preserves atomic subject/project invalidation across contract versions with the existing outer lock;
- retains detached copies at the cache boundary;
- strengthened the eviction test to touch the first entry before inserting the third, proving actual LRU ordering rather than FIFO behavior.

GREEN evidence: `go test ./internal/workbenchcontext/... -run "TestGrantCache" -race -count=1` passed.

### 2. ZITADEL pagination decoded ProtoJSON `uint64` inconsistently

Root cause: authorization runtime had a private compatible decoder, while three provisioning pagination responses decoded `totalResult` into Go `int`. Canonical ProtoJSON quoted integers therefore failed only in provisioning paths.

RED evidence: the organization, project-grant, and authorization pagination tests each failed with the JSON type error equivalent to `cannot unmarshal string into Go struct field ... totalResult of type int`.

Fix:

- introduced one shared `internal/zitadelprotojson.Uint64` decoder;
- applied it to runtime authorization and all three provisioning pagination responses;
- retained numeric JSON compatibility while accepting canonical quoted decimal values;
- added direct boundary tests for max `uint64`, numeric compatibility, overflow, negative, fractional, and malformed quoted inputs.

GREEN evidence: `go test ./internal/zitadelprotojson ./internal/authruntime/zitadel ./internal/zitadelprovision -race -count=1` passed.

### 3. Conflicting Organization names could be hidden by downstream normalization

Root cause: grants from different raw pages were merged by Organization ID without rejecting two different nonblank raw names. Later normalization could hide the conflict.

RED evidence: the new cross-page conflicting-name test returned a nil error.

Fix: detect conflicting trimmed nonblank names while accumulating the raw authorization response, before downstream grant normalization, and return the stable non-sensitive error `ZITADEL authorization contains conflicting organization names` without echoing either provider value.

GREEN evidence: the authorization runtime race suite passed, including quoted/numeric pagination and the cross-page conflict regression.

### 4. Workbench BFF trusted partial and hostile upstream envelopes

Root cause: the BFF checked only a small subset of successful fields and relayed selected upstream headers/body bytes. It did not enforce one exact success/error schema or exact status/body pairing.

RED evidence: eight adversarial cases initially failed, covering partial/unknown/duplicate success payloads, malformed/unknown error payloads, success/error status mismatches, unexpected `201`, invalid UTF-8, and hostile response headers.

Fix:

- exports and reuses one strict Workbench context schema between browser API client and server proxy;
- validates bounded IDs, display names, roles, unique Organization IDs, effective membership, and selection-state consistency;
- validates one strict error envelope and bounded field errors;
- accepts context only at `200` and error envelopes only at `400..599`;
- decodes UTF-8 fatally, rejects invalid JSON/unknown fields/status mismatches, and reserializes only validated data;
- forces `Content-Type: application/json`, `Cache-Control: private, no-store`, and `X-Content-Type-Options: nosniff`;
- does not forward upstream cache, ETag, request ID, redirect, cookie, connection, or presentation headers;
- preserves the existing HttpOnly Organization-selection cookie behavior only after successful validation.

Regression diagnosis: the first broader route run exposed two stale tests, not implementation regressions. A manually returned redirect now correctly maps to stable `502` rather than the old `302` expectation, and a switch fixture was invalid because its effective Organization was absent from its Organization list. Both fixtures were corrected to the strict contract.

GREEN evidence: focused BFF/API/provider/shell coverage passed 7 files and 82 tests; the full frontend suite passed 286 files and 1878 tests.

### 5. Store Center plan omitted an executable browser trust boundary

Root cause: the vertical-slice plan described Go routes and UI work but did not assign ownership for strict browser-to-Go route mapping, request/header filtering, exact response DTOs, or status contracts. It also left create/delete idempotency and mutation version requirements ambiguous across layers.

Fix (plan only; no Store endpoint was implemented):

- made create and delete require `Idempotency-Key`;
- made update/enable/disable/delete require quoted `If-Match` and explicitly excluded idempotency keys from enable/disable;
- defined exact Store/list/delete response DTOs and statuses without Organization ID or connection reference;
- added Task 7A for the exact BFF path/method allowlist, canonical UUIDs, bounded allowlisted queries, strict request bodies, header reconstruction, identity stripping, manual redirects, bounded response handling, strict Zod/status validation, safe response headers, and adversarial/regression tests;
- corrected the plan's ESLint flat-config verification command to use positional file paths instead of the removed `--file` option.

Architecture note: Store Center must not be implemented by directly widening the existing context proxy. The new plan makes its browser trust boundary an explicit task and contract owner.

### 6. High-risk Organization decisions had no narrow injected audit port

Root cause: Organization switching and post-authentication denials passed through middleware without a dedicated audit contract. Adding ad-hoc logs at each branch would have risked inconsistent fields and accidental secret logging.

RED evidence:

- audit tests initially failed to compile because no audit contract/dependency existed;
- the composition test then observed a nil audit recorder;
- a mutation check that temporarily removed the resolver-error audit branch made the switch-denial regression fail with no recorded event.

Fix:

- added a narrow `AuditEvent`/`AuditRecorder` port whose type can carry only subject, home/effective Organization IDs, resource, action, result, timestamp, and request ID;
- added a production adapter over the existing structured Logrus logger;
- injects the recorder and clock through Workbench composition rather than globals;
- records successful live switches before the handler runs; audit failure fails closed as stable `503` and prevents success;
- records invalid/denied/revoked/suspended/selection/dependency outcomes and role denials best-effort without changing an existing denial into an allow or a different status;
- never places bearer tokens, cookies, payloads, credentials, or raw provider responses into the audit event type.

Regression diagnosis: one existing successful switch fixture had no recorder after success became audit-required, so it correctly returned `503`; the fixture was updated with an injected stub.

GREEN evidence: affected HTTP/composition tests passed under `-race`; structured-log regression checks confirm the narrow fields and forbidden-field absence.

### 7. Loopback provisioning client had no overall request deadline

Root cause: the hardened transport constrained destination/redirect behavior but `http.Client.Timeout` remained zero, allowing an indefinitely stalled response outside caller-specific deadlines.

RED evidence: the new test observed `client.Timeout == 0`.

Fix: set a finite five-second overall client timeout while retaining caller-context cancellation, loopback-only resolution/dialing, and redirect rejection.

GREEN evidence: the test confirms the exact finite bound and cancellation completes well under one second; provisioning race tests passed.

### 8. Delegated-operation context was not persistently visible

Root cause: the provider did not expose the home Organization ID, so the shared application shell could not distinguish home-context work from delegated work.

RED evidence: provider and shell tests could not observe a home Organization value or delegated-operation indicator.

Fix:

- exposed validated `homeOrganizationId` through `WorkbenchContextProvider`;
- renders a persistent accessible status in the shared header whenever effective and home Organizations differ;
- displays the validated effective Organization name and home Organization name, with a non-ID fallback when the home Organization is not in the accessible list;
- keeps the same shell/header behavior across mobile and desktop and does not prematurely add Store navigation.

GREEN evidence: provider, shell, switcher, and application-frame regressions are included in the 82-test focused frontend run.

## Verification ledger

No command is still running.

Passed:

```text
go test ./internal/zitadelprovision ./internal/authruntime/zitadel ./internal/workbenchcontext/... ./internal/app/httpapi -race -count=1
go test ./internal/zitadelprovision/cmd ./internal/authidentity ./internal/httproute -race -count=1
go test ./internal/workbenchcontext/... -run "TestGrantCache" -race -count=1
go test ./internal/zitadelprotojson ./internal/authruntime/zitadel ./internal/zitadelprovision -race -count=1

npm.cmd test -- src/lib/server/workbench-proxy.test.ts "src/app/api/workbench/[...path]/route.test.ts" src/lib/api/workbench-context.test.ts src/components/providers/workbench-context-provider.test.tsx src/components/workbench/workspace-app-shell.test.tsx src/components/workbench/organization-switcher.test.tsx src/components/application-frame.test.tsx
# 7 files, 82 tests passed

npm.cmd run typecheck
npm.cmd run lint -- src/lib/server/workbench-proxy.ts "src/app/api/workbench/[...path]/route.ts" src/lib/api/workbench-context.ts src/components/providers/workbench-context-provider.tsx src/components/workbench/workspace-app-shell.tsx
npm.cmd test
# 286 files, 1878 tests passed in 161.2s

git diff --check
```

Non-passing attempts and exact disposition:

- The first full frontend run was terminated by the external 124-second command limit, with no test failure reported before termination. The identical suite was rerun with a 300-second limit and passed all 286 files/1878 tests. Twelve jsdom `Could not parse CSS stylesheet` warnings remained non-failing test-environment noise.
- One scoped lint invocation used `--file` and failed immediately with `Invalid option '--file'` because this repository uses ESLint flat config. The plan and invocation were corrected to positional paths; lint then passed.
- All intentional RED failures and the two stale strict-contract fixtures are described in the corresponding finding sections above; none remain unresolved.

## Secret and boundary review

An additions-only diff scan for tokens, bearer values, cookies, credentials, passwords, raw payloads, and authorization responses found only deliberate test fixtures and plan prose. Production audit events cannot carry those values. The BFF adversarial `accessToken` fixture verifies that credential-shaped unknown response data is rejected rather than relayed.

No real/local ZITADEL provisioning, authorization, mutation, browser login, or account change was performed during this wave.

## Remaining external gates and concerns

- Run the enabled Workbench composition against an approved local/non-production ZITADEL environment with quoted pagination responses.
- Execute real browser Organization switch/delegated-indicator acceptance with a safe test identity.
- Prove grant revocation and suspension convergence against real provider timing and operational log collection.
- Confirm the deployment log pipeline gives the structured audit stream the required retention, access control, and alerting. The application now emits a narrow fail-closed switch audit, but durable operational handling is outside this repository change.
- Execute the Store Center vertical-slice plan separately. This wave made the BFF/DTO contract executable; it did not create Store routes or UI.

Until those gates pass, this branch is code-complete for the reviewed findings but not production-ready.
