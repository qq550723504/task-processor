# Referral registration acceptance runner

This document describes the task-owned acceptance runner for Issue #413 C2, its M1 identity matrix, and its M2 runtime matrix. It validates the frozen referral registration contract against official ZITADEL Login V2, real Next.js BFF and Go processes, PostgreSQL, Caddy, and Mailpit. The runner does not repair product code and does not access shared or production resources.

## Scope and authority

- Run only from a clean checkout whose HEAD is the C2 candidate.
- The C2 branch is stacked on the exact C1 dependency recorded in the PR; M1 is stacked on the fixed C2 candidate.
- The runner creates a random Issue 357 runtime and adds only resources labeled with its exact run ID.
- It may delete only resources whose manifest ID and owner label both match that run ID.
- It must not print or save passwords, OTPs, cookies, access tokens, referral service credentials, proof values, resume secrets, or reusable verification links.
- A failed or unknown business, cleanup, residual inspection, or report write result exits nonzero.
- A later successful cleanup pass does not replace an earlier failure or unknown result.

## Prerequisites

Use Windows with Docker Desktop's Linux engine running. The repository commands used by the runtime must be available: Git, Node.js, pnpm, Go, and Docker Compose. The runner uses the repository-pinned official images and the current application's configuration; it does not accept shared service endpoints or pre-existing credentials.

Install the locked web dependencies from the repository:

```powershell
Set-Location web/listingkit-ui
pnpm install --frozen-lockfile
pnpm exec playwright install chromium
```

The authoritative base image declarations are `scripts/issue357/compose.mjs`: ZITADEL API and official Login V2 `v4.17.1`, Traefik `v3.6.8`, and PostgreSQL `17.2-alpine`. The C2 runner declares Caddy `2.11.4-alpine` and Mailpit `v1.30.4`. Every run records the locally resolved image IDs and repository digests in its private manifest/report; the tag alone is not digest evidence.

The runtime creates bootstrap, machine PAT, database, Auth.js, referral service, lookup, proof, and encryption credentials inside the random run directory. Files are mode `0600` where the platform supports POSIX modes and are never command inputs from a shared environment. Official verification mail stays in the task-owned Mailpit container. The report saves only booleans, counts, statuses, hashes, image identities, and bounded error codes; credential values, verification codes/links, cookies, tokens, proofs, and resume capabilities remain private.

Confirm the candidate and clean state before every official run:

```powershell
git rev-parse HEAD
git status --porcelain
git merge-base --is-ancestor 8281f775fcc60dfeb3f1074f321963de9ce95b16 HEAD
```

The second command must print nothing. Record the full 40-character output of the first command; do not reconstruct a SHA from an abbreviated value.

## Fast lifecycle and failure tests

These tests validate fail-visible orchestration, actual child-process failure handling, atomic report rename failure, dispatched-runtime manifest failure, continued cleanup actions, sanitized fallback output, and the order/failure/finally contracts for M1 and M2 controls. M2 regressions reject unhealthy identity prerequisites, changed receipts/counts, false parallel-process claims, secondary-process cleanup gaps, late dispatch, leaked observer requests, and any missing, duplicated, failed, or `NOT_RUN` Must item. They do not start the official business runtime and are not business acceptance.

```powershell
node --test scripts/referral-registration-fixture.test.mjs
```

The test process must exit 0 with no skipped tests. Individual test-only CLI scenarios require `ISSUE413_FIXTURE_TEST_ONLY=1`; they are internal to the test file and are not substitutes for the official command.

After changing configured application lifecycle wiring, run the bounded configuration smoke before another full chain:

```powershell
$env:ISSUE413_CONFIGURATION_SMOKE = '1'
node scripts/referral-registration-fixture.mjs
Remove-Item Env:ISSUE413_CONFIGURATION_SMOKE
```

The smoke intentionally exits 1 with business code `CONFIGURATION_SMOKE_COMPLETE` after the configured Go and Next.js applications pass health checks. Both cleanup passes and evidence persistence must still pass with all residual counts at zero. This only proves configuration/start/cleanup wiring.

After changing M2 process isolation or the dispatch observer, run its bounded smoke as well:

```powershell
$env:ISSUE413_M2_CONFIGURATION_SMOKE = '1'
node scripts/referral-registration-fixture.mjs
Remove-Item Env:ISSUE413_M2_CONFIGURATION_SMOKE
```

