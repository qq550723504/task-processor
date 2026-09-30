# Member directory query v1

Execution: [#575](https://github.com/qq550723504/task-processor/issues/575).
Design Basis: Independent Architecture. Admission: IMPLEMENTATION_READY.
Baseline: main `5a32b153b4a73a2072054ec836e46b4ba193eb8e`.

## Product authority and outcome

Current enterprise readers find a member anywhere in the directory, combine
search/role/state filters, then use the existing detail/management/resource paths.
Authority: #575, #469, [Final UI / IA](../product/final-ui-ia-authority.md), Figma
`tg48P46SSXl6TBy9lZwg63 / 1532:359` (current member management screen, read 2026-09-29).
Its search and two selectors already exist in the current members page but are disabled.
The user explicitly selected **display name and login account** search on 2026-09-29.
Disclose that phone/email matches only when it is the login account. This replaces
the Figma hint's implication of searching independent contact fields for this batch.

Scope: complete directory query, existing three ordinary role options and active/inactive
authorization state, filtered total/paging, typed BFF/client and current controls.
Out of Scope: contact-profile reads, custom roles, IAM/policy changes, invitation or
member mutations, UNKNOWN recovery changes, resources/billing, directory copies/cache,
schema, migration/compatibility, shared runtime changes or new validation platforms.
Invitations retain their separate owner/list; they are not a third authorization state.

## Existing owners and provider evidence

Reuse [#410 membership contract](../engineering/issue410-membership-contract.md) and
[account facts/invitations](account-facts-invitations-v1.md). ZITADEL project
authorizations remain the only member/role/state facts; Home Organization is not
membership. Membership owns the query contract and permission projection; the
ZITADEL adapter owns translating official reads. Existing runtime injection stays.

Checked pinned ZITADEL v4.17.1 primary sources:

- [authorization proto](https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/authorization/v2/authorization.proto):
  state, exact role, display name and preferred login filters; no contact phone/email or OR filter.
- [request translation](https://github.com/zitadel/zitadel/blob/v4.17.1/internal/api/grpc/authorization/v2/query.go):
  role/state map to existing queries; preferred-login filter maps to username query.
- [query implementation](https://github.com/zitadel/zitadel/blob/v4.17.1/internal/query/user_grant.go):
  repeated predicates combine, provider paging and total; permission filtering can remove
  rows. The existing adapter checks bare `user.grant.read` for exact org before/after
  each read; scoped permission suffixes do not establish a complete directory.

Use native role/state predicates. For search, read their complete bounded result set
and match the **returned** displayName OR preferredLoginName in the adapter. This
avoids both AND semantics and the native username/preferred-login mismatch. Do not
query global users, infer Home Organization or join private contacts. Only request-local
matched members are retained; this is not a persisted/cached directory or new fact owner.

## Contract and complete query

`GET /api/v1/account/members` adds optional `q`, `role`, `state` to existing limit/offset.
All parameters are single-valued, unknown keys/invalid encoding/controls rejected;
raw query <= 2048 bytes. Limit 1..100 (default 20), offset 0..10000. Search is valid UTF-8,
trimmed, <=200 UTF-8 bytes before trimming; blank means no search. Matching is literal,
case-insensitive Unicode substring of displayName OR loginName, no regex/wildcards,
accent folding or contact inference. Role is absent or exactly listingkit_viewer,
listingkit_operator or listingkit_admin; membership in a multi-role assignment matches.
State is absent or active/inactive. Role AND state AND search are combined.

Extend the existing PageRequest with a typed query filter; existing List signature and
unfiltered consumers remain unchanged. No fallback or second directory interface.
Service/HTTP/adapter validate the same admitted bounds. Service rejects mismatching
filtered items as invalid provider data, retaining original scope/identity validations.

Without q, official role/state filtering occurs before native ID-ascending paging;
total is the provider's filtered total. With q, enumerate official role/state results
in pages of 100 in one 10-second request budget, at most 100 pages / 10000 rows.
Require full expected page lengths, stable total and unique strictly ascending IDs;
short/missing/repeated/cross-scope/mismatching pages fail, with no partial result.
Apply the literal search, then requested offset/limit to that complete matching set.
Return filtered total, even when requested offset is past its end (empty page, genuine total).
No aggregate is inferred from just one provider page or a browser-loaded page.
The unchanged response membership-v1 fields retain their meaning; no new persistence.

Worst search is 100 official list reads plus existing before/after permission probes,
within the shared 10-second budget; per-page response remains <=1 MiB and at most
100 entries with the existing bounded fields. On deadline or >10000 provider total,
return explicit dependency/invalid-upstream error, not partial or zero results.
This deliberately uses the existing directory bound and timeout; no new scheduler,
indexing service or background retry. Plain role/state requests use one native page.

Reads remain current observations, not a provider transaction/global snapshot.
Known total/ID drift is rejected; same-count concurrent changes are not claimed to
be a snapshot. Offset paging retains its existing live-read semantics. Refresh repeats
the read safely; no write idempotency, reservation, reconciliation or retry owner changes.

## Wiring, authorization and UI

Current members page -> getMembers -> existing same-origin members BFF -> existing
membership HTTP/service -> injected ZITADEL Directory.List -> official authorization API.
No new routes, runtime/cmd/Compose/authz injection or access policy changes.
Existing current identity, expected actor/org, LiveWrite grant resolution and explicit
member.read remain mandatory for all filtered reads. Native bare read permission is
still required; failures never silently reduce results to the credential's own grants.
Manage/assignableRoles/observed versions and protected assignments retain #410 behavior.
Read-only users can filter the three ordinary roles; filter choices are not assignable
roles or authorization grants. All roles includes protected/other existing rows.
No actor/org/project query field is accepted and no credential is exposed.

Keep native accessible input/select controls, existing Console styles and responsive
three-column-to-single-column layout. Submit search explicitly; selectors apply immediately.
Changing an applied filter resets page to zero and clears selected detail. Query keys
include actor/org and applied filters/offset; consume AbortSignal, no previous-data placeholder.
Controls stay mounted during loading, empty or failed reads so users can change/clear filters.
Show loading/error separately from zero matches and distinguish filtered total from full
enterprise statistics (existing MemberStats remains unfiltered). Clear filters restores
the unfiltered page. In-flight filter changes cancel old queries and late results cannot
replace the current query. Enterprise switch remounts/reset controls and scope as before.
Existing detail/manage/resource entries continue to consume canonical IDs and current
capabilities. Busy member mutations disable filter changes; no mutation is introduced.

## Verification and delivery

TDD uses existing Go/httptest and Vitest infrastructure: beyond-first-page name/login
match; role/state/search combinations, multi-role membership, zero and past-end paging;
provider filter payload; complete scan and bounds; incomplete/changed/failed pages,
cross-org/project, cancellation and native permission failure; parameter/BFF/client
validation; UI page reset, loading/error controls, cleared filters, late responses and
enterprise switch; current detail/management/resource and pending-receipt regressions.
Only affected tests/compilation/typecheck/lint and mandatory stable-candidate CI.
Independent architecture review admits this query increment; final delivery review
checks actual diff and evidence. No official provider mutations/shared data are needed.
Real browser/provider product acceptance is NOT_RUN unless separately assigned.
Handoff `/workbench/account/organization/members` using existing login/enterprise selection
and normal startup instructions; user/independent verifier confirms actual use separately.
No merge/deployment/Issue closure authorization in this batch.

### User handoff

Start a separately authorized isolated instance using the existing
[account Compose instructions](../../deployments/docker/account-compose/README.md).
Sign in normally, select an authorized enterprise, and open **企业空间 → 成员与权限**
at `/workbench/account/organization/members`. Enter a name or login account and
press **搜索**; role and authorization-state selectors apply immediately and combine
with the submitted search. Changing criteria returns to page one and closes the
old detail. **清除筛选** restores the full directory. Statistics always cover the full
enterprise; the result count covers the selected criteria. A member's existing detail
and resource/management actions remain governed by current capabilities.

Phone/email match only when included in the login account. Contact-profile search,
custom roles and invitation-state search are not available here. Searches are literal,
case-insensitive and limited to 200 UTF-8 bytes. A native filtered directory larger
than 10,000 rows, incomplete response, failed permission probe or exceeded shared
10-second read budget produces an error; it never becomes a partial result or zero.
No directory is saved locally; existing identity/directory data stays with ZITADEL.
This candidate does not update the retained localhost instance or grant product acceptance.

Legacy decision: N/A. Current owner only; no legacy wrapper, adapter, fallback or migration.

## Independent admission review

The original independent reviewer `audit_summary_review` completed round one on
2026-09-29 against design-only HEAD `ac5414d0b6542eaadfd58c9043cd8d292397af28`,
document blob `d1de223f72e4fd3ca425aa5a05192b45faf93f42` and the current #575 product
decision. BLOCKER=0; explicit IMPLEMENTATION_READY. Complete traversal/bounds,
drift/cancellation/permission probes, Unicode/parameter agreement and UI scope/busy/detail
regressions are IMPLEMENTATION_TEST within this baseline, not a new IAM design.
Issue Ready admission must be recorded before production implementation.
