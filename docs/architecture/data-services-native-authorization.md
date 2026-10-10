# Data Services native authorization — #619 / PR #631

- Product Decision: `PD-DATA-SERVICES-NATIVE-IAM-2026-10-10`.
- Design Basis: **Independent Architecture**, limited to the discovered native
  authorization composition blocker. Other boundaries reuse the frozen
  [Data Services v1 design](data-services-v1.md).
- Admission: **DESIGNING — independent review pending**. Do not change the
  production authorizer to this contract before IMPLEMENTATION_READY.
- Design baseline: `7e0a5e31a65490dd36784d9a1990f447f9b536a7`.

## Product authority and result

The user explicitly selected on 2026-10-10: “沿用当前正式授权，本轮不新增本地企业业务停用功能（推荐）”. This decision supersedes the feature design's assumption that a mandatory local business-suspension owner is available. The normal Workbench composition supplies no such owner. Its existing `workbenchcontext.Resolver` contract explicitly treats nil as no additional local suspension policy.

The result is normal-login Data Market/API Management and the original actor-private DataKey/worker paths using current formal authorization. No new UI, names or interaction design is introduced; current [final UI/IA authority](../product/final-ui-ia-authority.md), referenced Data Services Figma frames and existing pages remain applicable.

Must: exact original subject/project/org/member authorization and active grant; active ZITADEL user; current native RoleModules/Casbin permissions; original actor-private key/job/results and frozen Resource funding; dependency failure denies. Out of Scope: a local organization business-suspension feature, status owner/table/API, new identity or authorization system, and provider/data operations outside the approved isolated trial. No new Accepted Risk or reduced requirement is inferred beyond the user's explicit phase decision.

## Root cause, ownership and call path

`buildDefaultWorkbenchContextModule` supplies nil to the formal Workbench resolver. Repository production code has no `OrganizationBusinessStatusChecker` implementation. `dataserviceauth.NewAuthorizer` currently requires a nonnil checker. Therefore the normal Data Services application cannot construct; injecting an always-allow adapter would hide the mismatched requirement.

Identity/member/role facts remain ZITADEL and the current native policy owner. `OrganizationBusinessStatusChecker` remains an optional deny-only port owned by WorkbenchContext; this delivery creates no implementation or local status facts.

Contract → implementation → injection → consumer:

1. Consume `workbenchcontext.Resolver.BusinessStatusChecker()` as its current optional port.
2. `dataserviceauth.NewAuthorizer` still requires exact IAM reader, active-user reader, server-only service token callback, bounded project ID and native policy. A nil status port represents the authorized absence of the extra local policy.
3. `current()` always verifies the original exact grant, active user and caller deadline. If a checker is explicitly supplied, call it and deny suspension or dependency failure. A nil checker skips only that extra local policy; it cannot skip the exact grant, active-user, current permission or deadline checks.
4. Native application supplies the formal resolver's original checker unchanged, including nil. It never substitutes a fake checker. Console, Specialist, DataKey, worker and Resource proof consumers keep the same authority object and original permission/funding contracts.

When the port is nil, success requires the same formal IAM and current permissions as above. Missing IAM/policy/token or unavailable exact/user reads still fails closed. With a supplied checker, errors, suspension or cancellation still deny. API keys cannot survive inactive users, revoked/recreated grants, revoked modules, expiry/revocation or cross-actor/org access. Future local suspension capability requires its own current product/design authority; this port does not authorize building one.

## Unchanged boundaries

No schema, durable fact owner, public API endpoint, state machine, transaction, compensation, retry/UNKNOWN or recovery change. Product publication/Collection/quota/key locking and Resource reservation/settlement proof stay frozen. Temporal remains execution infrastructure. No new external side effect. Service credentials stay server-only; normal verified login and original tenant/member scope are retained.

Legacy decision: N/A; no retired owner, compatibility adapter, fallback, migration, dual read/write or second fact source is used.

## Bounded implementation and verification

After independent admission, first add tests proving nil local policy permits current valid IAM but still denies inactive/recreated grants, inactive users, revoked native permissions, and unavailable IAM/token/user/policy dependencies. Keep/check the supplied-checker suspension/error denial path and caller cancellation. Then change only constructor validation, the optional status branch and native missing-checker diagnostic/test that reflected the superseded assumption.

Reuse existing DataKey revocation/private-result, worker authorization and Resource proof tests. Use current normal Compose plus the new-empty Data Services profile for actual startup/login/API read-only verification. No real Amazon/paid calls or synthetic fixtures will be presented as business acceptance. A retained runnable local handoff follows successful prerequisites; old instance data remains untouched.

## Independent review checkpoint

Review only this actual authorization boundary and affected sibling consumers against the approved product decision, unchanged Must and current diff. Classify findings before changes; a real wrong-authorization/core-happy-path defect is BLOCKER, hypothetical local suspension functionality is out of this phase. Maximum two normal architecture review rounds; no new governance/validation platform. Record independent reviewed design SHA and explicit IMPLEMENTATION_READY before production auth edits.
