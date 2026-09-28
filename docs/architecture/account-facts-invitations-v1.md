# Account facts and invitations v1

Execution: #551. Design Basis: Independent Architecture.
Admission: IMPLEMENTATION_READY, including automatic SMTP notification.
Baseline: main `8478800e31bf2e126bd03fae5ec45536056d823e`.

## Product authority and outcome

The user requested fixing the Account Center's hardcoded missing fields and
explicitly included the formal invitation flow in this delivery batch.
Authority: `docs/product/final-ui-ia-authority.md` and Figma file
`tg48P46SSXl6TBy9lZwg63`, nodes 432:323 (profile), 451:493 (settings),
432:4483 (enterprise), 1532:359 (members), 1541:359 (invite drawer),
1676:363 (pending invitations), 1678:359 (recipient confirmation).

Users can read actual account facts, save their region, inspect current
enterprise members/stores/resources and create, accept, decline or cancel a
real enterprise invitation. Acceptance, not sending or creating an identity,
establishes membership. Scope includes the existing three assignable roles.
No custom roles, payment, quota rule changes, generic notifications, global
activity recorder, old-data migration or unapproved runtime deployment.
The user selected automatic mail or SMS notification in this batch. Use an
SMTP mail adapter first, with automatic notification for email invitations;
do not open phone-only automatic invitations until an actual SMS notification
adapter/configuration exists. Mailpit is only the isolated localhost mail
transport; production SMTP configuration is separate. No fake sent state.

## Existing owners and read contracts

| Fact | Authority | Consumer path |
| --- | --- | --- |
| Registration | ZITADEL Auth v1 GetMyUser | SelfServiceClient → account identity read → BFF → profile/header |
| Current session authentication time | Official OIDC `auth_time`, validated initial Auth.js profile | Encrypted Auth.js session → authenticated session BFF → profile/header |
| Name, verified email/phone | ZITADEL current user | Existing self-service/userinfo clients; never local copies as identity authority |
| Country/region, province, city | AccountProfile user preferences | Preferences repository → self-scoped HTTP → BFF → settings/readback |
| Membership and roles | ZITADEL authorization v2 | Existing Directory; scoped project and organization |
| Member/admin/inactive totals | Same ZITADEL ListAuthorizations filtered totals | Summary reader → live-authorized member summary → members/enterprise |
| Role permission scope | Existing Casbin authorizer | Backend projection of allowlisted current permissions; no policy changes |
| Recent enterprise activity | Existing account audit owner | Actor-filtered latest event for the current enterprise; disclose coverage |
| Registered store count | StoreCenter | Existing authenticated store list's pagination total; never source account count, connection status or entitlement limit |
| Resource balances | Commercial owner | Reuse #550 enterprise resource read and display |
| Pending invitation count/status | Membership invitation protocol | New invitation repository, separate from provider membership directory |

