# ACC-2 membership implementation contract

Status: IMPLEMENTATION_READY for read and write protocol, confirmed by the
original independent reviewer in the second local review on 2026-09-12.
This is the frozen implementation baseline; final code/browser acceptance
still requires the implementation evidence below.
Authority: #410 / #408 and the user's Gate B permission decision (2026-09-12).
This is one bounded implementation mapping, not a replacement IAM design.

## Owners and admission

- ZITADEL project authorizations remain the sole member/role fact source. A
  member is an authorization in the configured project and Effective
  Organization, not a user whose Home Organization happens to match.
- `internal/organization/membership` owns this product's list/detail and action
  protocol. `internal/integration/zitadel/membership` adapts official APIs.
- `internal/authidentity`, the existing verifier and `workbenchcontext.Resolver`
  retain authentication, token expiry and current Organization grant ownership.
- Existing `authz.ListingKitAuthorizer.Authorize` evaluates the new explicit
  `workbench.organization_member.read` and `.manage` permissions. Viewer and
  operator have read only; organization admin has both; configured platform
  roles/users are explicitly projected by the existing policy owner. Neither
  `IsTenantAdmin` nor source-account permissions substitute for these checks.
- Every HTTP operation resolves the explicit Organization with the existing
  live-grant policy before the domain service. All provider writes and explicit
  recovery steps recheck current identity, expiry, exact Organization and manage
  permission. Replaying a receipt does not bypass revoked access.
- A dedicated read credential supplies `user.grant.read` and exact user read-back
  uses `user.read`. A separate write credential supplies `user.write`,
  `user.grant.write` and `user.grant.delete`. Neither `user.delete` nor
  `organization.delete` is needed. Reuse native provider permissions and current
  secret assembly; task-owned provider tests verify missing-permission failures.
  Neither is a browser token, bootstrap authority, nor persisted in receipts.
- No import of old memberinvite service, root ListingKit handler, tenantbridge,
  old numeric tenant model or legacy audit table. Valid HTTP/role/error behavior
  may be extracted; old platform-subscription orchestration is not reused.

## Read and public projection

The existing bounded directory adapter returns assignment ID, user ID, display
name, login identifier, roles, assignment state and provider timestamps. Exact
project/Organization is checked in both adapter and service. Missing, malformed,
cross-scope, over-bound or failed responses are unavailable, never empty.
List limits are 1..100; offset and total are bounded at 10,000. No avatar/raw
provider body or provider secret is exposed. Detail uses an exact assignment ID
filter with the same Organization/project checks, not an unscoped ID lookup.

Browser capability is only `{read, manage}` for these two permissions, computed
by the backend. It is tied to actor and Effective Organization and is not a
permission grant. UI clears old members/capabilities on switch, failure, expiry
and new requests; every request consumes cancellation. AccountShell is consumed
from #415 without copying its breadcrumb/navigation. Fields supplied in this
projection are current provider facts, not operation-receipt membership state.

## Stable operation and observed version

The command identity is `(project, organization, actor, UUID operation key)`.
A canonical payload fingerprint includes kind, target authorization/user,
requested roles or invite identity fields, and the expected observed version.
Same identity with a different fingerprint returns conflict. Same identity and
fingerprint returns the persisted outcome or verifies the existing dispatch;
it never silently creates a fresh command.

The observed version hashes the exact provider assignment ID, org/project,
user, roles, state and change timestamp. It is compared during fresh read-back
before dispatch. It is **not provider CAS** and does not prevent an external
ZITADEL administrator from writing between that read and dispatch. The API must
not claim stronger atomic version guarantees. Local reservations serialize this
application only. External administration can cause provider rejection, a later
write to win, or observed state differing after ACK. Return the true ACK and
latest observed state separately; never automatically rewrite external changes.

The browser chooses exactly one of `listingkit_viewer`, `listingkit_operator`,
`listingkit_admin`, excluding any of these names that the actual backend
PlatformAdminRoles configuration protects. The backend projects eligible
actions/options; the frontend never copies this configuration. The service
translates an eligible role to the assignment's role keys.
`platform_admin`, `admin`, configured platform roles and arbitrary role strings
are never assignable through this feature. Existing assignments containing
roles outside those three or any configured platform role (even a name within
those three) are read-only in this v1 (server enforced), avoiding
silent loss of system/configured roles. Remove deletes this exact authorization
only, never the user or Organization, owned assets, paid plans or entitlements.

