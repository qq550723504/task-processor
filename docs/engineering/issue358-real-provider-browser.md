# Issue #358 real-provider browser acceptance

This L1-B slice consumes the approved 2026-09-05 native-login decision and the
current Console, account and commercial contracts. It owns browser regression
and only Next defects demonstrated by actual counterexamples. #357 alone owns
the official provider, provisioning, Go composition, database and runtime CLI.
The execution Issue and PR hold rolling SHA, review and execution status.

## Admission and product boundary

First independent narrow review: IMPLEMENTATION_READY, no BLOCKER. The reviewer
classified immediate logout selection-cookie removal as IMPLEMENTATION_TEST:
assert it before any subsequent context response can clear a leftover selection.
No production defect is claimed from static inspection alone.

Initial slice: tests and this runbook; no production change proposed. Reuse
`/login`, Auth.js, current BFFs, Shell and pages. Do not add authentication,
authorization, recovery, UI or environment architecture. A real Next defect is
first captured as RED, classified against the Issue Must, then minimally fixed
with sibling-path regressions. Go/provider/runtime counterexamples go to #357.
Domain or state-machine disputes go to the designated read-only architect.

Must: actual email/password authentication at official Login V2; server-side code
exchange; naturally issued Auth.js session; self identity separate from Effective
Organization; existing organization and commercial permissions; no stale data
after switch/logout/account change; bounded real revocation/expiry; zero business
writes. Existing account/context CachedRead <=60s (bounded by token expiry) is an
accepted policy; commercial reads and switching use live authorization.
HTTP loopback is LOCAL_REAL_PROVIDER_ACCEPTANCE only. Phone, Passkey, IDP, MFA,
production TLS, registration, payment and member management are outside this run.

No Auth.js/JWT signing, cookie or storageState injection, bearer/PAT user-login
substitution, response mocks, synthetic final DTOs or fabricated clock/expiry.
Browser cookies produced by this run's real login may be used naturally in that
same browser context. Bootstrap/control credentials never enter the browser.
Do not enable the existing acceptance-token file export or mock environment flags.

## Actual call paths and bounded matrix

| Case / entry | Actual dependency and scope / effects | Required browser evidence |
| --- | --- | --- |
| M1 fresh protected profile and organization routes | `proxy.ts` serverAuth -> bare `/login?returnTo` -> `/api/zitadel-auth/login` -> Auth.js ZITADEL -> official Login V2 -> `/api/auth/callback/zitadel` code exchange | New empty context, password UI completed, provider and callback paths observed without query values, public session has current subject and no access/ID/refresh token |
| M2 self profile before selection; admin and no-org | AccountServerPage -> sole account TS client -> `/api/account/profile` -> CurrentIdentity -> introspection + UserInfo; no selection or business write | Profile subject/Home match actual provider IDs; no-org can read self; organization/plans/sibling routes retain gates |
| M3 admin B/C/Empty and forbidden D | UI switch -> workbench BFF -> live resolver + selection cookie; account organization CachedRead; commercial BFF -> live resolver/Casbin AdminRead -> actual PG reader | Home A remains A; B/C organization and persisted usage differ; Empty subscription null/unknown usage; D switch/read rejected; no default plan or grants created |
| M4 viewer | Normal second-user password login; actual viewer grants from provider; existing AdminRead | Self and permitted organization readable; commercial request is denied, no success data; login never grants admin permission |
| M5 switch and late reads | Existing request abort/React Query scope lifetime, actual account/commercial requests | Pending read during B->C and logout cannot restore old content. Delay only transport of a real request if needed; never replace a response or call authorization through a test substitute |
| M6 logout and another account | Actual logout link -> serverAuth + signOut -> discovered official end-session; same browser subsequently enters bare login | Application session invalid, account/commercial APIs 401, selection absent, old content removed; official logout path observed; subsequent normal login has new subject and no old scope. Do not claim global token revocation |
| M7 returnTo/method | Existing normalization at page, login API and Auth.js redirect callback | Valid workbench target retained; external/protocol-relative/backslash/malformed/unsupported paths safe; otp/password 503; missing/unknown/repeated method generic; no dedicated-method UI |
| M8 provider failure and configuration | #357 owned provider/runtime controls, current error paths | No forged success/demo or unbounded redirect loop; missing configuration fails closed. Separate executable existing-entry checks from provider acceptance |
| M9 revocation and restoration | #357 CLI changes only registered test-user grants; real provider API | Commercial live denial after provider confirms revoke; account/context convergence by documented <=60s bound plus bounded request time; record real timestamps and provider consistency, restore in finally; no fake clock |
| M10 token lifecycle | #357 short real provider lifetime + offline_access/refresh application contract, current Auth.js refresh/exchange | Observe real expiry/renewal with wall time; current identity preserved or expired session safely rejected according to existing contract; no token values exported or JWT manipulation |
| M11 business zero writes and cleanup | #357 all-saas row values/xmin snapshots and normal stop | Before/after equality; IAM session/audit and test grant changes separately classified; exact owned PIDs/ports/containers/network/volumes cleaned; repeat stop per owner contract |