Pinned provider authority: ZITADEL v4.17.1 `proto/zitadel/auth.proto` declares
`user.details.creation_date` and `last_login`, but the actual
[`internal/api/grpc/auth/user.go`](https://github.com/zitadel/zitadel/blob/v4.17.1/internal/api/grpc/auth/user.go)
GetMyUser handler only fills `User`, not `LastLogin`. A missing value therefore
does not prove that the user never logged in. The original last-login product
requirement is replaced by the user's 2026-09-28 decision: show official OIDC
current-session authentication time and do not build a global login history.
The historical preliminary last-login FAIL remains historical evidence.

Registration still checks returned user ID against the verified token subject.
Authentication time comes only from the initial ZITADEL OIDC profile validated
by Auth.js: positive safe integer `auth_time` seconds, no future value, rendered
as UTC ISO time. Save this value in the existing encrypted session; preserve
it on token refresh for the same subject. Missing/invalid claims, identity loss
or a changed refresh subject expose no date. Do not substitute current time,
token issuance/refresh time, provider password-check time or membership time.
The authenticated session BFF returns only identity and this timestamp; no
credentials. The UI checks the returned subject, scopes its query by user and
keeps registration reads independent. Existing sessions without this claim
show unavailable until a new official sign-in, without backfill or fallback.

The bounded preliminary-acceptance repair reuses current Store, Membership and
Allocation read contracts. Resources display Store `pagination.total`; member
Token rows join the existing monthly-AI-point-limit Directory role projection
by canonical member ID under the same expected user/organization. A failed or
unmatched role read remains unavailable. Per-member stores, renewal periods
and data-row quotas have no current owner and remain unprovided.

Entitlements separately display the existing member-token-allocation reader's
enterprise total, allocated/unallocated amounts and period in exact Token
units. This is not the AI point balance, model usage or a new Commercial usage
metric. The Commercial five-metric read contract remains unchanged. Pending,
failed and mismatched scoped reads hide previous values, never synthesize zero.
No schema, authorization, billing, invitation or recovery protocol changes.

Region is an application localization preference belonging to a user, not
an enterprise identity or ZITADEL credential. Persist it in a new
`account_user_preferences` table in the existing AccountProfile database,
keyed only by verified user ID, with country/province/city and updated_at.
Do not put it in the enterprise-scoped business profile. PUT replaces only
these three bounded strings, trims whitespace, rejects control characters,
invalid UTF-8, unexpected keys or more than 128 bytes per field; no external
geolocation lookup or invented geographical catalog. A successful save is a
committed row and subsequent GET is authoritative. Blank is legitimately
'not set'. Reads/writes require authentication, no client user/org field.

Summary totals use fixed role/state filters under exact project/org. Query
the provider totals, not enumerate a directory or extrapolate the loaded
page. Counts are separate current reads, not an atomic cross-system
snapshot; report readAt. Provider failure is unavailable, never zero.
Administrator means current listingkit_admin role; inactive means inactive
authorization, not user's global disabled state. Project current permission
IDs with the actual member's user ID and roles through the existing
authorizer (including existing platform-admin configuration), without
turning this display into an authorization source.
Inactive authorizations project no effective permissions. The invitation
confirmation projects the intended ordinary role through the same fixed
permission allowlist; this HTTP view is derived, never a persisted grant.

Recent activity is the latest recorded account operation for this actor in
the current enterprise, reusing the existing account-audit read. Label its
coverage clearly. No recorded operation is different from failed read, and
is not a claim that the user never used the application. Page queries are
bounded; do not silently retrieve another enterprise's activity.

## Invitation fact owner and API

Membership owns only invitation intent, consent and dispatch/recovery facts.
ZITADEL remains the sole authorization/member directory. No local 'member'
row, no automatic new human user, no home-organization transfer, no paid
plan/entitlement overwrite. A recipient without an account uses the existing
official ZITADEL registration/verification flow before acceptance.

New `organization_member_invitations` belongs to the existing independent
membership receipt database and runtime role. Indexed fields: project_id,
organization_id, invitation_id UUID, normalized email contact, state,
revision and expires_at. A bounded object JSON receipt stores those facts
plus creator ID, intended role, fingerprint, created_at, recipient ID,
dispatch ID, confirmed authorization ID and mail attempt facts. Reads check
the receipt against indexed columns. The primary key is project/invitation ID;
replay also requires the same organization, creator and payload fingerprint.
A partial unique pending+accepting contact prevents concurrent live
invitations for the same enterprise/email. ID is not an acceptance
credential: even knowing the link cannot disclose recipient details or join
without a fresh verified matching contact. Expires after seven days.

Admin routes list/create/read/cancel invitations and read pending count.
They consume current Effective Organization plus existing live member.manage
authorization; read-only summary respects existing member.read permission.
Create input is only contact and ordinary role, plus UUID idempotency key.
No names, user IDs, arbitrary organization IDs or passwords. Same key/same
payload returns the same saved invitation (also after terminal states), same
key/different payload conflicts. Reject a pending duplicate. Expired rows
are marked expired atomically before a new invite reserves the same contact.

Recipient routes under a separate fixed `/account/invitations/:id` path
allow authenticated reads and accept/decline without a pre-existing grant
in the target enterprise (OrganizationAccessPolicyNone). They do not use a
browser-provided organization/user ID. Find invitation by ID, then compare
fresh ZITADEL self-profile contact to the stored normalized contact:
verified email, case-insensitive whole address. Phone invitations are not opened.
Unverified/mismatched contacts receive permission denied with no details or
mutation. The application uses official verification; no local OTP/Consent
substitute. Responses bind current recipient user, invitation ID and owner
organization. The user sees the actual enterprise, inviter, role, expiration
and 'accept and join' confirmation. Recipient IDs/contact values are not
logged. GET is never acceptance and link prefetch has no write effect.

## State, transactions and external side effect

States: pending → declined/cancelled/expired, or
pending → accepting → accepted. `accepting` is durable UNKNOWN until a
scoped provider read confirms the intended grant. No resend of a dispatched
creation, including a timeout/404, cancellation, restart or lost ACK.

Accept sequence:
1. Fresh recipient authentication and verified matching contact; validate
   expiry and current ordinary role. Query exact target org/project/user:
   existing authorization is a conflict, never change its roles.
2. Independently read the creator's current active grant in the target enterprise
   and authorize member.manage through existing rules. If withdrawn,
   acceptance is denied. Do not trust the creator's stored role/receipt.
3. Short database transaction locks invitation. Require pending, unexpired,
   revision and current recipient; durably transition to accepting with
   recipient ID and unique dispatch ID before any external call. Concurrent
   cancellation/decline/accept has one serialized winner. Repeated accept of
   accepting only reconciles; terminal states cannot dispatch.
4. Check context deadline and repeat fresh recipient/contact and creator
   permission checks immediately before dispatch. Call the existing scoped
   ZITADEL writer once, only CreateAuthorization for the current user and
   stored org/project/role. There is no identity creation/email verification
   mutation. Committed dispatch with no send remains UNKNOWN, not retryable.
5. Read scoped provider authorizations (by recipient user); confirm exactly
   one matching active role/grant and store accepted + authorization ID.
   ACK alone does not establish accepted. Lost ACK/failed local save is
   reconciled on a later recipient read/accept or authorized admin read;
   never recreate. A conflicting grant remains an explicit unresolved
   conflict and is never overwritten/deleted.

Canonical provider confirmation may be read-only even after creator
revocation, but a new write may not. Unknown results cannot be cancelled or
released to invite again; UI exposes retained status and read-only check.
No automatic background reconciler is introduced. The invitation service
is the single recovery owner and bounded demand-driven reads are its entry.
Provider grants use its existing uniqueness; concurrent unrelated membership
writers cannot cause duplicate grant/role overwrite via invitation accept.

Cancel/decline lock the same row, require pending and current actor/recipient
permission, and persist a terminal fact without provider side effects.
Expired is computed from authoritative stored expiry and persisted during
mutations; counts exclude expired pending. Resource allocation stays in the
existing member allocation path, possible only after canonical membership.

Legacy decision: N/A. The current immediate-create-user invitation action
is superseded for new invitations, and its create entry is disabled/removed
from current UI/HTTP admission. Existing membership operation receipts and
their bounded read/verify path remain available for already-dispatched work;
especially #492 UNKNOWN is neither migrated, retried nor released. This is
not a compatibility fallback or a second new invitation path.
The obsolete immediate-create-user HTTP submission, BFF and client command
are retired; reading an existing receipt does not admit a fresh submission.

## Automatic mail notification

The saved invitation is the notification intent/outbox; add delivery status,
attempt count, attempt ID and updated_at to that same owner row. Create
commits invitation before SMTP, then claims a bounded delivery attempt in a
short transaction. Call a maintained SMTP library (wneessen/go-mail) with a
context deadline, fixed configuration and validated addresses. On SMTP
acknowledgment record mail_server_accepted; it does not prove inbox delivery.
An error/response loss is delivery_unknown, not invitation acceptance.
The API still returns the durable invitation when SMTP fails, with the actual
notification state and an explicit retry affordance. No silent automatic
loop. An admin-only resend uses the same invitation/link and checks current
manage permission, pending/unexpired state and a serialized attempt claim.
Repeat email is an accepted transport risk: it is the same immutable consent
link and never grants membership, extends expiry or creates another user.
Recipient opt-in and unique grant dispatch remain independent. Accepted,
declined, cancelled or expired invitations cannot be mailed again. A pending
attempt older than the bounded mail timeout may be reclaimed; attempt IDs
prevent an older acknowledgment from overwriting a newer attempt result.
No generic notification daemon, scheduler or outbox platform is introduced.

Sender/from address, SMTP host/port/TLS/auth and public application origin
are fixed deployment configuration. Link uses configured public origin plus
the invitation UUID, never browser Host/Origin or user-provided URL. Require
TLS on external SMTP; plaintext is allowed only for an explicitly configured
loopback Mailpit transport. Credentials remain server-only/private files,
never responses, logs or git. Missing transport disables invitation creation
with an explicit unavailable capability; directory and invitation reads can
remain available. Compose fresh initialization wires loopback Mailpit and
the actual project UI port, without modifying the retained running instance.

## Runtime wiring, schema and boundaries

Use existing GORM/SQL transaction and locking facilities, UUID, Casbin,
ZITADEL clients, Auth.js BFF and current UI components. No custom IAM,
notification platform, event recorder, Saga framework or runner.

Explicit schema initialization adds only current tables/columns to the
existing schema owners. Request/startup paths never run DDL. Runtime owner
permissions gain only SELECT/INSERT/UPDATE on the new own tables, and their
existing catalog checks/grant initialization are extended together. Schema
readiness fails clearly rather than falling back to fake values or legacy
data. Current retained localhost main runtime is untouched; setup of a new
schema/runtime is a separate explicitly authorized action.

All private reads use no-store, fixed upstream routes, manual redirects,
bounded JSON/request bodies (16 KiB for self facts, preferences, recipient
responses and create input; existing 1 MiB membership response transport for
admin directory, summaries and invitation lists), a
15-second whole-request deadline and no mutation retries. BFF checks
expected current user; admin requests bind Effective Organization. Session
or enterprise changes abort/isolate requests and clear private cached data.
Recipient confirmation is self-scoped, independent of enterprise selection;
after acceptance refresh official identity/grants before showing entry into
the enterprise. A saved invitation never grants BFF/application access.

## Verification and delivery

TDD covers target bugs and invitation transitions before implementation.
Relevant tests: provider subject/timestamp validation; preferences save/read
and subject isolation; full totals/real permissions; correct store count;
verified contact mismatch, no implicit grant on creation, existing-member
conflict, creator revocation, same-key conflict, cancel/accept races, expiry,
one dispatch under concurrent accepts, lost ACK/restart/read-confirmation,
wrong-org/project responses and unknown no resend. Persistence tests exercise
real locking/unique constraints in isolated PostgreSQL where available.
Frontend tests cover real fields, blank vs failure, invitation lifecycle,
identity/org switch and late responses; type/lint/build and required CI.
No new acceptance platform or unrelated exhaustive test matrix. Invitation
administration displays the latest 100 records with the separate total;
pending counts cover all records. No historical invitation search is added.

Independent architecture check precedes production edits; findings classified
under AGENTS.md. One Writer, one primary PR. Independent final review checks
actual final diff and affected user flow. User retains product acceptance;
no claim of runtime acceptance, deployed candidate or merge authority.

## Architecture review 2026-09-28

Independent Reviewer: existing `payment_architecture_review` agent, read-only
against the stated main baseline, this design and actual repository owners.
Round 1: IMPLEMENTATION_READY, no BLOCKER; four IMPLEMENTATION_TEST items:
active contact reservation includes accepting UNKNOWN; separate recipient
admission with fresh active creator authorization; synchronize schema/grants
and runtime checks; verify complete counts/real permission and scoped reads.
The first two wording clarifications are now incorporated above; their
behavior, persistence and authorization tests remain required before merge.
Round 2 increment: automatic SMTP notification IMPLEMENTATION_READY,
no BLOCKER. IMPLEMENTATION_TEST: durable attempt status and stale ACK
isolation; DATA/ACK cancellation terminates transport; resend rechecks live
permission and uses fixed origin, verified TLS and private credentials.
Duplicate delivery of the same immutable link is ACCEPTED_RISK; it never
repeats consent/grant, extends expiry or creates another invitation.