## Durable protocol and failure boundaries

Admitted persistence owner:
`internal/integration/persistence/organization/membership`, with an explicitly
initialized `public.organization_member_operations` table. Receipts are action
protocol records, not a replicated directory. They hold identity/fingerprint,
bounded canonical input, target/generated IDs, before observation, dispatch
phase, safe provider acknowledgment and result/evidence kind. No tokens/raw
provider errors. A distinct minimum-privilege runtime DB role/pool in the same
PostgreSQL infrastructure is admitted: do not expand SA1
or commercial pool privileges, use legacy DBs, or perform request-time DDL.

Use short PostgreSQL transactions and unique keys for operation creation and
phase changes. Serialize active operations per Organization/target across
processes with durable reservation; never hold a SQL transaction across HTTP.
An unresolved dispatched operation retains its reservation. A same-target new
command cannot bypass unknown with a different key. A lost DB commit acknowledgment
is verified by reading the stable operation, not by calling the provider.

The reservation key is `(project, organization, verified targetUserID)`
independent of actor and action kind. Role/remove retain the exact grant ID as
an object precondition. Invite reserves its original generated user ID and also
has a separate normalized lowercase email creation-deduplication constraint.
Thus another actor cannot bypass an unresolved invite via a now-visible grant
ID to change/remove roles for that same user. Intent
creation locks that reservation in a short transaction and inserts the operation
and active reservation atomically. Another actor/key gets a conflict while it
is occupied. A persisted revision and expected phase guard each `ready ->
dispatched` compare-and-set; only one transaction winner may send. This is local
receipt CAS, never provider role CAS. A restart never steals a dispatched phase.

Each provider step follows:

1. Validate command and live authorization. Persist normalized intent and target
   identity. Invitation generates one random user ID at intent creation.
2. Freshly read the target and check scope/observed version. A conflict stops
   before dispatch; the command key cannot be repurposed.
3. Persist a single transition `ready -> dispatched` before HTTP. Only the
   request that confirmed that transition may send that step once.
4. A known success acknowledgment is persisted, then current provider state is
   reread. An acknowledgment proves the provider accepted the action; current
   read-back proves only the currently observed state. These are distinct facts.
5. Lost response, cancellation, crash/restart or lost completion commit leaves
   the durable dispatched phase unresolved. Recovery only reads the receipt and
   provider; it does not repeat a dispatched call or reset it to ready.
6. Explicit verify is owned by this membership service. No background scheduler,
   autonomous retry, provider compensation or deletion of partially created
   identities is introduced. Every verify/next-step requires current manage
   permission; receipts stay available for recovery after process rebuild.

For invite, `create user -> create authorization` is two separately recorded
steps. Fixed generated user ID and expected Organization/identity properties
allow exact user read-back. Before first dispatch, an already existing generated
ID is a conflict, never an accepted old object. After a lost response, only an
original dispatched intent plus exact-ID human read-back with matching owner
Organization, normalized username/email and required identity fields may become
`identity_verified`. This is a current identity prerequisite, not an HTTP ACK,
email-delivery proof or global claim of permanent ID non-reuse. Absence,
incomplete view, mismatch or missing permission remains unknown with the same ID.
After the first step is acknowledged or identity_verified, the second can
be dispatched once. If the first step is unknown, no grant is sent until its
identity is proven; if the grant step was dispatched, it is never resent.
Continuation retains the original intent, generated user ID, Organization and
payload. It never changes IDs, links an existing user found by email, or deletes
a partially created identity. Recovering an undispatched next step repeats live
authorization and exact identity ownership read-back before its dispatch CAS.
Email verification/setup delivery remains provider-owned and must not be
reported delivered merely because an identity or authorization exists.