## Runtime dependency

Consume only an explicit #357 implemented, pushed full SHA and its matching
runbook. PLANNED commands are not runnable evidence. At initial admission #357
proposes `node scripts/issue357-runtime.mjs start [--web-dir <absolute-ui>]`,
`check --run <uuid>`, `restart --run <uuid>`, `stop --run <uuid>` and a private
temporary `task-processor-issue357/<uuid>/manifest.json`. The fixed PLANNED v1
schema uses sourceSha/webSha, origins.web/go/issuer, organizations.A/B/C/Empty/D
and users.admin/viewer/no-org with private credentialFile paths. Planned
`revoke|restore --run <uuid> --user admin|viewer --org B|C|Empty` and 120-second
actual token lifetimes still require implemented handoff. Do not guess values or
create a second seed/fixture. Each owner gets a separate run or explicit exclusive
instance handoff. Do not stop or mutate the producer's ongoing run.

The implemented handoff must provide source/web SHA, supported OS, generated
variable names, credential path (no values), actual Home A/B/C/Empty/D IDs,
admin/viewer/no-org user IDs, OIDC audience/scopes, loopback origins/ports, short
token lifetime, ownership/control/zero-write/cleanup contract. The browser runner
will validate that contract before navigation, with missing prerequisites failing
explicitly rather than skipping. Final main integration follows #357 merge.

The designated architect's narrow ruling allows only this run's newly created
business Project to opt out of projectRoleCheck/hasProjectCheck, so no-org can
authenticate for self reads. Go grants/Casbin stay authoritative for enterprises
and commerce. The same existing OIDC client uses strictly paired same-origin,
same-port callback/logout LocalApplicationConfig. #357 implements these rulings;
this slice verifies them and returns any counterexample to that owner.

## Prepared regression entry

From `web/listingkit-ui`, after installing the pinned lockfile:

```powershell
pnpm.cmd exec vitest run scripts/real-provider-browser-contract.test.mjs
node scripts/real-provider-browser.mjs <private-manifest.json> <exact-357-SHA> <evidence-directory> <357-checkout>
# Only for an exclusively owned run that this operator started:
node scripts/real-provider-browser.mjs <private-manifest.json> <exact-357-SHA> <evidence-directory> <357-checkout> --stop-owned-run
```

The first command only verifies handoff rejection boundaries, not authentication.
The second is prepared for the implemented runtime and rejects absent input,
non-ready/old session fixtures, wrong SHAs, non-loopback origins and foreign
credential paths. It never launches a substitute environment. Entry methods,
returnTo, real password/code/session, self/organization/commerce, late delivery,
renewal and logout/account changes are executable cases. Provider outage,
revocation and owner check/cleanup schema are prepared against the PLANNED owner
contract and must be verified against its formal implemented handoff. The runner
requires clean matching source checkouts and rejects Playwright debug/log/trace
environment settings before importing the browser library or reading credentials.
No cleanup flag means INCOMPLETE. The explicit flag invokes only the #357 CLI for
this run and requires the exact-source normal cleanup/zero-write evidence before
overall PASS; a failed browser case still remains FAIL even after successful stop.
It has not yet been run against a real provider. Rolling execution is in the PR.

## Evidence and execution rules

Use the installed Browser skill first. If the in-app browser demonstrably cannot
reach the task loopback endpoint, record the exact safe failure and use this
repository's Playwright dependency with a disposable empty Chromium context.
Automated regression remains isolated from default fixture suites: do not inherit
their webServer, storage state, synthetic identity setup, traces or screenshots
on arbitrary failure. Browser traces, HAR, video, raw errors/network payloads and
authentication screenshots after entering a password are not published.

Reports record only named assertions/status, path without query/fragment, HTTP
method/status, opaque generated identity IDs, source SHA, elapsed times and owner
zero-write/cleanup results. Assertions compare sensitive values in memory but
output only a fixed case label on mismatch. Sanitize exceptions before reporting;
Playwright call logs can contain filled values and OAuth callback queries. Capture
only explicit reviewed screenshots: empty official login form, authenticated
self/organization/entitlements pages with synthetic data, and logged-out state.
No password, token, cookie, code or private manifest in Git, logs or reports.

The final PR maps every M case and Issue checkbox to PASS / FAIL / NOT_RUN with
the exact tested source/dependency SHA. Tests that did not execute, missing
prerequisites or historical substitute runs are NOT_RUN. Independent final review
checks full browser -> provider -> Auth.js -> BFF -> Go -> PG call paths, no
credential/session injection, real tests executed, scoped mutations and cleanup.
CI and local/browser/provider evidence are distinct. Partial delivery uses Refs
#358; no merge, Issue closure or deployment is authorized.