It starts and stops the copied run-private UI, second real Go/Next pair, and observer, then intentionally exits 1 with `M2_CONFIGURATION_SMOKE_COMPLETE`. Both cleanup passes and all residual counts must pass.

## Official command

Run the complete task-owned chain from `web/listingkit-ui`:

```powershell
node scripts/referral-registration-fixture.mjs
```

The final sanitized JSON line contains the run ID, invocation ID, business status, both cleanup passes, report evidence status, and named check outcomes. Every planned matrix item is a Must. If any item is missing, duplicated, `FAIL`, or `NOT_RUN`, including `F.screen_reader`, the whole command must report `MATRIX_MUST_INCOMPLETE` and exit nonzero. A successful automated M2 slice therefore remains distinct from full C2 acceptance until a real screen-reader result exists.

Private evidence is written under:

```text
%TEMP%\task-processor-issue357\<run-id>\referral-registration-evidence\
```

`report.json` is the authority for the run. Screenshots and sanitized axe files support the report. Diagnostic logs are private local artifacts and must not be pasted into an Issue or PR without a fresh secret review.

## Report semantics

The report separates four conclusions:

- `business`: official runtime and A-F behavior.
- `cleanup.initial`: the first cleanup attempt, including each action and exact residual counts.
- `cleanup.final`: an independent second attempt; it cannot erase the first result.
- `evidence`: atomic report persistence.

Residual inspection reports exact counts for containers, volumes, networks, and listeners. A query failure is `UNKNOWN`, never zero. A dispatched runtime with an unreadable or mismatched manifest is ownership-unknown; the runner records it and refuses to claim or delete unproved resources.

The top-level conclusion is `PASS` only when business, initial cleanup, final cleanup, and evidence all pass. A report persistence failure emits only a bounded sanitized fallback object and exits 1. Atomic persistence writes a private temporary file, renames it into place, and removes the temporary file on failure.

Runner provenance records two different SHA-256 values: `runnerSha256` hashes the exact executed bytes, while `runnerNormalizedLFSha256` normalizes CRLF and lone CR line endings to LF before hashing so it can be compared with the Git blob's content. `runnerByteFormat` records byte length, BOM presence, and line-ending counts. Do not describe the raw byte hash as the Git-blob hash when the checkout contains CRLF or mixed line endings.

## A-F matrix

The report's `matrix` array records each item independently so an early product failure does not hide unaffected checks.

### A. Admission and browser recovery

- Commit the real admission upstream, drop its browser response, and retry the exact body and idempotency key.
- Compare the replayed Intent, fixed subject, and original receipt capability in memory without writing the capability.
- Reject the same key with changed payload.
- Reload with the recovery fragment and resume only the original Intent.
- Open a fresh browser and prove an empty form does not auto-submit; separately pass an Intent without its capability and prove no recovery request or identity switch is dispatched.

### B. Official identity lifecycle

- Refuse to bind an existing account to a newly admitted fixed subject.
- Submit an invalid official verification check in a separate browser and prove it does not verify the user.
- Complete an initial valid official email verification, interrupt before authenticator enrollment, and prove the continuation is rejected in a fresh browser.
- Use ZITADEL's official replacement invite-code flow for the same verified task-owned subject, consume a new Mailpit-delivered Login V2 verification link, then complete the first authenticator and OIDC/Auth.js flow with the same subject. Repeating `SetEmail` with an unchanged address is not this control and is rejected by ZITADEL.
- Prove another authenticated subject cannot claim the Intent.

### C. Completion and durable receipt

- Commit completion upstream, drop the real browser response, restart the actual Go and Next.js processes, and replay the durable receipt.
- Execute concurrent first completion and later concurrent receipt replay, and prove all responses match one receipt.
- Query PostgreSQL for exactly one relationship, one receipt, and one consumed wiped Intent.
- Inject a browser-visible 503 for the post-completion projection and prove the visible receipt remains while the summary is unavailable. This does not count as a real Provider business-read failure.
- Keep the current identity, OIDC discovery, JWKS, and Auth.js provider health checks green. Start a run-owned secondary Go/Next pair whose Go Provider origin rejects only `GET /v2/users/{subject}` and metadata reads while all other Provider paths remain proxied. Replay the already committed completion receipt through that secondary BFF, require byte-equivalent receipt JSON and an unchanged relationship count, then stop the secondary pair and prove a healthy Provider user read succeeds. Whole-Provider shutdown and a browser-only projection failure do not satisfy this case.