Unknown update has no provider operation correlation ID. Equal desired roles
alone must not be called proof that this command completed: an earlier or
external write can match while the original request is still in flight. Keep
unknown and its fence. CreateAuthorization has no caller-chosen assignment ID;
matching listed roles is not operation proof. Delete also requires a valid ACK:
empty list/404 can reflect visibility or permission loss and remains unknown.
Only the exact user-creation phase proof above is admitted for continuation.
Do not let a late
old dispatch overwrite a later locally admitted command by releasing the fence
on ambiguous state equality.

Reservation release is permitted only for (a) a terminal pre-dispatch failure,
where no provider step was sent, or (b) a final provider success acknowledgment
durably recorded for the final dispatched step. The admitted user identity proof
only advances the invite's next ready step; it does not release its reservation.
No arbitrary non-2xx response proves absence of side effects. No TTL, request cancellation,
restart or user-selected replacement key releases it. For unknown Update there
is presently no decisive operation-correlated proof: it remains unknown and
occupied even if current roles match. An unknown final create-authorization step
also remains occupied, as does unknown Delete; plain equality or absence is
insufficient. Read-back is still returned as
`observed`, explicitly separate from `acknowledged` operation completion.

Fault/concurrency evidence required for these rules: two actors race one target
(one reservation and dispatch winner); lost intent/dispatch commit (zero send
without a known winner); crash after dispatch (no resend after reconstruction);
late provider response after unknown plus attempted new key (blocked); observed
desired roles while original HTTP is held (no release); invite user created with
lost response (original fixed ID only); grant-dispatch response loss (no resend);
undispatched grant after partial invite plus revoke (no continuation); tampered
platform role (no send); remove (no DeleteUser/DeleteOrganization request);
cross-actor invite/grant alias attempts (same targetUserID fence); configure
`listingkit_operator` as a platform role and prove ordinary member actions can
neither assign it nor alter its protected assignment. This action eligibility
constraint never introduces an IsTenantAdmin/PlatformAdm permission bypass.

No operation receipt is silently deleted/expired to allow a repeated key.
Admission is bounded per Organization and fails closed at the bound. Retention
and any operator-driven cleanup are outside this slice's runtime authority.

## Independent review decisions (2026-09-12)

The original reviewer admitted the four mappings now incorporated above:
observed precondition and external concurrency semantics; phase-specific ACK
versus identity proof; unified targetUserID reservation and restart protocol;
dedicated receipt owner/pool and exact native provider permissions. No proven
architecture BLOCKER remains. Concurrency, failure and permission evidence is
`IMPLEMENTATION_TEST`, required before slice acceptance. Second review only
confirmed this difference; no additional design chain is opened. The reviewed
document content before this status update had SHA256
`FA0F327CFFBC48A6D8E6F85766D3E2B45CC5D3CF2094F542973432F70CA9A879`.

Runtime/config/current application/cmd changes still require the PM's exact
single-writer window, including signatures, minimal diff and resource lifecycle.
Architecture admission does not authorize shared file occupation or real data.

