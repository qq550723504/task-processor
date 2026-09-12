# ACC-1 account profile and shared presentation

Authority: #409 / #408, `account-readonly-contract.md`, current Figma `31:463`
and `final-ui-ia-authority.md`. The live `432:534` profile design was read with
the Figma design-to-code skill before implementation. Existing account panels,
Console tokens and primitives remain the implementation basis. Figma example
values are never user facts.

## Current owner and permission boundary

The existing chain is browser `getAccountProfile` -> dedicated Next account BFF
-> current `workbenchcontext/httpapi.GetAccountProfile` ->
`authidentity.SelfProfileReader` -> `authruntime/zitadel.UserInfoClient.ReadSelf`.
CurrentIdentity verifies the caller independently of Effective Organization.
The caller and returned subject must match. Home is identity metadata, never
the selected Organization. Account-v1 and its no-store, cancellation, bounded
response and error contracts remain unchanged.

Read-only fields are userId, homeOrganizationId, displayName, email,
emailVerified, phoneNumber and phoneNumberVerified. Source/readAt describe the
projection. No current profile mutation API or write owner is admitted, so the
mutation/persistence/reread acceptance is **NOT_APPLICABLE**, not PASS or a
test-runner SKIP. No new IAM scope or profile write API is introduced.
Missing claims remain “未提供”; absent optional personal fields have an explicit
empty state without discarding the verified account ID. No region, registration,
password, certification or business profile values are inferred.

## Shared Account contract

`AccountShell({ pathname, title, description?, actions?, children })` in
`src/components/workbench/account/account-shell.tsx` owns the common page and
breadcrumb rendering. It consumes the existing `findConsoleRoute` hierarchy.
Each leaf still owns authentication, permission checks, loading and its data
reader. This component grants no authorization and holds no business state.

Account root redirects to profile. Breadcrumb parents provide return paths.
Navigation reveals the selected branch when the route changes and permits
manual collapse on the current route. The existing desktop/mobile navigation
components share this behavior. Resource source accounts remain below resources
in breadcrumbs without adding a sidebar level.

#410/#411/#412 retain their leaf ownership. Pending nodes remain unavailable
until an actual leaf candidate is coordinated; a link is not an assertion that
a provider or business capability is enabled. Global application-frame,
workspace-app-shell, BFF catch-all, authentication and runtime composition are
outside this change's write scope.

## Verification

Behavioral RED cases: missing account breadcrumb return, missing optional-claim
empty state, absent expired-session login recovery, and collapsed selected
navigation after route change. The shared source-account breadcrumb adds a
separate failing assertion. Existing cancellation, late success/failure,
identity changes, enterprise isolation and manual retry tests are retained.

Use the existing task-owned `scripts/account-fixture.mjs` and
`scripts/account-browser-verification.mjs` described in `issue348-account-ui.md`.
They exercise real client/Auth.js/BFF/Go reads with external provider substitutes;
they are not real IAM or production evidence. No database is needed for this
profile reader. Ports, processes, synthetic sessions and browser resources must
remain task-owned and be cleaned after verification. Exact HEAD, current CI,
independent complete-slice review and remaining NOT_RUN belong in the PR.