### D. Personal authorization and GET purity

- Read the current subject's personal projection.
- Prove an admin sees only the admin's own empty projection, not another user's relationship.
- Prove the task-owned no-enterprise user can read its own empty personal projection.
- Hash all four referral fact tables before and after an authenticated GET and require equality.
- Establish a dedicated official viewer login, select enterprise B, hold its real organization read, and require the captured upstream response to be HTTP 200 for that viewer and effective enterprise B. Deactivate the task-owned ZITADEL project authorization, wait for the authoritative context to clear B within the documented cache bound, refresh the UI, switch to an enterprise that remains in that authoritative authorization result, release the late read, and require that surviving enterprise to be the visible final state. The held read must be observed as delivered to the cancelled request or cancelled before delivery; an invalid upstream response, injection failure, or unknown visible state fails the control. Home organization is an identity fact and is not treated as project authorization.
- Preserve the same subject's personal referral count through a fresh official login after the potentially cache-bound enterprise control, so short-lived session age cannot masquerade as personal authorization failure. Prove the admin still reads only its own empty personal projection; restore the task-owned authorization in `finally`.
- Establish a dedicated official login for this control, start with a successful profile read, and require the held upstream profile response to be HTTP 200 for that original subject. Delete that subject's official ZITADEL sessions, prove they are absent through both search and actual GET requests for every deleted session, log out, change to the task-owned admin identity, and prove the captured old profile response cannot backfill the new identity view. An invalid upstream response or failed late-read injection is a control failure.

### E. Boundary and dependency behavior

- Traverse Caddy to the Next.js BFF and Go with the trusted source mapping; forged client headers must be overwritten.
- Reject direct Next.js access, missing or wrong Go service credentials, a Provider machine token used as a service credential, and missing same-origin write headers.
- Read the server-only Auth.js acceptance token, validate it as a real human OIDC token through `userinfo` plus the Provider user record, and place that token in the service-credential header. The probe runs while the task-owned business database is stopped and must still return 401/403; the exact checked source must also place `trustedCommand` and its return before `commands.Start`. This evidence excludes business-storage dispatch through a dynamic outage plus the checked guard path; it is not a direct in-process handler-call counter, and the report says so.
- Remove the task-owned service credential while the application is running, render a fresh authenticated page, and prove the invite entry is disabled; restore the credential in `finally`.
- Use two real Docker peers and a real Go/Next process restart for source-IP and persisted rate-window evidence. This is not evidence for two simultaneously active application instances.
- Start a second real Go/Next pair on independent dynamic ports with its own process, ready, stop, and shutdown files. Both pairs share the exact run-owned PostgreSQL database and use separate connection pools. Alternate one Docker source across both live BFFs in one fresh window: the first five admissions must return 200 and the sixth, sent through the other live instance, must return 429; a second Docker source must still return 200. Record both process identities, both route counts, the shared database container identity, and the secondary backend proxy dispatches.
- Route the secondary public path through a run-owned transparent BFF-ingress observer and route only the secondary BFF's business calls through a separate loopback Go-dispatch observer. The dispatch observer never drops a request merely because a test mode is armed. The same held path first receives a live healthy request, is explicitly released, reaches Go exactly once, and returns 200. For post-body cancellation and deadline controls, require the real BFF-to-dispatch-observer connection to close before explicit release; releasing afterward must complete the observer handler with zero Go calls. Separately stream a partial browser request body and wait until the ingress observer has received a body chunk, connected to the real Next.js BFF, and flushed that chunk upstream while the request body is still incomplete. Only then cancel the browser request, require both ingress sides to close, wait past the 15-second total deadline, and require zero BFF business dispatch and zero Go dispatch. Record local enqueue, ingress receipt, body-forward completion, incomplete-body state, BFF observer receipt, Go dispatch, real connection closes, explicit releases, response completion, handler completion, and open handlers as distinct observations. Already dispatched operations retain their original idempotency/receipt/unknown-outcome rules; these zero-dispatch controls do not claim rollback.
- The secondary supervisor, Go, Next.js, observer, configuration directory, and every dynamic listener belong to the random run. Both cleanup passes stop processes by their recorded identities, verify released ports, and remove the run-private configuration. A cleanup or observation failure remains nonzero/unknown.