Reviewer checked provider source at `a9311b8c702531832575351a663e98a2242778e5`:
[authorization service](https://github.com/zitadel/zitadel/blob/a9311b8c702531832575351a663e98a2242778e5/proto/zitadel/authorization/v2/authorization_service.proto)
and [human creation](https://github.com/zitadel/zitadel/blob/a9311b8c702531832575351a663e98a2242778e5/internal/command/user_v2_human.go).

## Verification and smallest implementation sequence

1. Existing adapter RED/GREEN retained. Replace the draft IsTenantAdmin service
   mapping with the frozen explicit member permissions; test viewer/operator,
   org admin, configured role/user, expiry, grant revoke and cross-org rejection.
2. Complete list/detail, capability projection, dedicated BFF/client and leaf
   rendering using the real current auth boundary and task-owned provider.
3. After write protocol admission, TDD intent/dispatch/receipt/read-back and
   invite/change/remove through task-owned PostgreSQL plus provider fixtures.
   Test same-key conflict/replay, same-target concurrency, each crash/response
   loss boundary, partial invite, removed actor, unknown and application rebuild.
4. Run actual browser -> Auth.js/BFF -> Go -> task-owned provider/receipt chain;
   desktop/narrow/keyboard/a11y, error/late response and permissions matrix.
5. Commit/push one primary stacked PR, then exact-HEAD relevant tests/build/CI,
   Code/Security and original independent full-slice review. Keep fixture/real
   provider, PASS/FAIL/SKIP/NOT_RUN separate. No merge, close or deployment.

Shared edits requested separately: authz policy; current application factory and
routes; opt-in membership config/DB lifecycle/credential assembly; #409-owned
navigation activation. Shared files remain untouched until PM assigns ownership.

## Implementation evidence, 2026-09-12

The PM assigned the five-file current-application/runtime/cmd window. Original
five-argument assembly and the two-pool, ten-route default remain covered.
Membership is opt-in with seven additional routes and a separately owned pool;
schema installation is explicit test-owned initialization, with no new CLI.
Startup checks exact column types/nullability, validated constraints, immediate
valid reservation indexes and effective runtime grants. Tests reject owner,
PUBLIC/inherited/column privilege expansion, missing UPDATE, schema drift and
aliased pool ownership. PostgreSQL reservation and dispatch races pass with race
detection, including restart/replay and durable payload/reservation divergence.

Mounted tests exercise real current Go auth/organization resolution, all three
PostgreSQL pools and the dedicated adapters against synthetic external HTTP.
Invite has separate user/grant ACKs; replay sends neither twice. Remove retains
the user identity. Cross-org receipts return 404, viewer writes/receipts are
denied, and unknown role updates remain unknown despite observed matching state.

The task-only `web/listingkit-ui/scripts/members-fixture.mjs` additionally runs
real Next 16.3.3/Auth.js/BFF/current Go/PostgreSQL. Session issuance and external
ZITADEL HTTP are explicitly synthetic. Browser checks covered list/detail,
invite, role change, confirmed removal, unknown-response reload, explicit verify,
viewer-only detail and org-switch receipt isolation. Production routes require
`force-dynamic`; writes reuse the existing trusted public-origin helper. React
effect replay and zero-byte POST streams have regression tests. Narrow layout
uses a keyboard-focusable horizontal table scroller. Automated accessibility and
final-HEAD browser replay remain separate checks, not inferred from screenshots.

Dedicated read permission observation is checked before and after directory or
human reads with an explicit target-org header. The fixed provider source at
`a9311b8c702531832575351a663e98a2242778e5` preserves project/grant suffixes; only
bare `user.grant.read`/`user.read` satisfies the org-wide read prerequisite.
This bounded observation is not an atomic snapshot and does not prove the
credential lacks broader authority. Required permissions are not an actual PAT
scope or native role bundle.

### Native provider verification

Clean source/web `97a16661b84c6473a79402f50fc66181fc08c94e` used fresh run
`835c9cb7-ae4e-4986-99d5-7f85d41e526e` from the unchanged
`scripts/issue357-runtime.mjs start --current-application`. Official API/Login
v4.17.1 ran with API digest
`sha256:3ac6910685d48f32481f01f45e3e6215efe5a9df2c069591b481e9a101712db5`.
After READY, the existing stop command stopped only Go/Next; native validation
used the dedicated adapters against the retained task-owned provider.

`ISSUE410_NATIVE_RUN=835c9cb7-ae4e-4986-99d5-7f85d41e526e go test -tags=integration -race ./internal/integration/zitadel/membership -run '^TestNativeMembershipProvider$' -count=1 -v`
passed. This is actual native list/create-human/read-human/create-grant/update/
delete-grant/retained-human evidence, not a fixture. Bootstrap was used only for
isolated setup, never as the tested adapter credential.

Native role assignments were reread: dedicated read identity has
`ORG_OWNER_VIEWER`, write identity has `ORG_USER_MANAGER`, and the denied identity
has no native role. The write role also contains `user.delete`, user feature and
session permissions; this is an actual native bundle, not least-privilege PAT
scope. No user/org delete endpoint was called. Explicit target-org read succeeds;
another org has no bare `user.grant.read`. The unprivileged native authorization
list returns 200/zero rows, while the adapter fails closed on missing coverage.
Revoking the read PAT makes native permission observation return 401.

The run's private #410 credentials file was deleted by exact path before the
existing destroy command. Cleanup reports passed, resourcesReleased and
portsReleased, with both applications exited 0. Its zeroWrite refers only to the
original commercial fixture. Sanitized role/scope/negative/cleanup evidence is
stored in `issue410-membership-native-evidence.json`; no PATs or human login
credentials are included. Native adapter evidence and the earlier production
BFF/Go/PG browser fixture remain separate verification layers. Final-HEAD CI
and independent full-slice review are still pending.

### Implementation review corrections and browser evidence (2026-09-12)

Finding classification: `IMPLEMENTATION_TEST`. A receipt transaction can wait
past caller expiry/cancellation/deadline. Every provider step now rechecks
context and current identity expiry after successful dispatch CAS and immediately
before sending. A blocked send retains dispatched/unknown and its reservation;
it never returns to ready or retries. The first eight expiry/cancellation cases
failed against the preceding implementation (tool output `7c060f`); all twelve
cases including deadline now pass with race detection in
`dispatch_expiry_test.go`. Provider adapters and native role configuration are
unchanged by this correction.

Finding classification: `IMPLEMENTATION_TEST`. Once a target identity has been
verified, original intent is reserved before mutable version/eligibility checks.
Existing revision CAS `ready -> rejected` records conflicts or a target that
disappears before dispatch. A concurrent dispatch cannot be released by rejection.
Role/remove stale-version tests first failed (`76374e`), as did the disappearing
target test (`f6ee7c`); `precondition_test.go` now passes. Receipt GET and same-key
replay expose the terminal rejection. The UI test proves a receipt 404 alone
does not enable closing pending work, while a durable rejection does.

`BACKLOG`: safe user recovery when the initial target read returns 404 before
target identity can be verified. Retain the original key. A missing receipt or
target does not prove another same-key call has not admitted work. No automatic
pending clear, replacement key, resend authorization, or reservation release is
derived from that absence. This exception does not cover verified-target
conflicts or same-key concurrency correctness.

Role editing now initially selects the member's existing single role. Multiple
roles require explicit selection. The operator-default test first failed
(`558b61`) because the form selected viewer. The unused frontend `Member` type
export was removed; pinned Knip 6.32.2 now reports `issues: []` without changing
its rules, baselines or consumers.

Browser fixture `issue410-browser-WXaKmd` started at clean source
`34da2e21521bf9f442c456a81fa08adc059969ed`. It exercised real Next/Auth.js/BFF/
current Go/three PostgreSQL pools with synthetic session issuance and external
provider HTTP. Successful operation keys were invitation
`2094261d-3a2b-4e3d-b071-07fe18891e6e`, role update
`a06034c4-c1be-4527-9b4e-eba7e08af08c`, and removal
`62e70936-fcaf-41ae-b134-144be0685675`. Unknown update
`cc1d80dc-67df-47a7-9213-d89779e7ef92` retained the same key and unknown state
after page reload and explicit verification despite the directory showing the
requested role. Viewer had no invitation or mutation controls.

The role-selection UI correction was hot-reloaded during that run; therefore
this is not a clean final-HEAD replay of the subsequent corrections. The Go
process remained at its startup version. At 390px viewport, page scroll width
was 390px and the named table region was 356px wide with 580px scroll content.
The region accepts keyboard focus, and the invitation email field advances to
first name with Tab. Axe checks reuse the existing dependency and verify DOM
semantics in jsdom with contrast disabled; they do not certify rendered contrast
or full WCAG conformance. The task fixture cleanup reports `goExit: 0` and
`nextStopped: true`.

The final full-slice review also identified cached directory/capabilities after
mutation, verification or receipt authority failures (`IMPLEMENTATION_TEST`).
Nine cases covering 401, 403 and explicit org-context changes first failed
against 3ab2293d6 (`e76f82`). The UI now hides old directory/details/capabilities,
retains the original operation key and requires a subsequent successful matched
directory refresh before restoring capabilities. Detail-read authority failures
share the same behavior. A directory refresh started before the authority
failure cannot restore capability afterward. No automatic POST or new operation
key is introduced. All 34 member frontend tests pass after this correction.