### F. User interface and accessibility

- Save desktop and 390x844 narrow screenshots for registration, completion, and overview pages.
- Exercise keyboard focus on each narrow page.
- Run axe on each page and save both violations and incomplete results as rule IDs, impacts, and node counts.
- Serious or critical axe violations fail the check.
- Real screen-reader execution is separate and remains `NOT_RUN` unless a screen reader was actually operated.

## Failure handling and handoff

Keep every real Must failure as `FAIL` and every unexecuted case as `NOT_RUN`. Continue independent matrix items where the runtime remains safe. Send product failures to the existing C1 owner through PM; C2 must not edit C1 product, schema, adapter, authentication, or application files.

For a rerun after interruption, first use the report's exact run ID and inspect the existing report. The normal command always performs both cleanup passes. If only private C2 artifacts remain after all runtime resources are independently confirmed absent, remove them with:

```powershell
node scripts/referral-registration-fixture.mjs cleanup-artifacts <run-id>
```

This command validates the UUID, fixed temporary root, and manifest ownership before removing only C2 private files. It does not discover, adopt, or delete an unknown runtime.

## Final manual activity

M1 and M2 keep their exact stacked Draft boundaries and independent fixed-HEAD reviews. The only acceptance activity after an approved automated M2 is real screen-reader operation on the final three-page product version. Axe, screenshots, DOM inspection, keyboard-only evidence, the manual-session environment flag, and a syntactically valid record cannot change `F.screen_reader` from `NOT_RUN`.

The user designates the actual operator and a durable Issue comment records that designation. Do not start a headed runtime while waiting for an operator. At the agreed time, the same fixture owner uses a clean exact candidate and a first terminal in `web/listingkit-ui`:

```powershell
$env:ISSUE413_SCREEN_READER_SESSION = '1'
$env:ISSUE413_SCREEN_READER_DESIGNATION = 'https://github.com/qq550723504/task-processor/issues/413#issuecomment-<designation-comment-id>'
node scripts/referral-registration-fixture.mjs
Remove-Item Env:ISSUE413_SCREEN_READER_SESSION
Remove-Item Env:ISSUE413_SCREEN_READER_DESIGNATION
```

This mode launches the same Playwright Chromium visibly and otherwise runs the same isolated official chain. It does not change the browser or BFF 15-second deadlines, the database-clock 15-second create lease, the fixed 15-minute creation deadline, or the fixed 24-hour completion deadline. No human wait occurs inside a provider call, database transaction, lease, BFF dispatch, or in-flight request. For the two transient pending states the operator is ready before activating the control; the request then runs naturally with its original client and server budgets. A late record cannot revive an expired state or create a new identity.

The runner writes a private checkpoint status and prints only the checkpoint ID and run ID. In a second terminal, read the current safe state without extending it:

```powershell
node scripts/referral-registration-fixture.mjs screen-reader-status <run-id>
```

`phase=action-ready` means the operator must use the visible browser to activate the named control. Do not acknowledge that phase. After the real request reaches its stable result, `phase=observation` exposes a new nonce while retaining the same checkpoint attempt ID and actual transition timestamps. The writer faithfully transcribes the operator's observation into the exact private `inputFile` printed by `screen-reader-status`, then records it:

```json
{
  "checkpointId": "copy from status",
  "sequence": 1,
  "nonce": "copy from status",
  "checkpointAttemptId": "copy from status",
  "operator": "user-designated operator name",
  "designationReference": "copy from status",
  "screenReader": { "name": "actual product", "version": "actual version" },
  "browser": { "name": "copy from status", "version": "copy from status" },
  "observationSource": "where the live operator supplied this record",
  "viewport": { "width": 1440, "height": 1000 },
  "readingSequence": ["actual ordered reading steps"],
  "controlSequence": ["actual operator controls; distinguish runner actions"],
  "announcedText": ["actual bounded non-secret announcements"],
  "visibleErrors": [],
  "announcedErrors": [],
  "result": "PASS",
  "observedAt": "actual ISO-8601 timestamp"
}
```

Write that file atomically, then run:

```powershell
node scripts/referral-registration-fixture.mjs screen-reader-ack <run-id>
```

The acknowledgement is bound to the exact run, source and web SHAs, normalized runner SHA-256, checkpoint order, state attempt, nonce, real viewport, browser and Windows versions, designation, and observation time. Wrong-run, wrong-source, stale, duplicate-conflicting, out-of-order, inconsistent-operator/software, direct-status, test-control, oversized, secret-bearing, email-bearing, or reusable-link records are rejected. Replaying the exact same decision is idempotent. A test-only fake record tests the control only and can never enter formal `F.screen_reader` evidence.

All eight states are required once, at their actual viewport:

| ID | Real state and required operator evidence | Runner boundary |
| --- | --- | --- |
| `registration-initial` | `/referrals/register`: main heading, read-only invitation code, email/given/family labels, focus order, and empty/invalid-email validation feedback | No Intent or POST exists; runner later fills the valid task-owned data |
| `registration-pending` | Operator activates Start; the focused button's real submitting/disabled transition is heard | Existing response-loss attempt runs on the original client/BFF budgets; runner records the actual transition and attempt |
| `registration-unknown` | Alert for unknown registration outcome, locked original fields, and the original-request retry control | Upstream admission is already committed and the browser response was lost; runner, not the operator, performs the recorded original-key retry after observation |
| `registration-mail-pending` | Official-mail status, official verification explanation, and continue-to-login control | Original Intent has resumed; there is no pause inside create permission, lease, or Provider work |
| `completion-initial-pending` | Completion heading/content, real personal projection, unavailable revenue, and the pending transition after the operator activates Complete | Existing completion response-loss attempt runs without a manual network hold or deadline reset |
| `completion-receipt-projection-unavailable` | Committed receipt/status/time and projection-unavailable state are both announced without rewriting success | Real completion is committed/replayed and the existing projection 503 has completed |
| `overview-entry-available` | Viewer overview, real relationship count 1, unavailable revenue, code, focus, and safe invitation-link navigation to the registration heading and back | Runner reads the current viewer's task-owned facts and does not activate the link |
| `overview-entry-unavailable` | The same viewer's real count remains readable while registration entry is unavailable and no active invitation link exists | Existing run-owned service-secret fault is restored in `finally` |

Stable checkpoints have a maximum 15-minute observation wait and the whole manual session has a 90-minute hard limit. The post-admission UNKNOWN checkpoint is limited to 120 seconds and must retain safe remaining time in the original 15-minute creation window; the official-mail checkpoint is limited to five minutes. A real expiry, timeout, browser close, operator abort, acknowledgement race, or write failure remains its actual failure or `NOT_RUN` result and enters cleanup. It is never replaced with a new key, Intent, identity, or synthetic state.

Abort from the second terminal without killing the fixture process:

```powershell
node scripts/referral-registration-fixture.mjs screen-reader-abort <run-id>
```

Acknowledgement and abort compete for one atomically published decision file, so only one complete record can win. Each checkpoint has its own input path, preventing a late acknowledgement cleanup from deleting the next checkpoint's input. During the run, `screen-reader-observations.json` is explicitly marked `CHECKPOINT_SNAPSHOT`; it is not final session evidence. Every exit uses the existing lifecycle: browser and pending waits end, task-owned faults restore in `finally`, then initial and final cleanup run independently. After both cleanup passes, any unfinished manual session is marked `TERMINATED` with its actual business termination reason, a known human FAIL stays FAIL, and the terminal manual evidence is atomically persisted to both `screen-reader-observations.json` and the final report. The owner must freshly query containers including stopped containers, volumes, networks, processes, and every manifest port. Query failure is `UNKNOWN`, never zero. Run-private nonce, input, decision, credentials, and browser state are removed; the private report retains the complete bounded human-source records, including actual reading/control sequences and errors, rather than only checkpoint PASS labels.

Only eight valid observations from the same user-designated operator, screen reader, browser, exact source and run can make `F.screen_reader=PASS`. Any actual operator failure makes it `FAIL`; missing or unfinished evidence stays `NOT_RUN`. Record the actual screen reader and version, Chromium version, Windows build, page path and viewport, state attempt, reading and control sequence, visible or announced errors, operator, source reference, date, exact product and runner provenance, result, and final zero-resource evidence. Keep secrets, full email addresses, codes, query strings, fragments, credentials, recovery capabilities and verification links out of the input, report, Issue and PR.
