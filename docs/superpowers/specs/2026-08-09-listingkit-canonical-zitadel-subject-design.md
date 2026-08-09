# ListingKit canonical ZITADEL subject design

## Status

- Status: approved design, pending written-spec review
- Date: 2026-08-09
- Scope: ListingKit identity ownership and owner-scope configuration convergence

## Goal

Make the ZITADEL subject (`sub`) the only user identifier used for ListingKit
resource ownership. Remove the misleading owner-scope configuration switch so
that user-level isolation is a fixed security rule rather than an optional
runtime feature.

## Context

ListingKit already treats the Go API's ZITADEL introspection middleware as the
authoritative identity boundary. It stores a verified tenant, user, and role
set in `AuthenticatedIdentity`, and protected handlers derive their request
context from that value.

Two identity ambiguities remain:

1. The middleware and Auth.js session prefer an optional `user_id` claim and
   fall back to `sub` or username. The value written to owner-scoped database
   columns can therefore depend on token shape.
2. Configuration still exposes `listingkit.ownerScopeRequired`, while the
   ListingKit bootstrap unconditionally enables owner scoping. Operators can
   reasonably but incorrectly believe that the YAML value controls production
   behavior.

The legacy user migration is complete. This design intentionally introduces no
runtime compatibility path for legacy Yudao user IDs.

## Decisions

### Canonical user identifier

- `AuthenticatedIdentity.UserID` is always the normalized ZITADEL `sub`.
- A protected ListingKit token without a non-empty `sub` fails closed with HTTP
  `403` and error code `zitadel_user_missing`.
- The optional `user_id` claim and username may remain available as display or
  diagnostic attributes, but neither may determine resource ownership,
  authorization subjects, audit actors, or persisted task identity.
- The Next.js Auth.js session uses the same rule: `identity.userId` is derived
  from `sub` only.
- Database columns remain named `user_id`; their documented meaning becomes
  "canonical ZITADEL subject". No identity mapping table is added.

### Fixed owner scope

- Remove `listingkit.ownerScopeRequired` from the Go configuration type,
  defaults, loader bindings, YAML files, environment bindings, documentation,
  and tests.
- ListingKit and ListingAdmin owner scoping remain enabled as an invariant of
  module startup.
- Production code must not expose a switch that disables owner filtering.
- Test-only helpers may still install explicit scope state when a focused unit
  test needs to exercise repository behavior. They must not be reachable from
  production configuration.

### Trusted actor identity

- Security-sensitive audit code reads the actor from
  `AuthenticatedIdentityFromContext`.
- Compatibility headers written after token introspection remain transport
  hints for legacy internal code, but audit and authorization code must not
  treat those headers as authoritative.
- Member-invitation audit records use the verified subject from context.

## Request flow

1. Auth.js completes ZITADEL OIDC login and stores the access token in its JWT
   session.
2. Auth.js extracts the resource-owner ID as `tenantId`, `sub` as `userId`, and
   the project roles as `roles`.
3. The Next.js ListingKit proxy forwards the bearer token. Any forwarded
   identity headers are non-authoritative hints derived only from the verified
   session.
4. The Go API introspects the bearer token and requires both the resource-owner
   ID and `sub`.
5. The middleware removes caller-supplied identity headers, installs
   `AuthenticatedIdentity{TenantID: resourceOwnerID, UserID: sub, Roles: roles}`
   in request context, and rewrites compatibility headers from that identity.
6. Handlers, asynchronous detached contexts, repositories, AI identity, and
   audit records propagate the canonical subject without fallback.
7. Owner-scoped repositories filter ordinary viewers and operators by tenant
   plus subject. Tenant administrators retain tenant-wide visibility, and
   platform administrators retain their separately authorized cross-tenant
   operations.

## Identity preflight

A new read-only preflight validates that the deployment is safe for a clean
cutover.

### Inputs

- The configured application database.
- The existing dedicated read-only ZITADEL directory credential.
- The configured ZITADEL issuer and ListingKit project.

### Behavior

1. Inventory every production table that participates in ListingKit or
   ListingAdmin owner scoping.
2. Read distinct non-empty `tenant_id` and `user_id` ownership pairs without
   modifying rows.
3. Query the official ZITADEL Users API for the corresponding organizations and
   build the set of current ZITADEL user subjects.
4. Compare persisted owner IDs with those subjects.
5. Exit successfully only when every persisted owner ID is a known subject for
   the corresponding tenant.

The report includes the table, tenant, affected-row count, and a masked owner
identifier. It must not print email addresses, names, bearer tokens, cookies,
client secrets, or complete identifiers.

An unknown owner is a release blocker. The preflight never rewrites data and
does not introduce a fallback mapping. Because the legacy migration is already
complete, operators investigate and correct the underlying data or directory
state before retrying deployment.

The initial table inventory is maintained next to the preflight and has a
repository-structure test that fails when a newly introduced owner-scoped model
is not accounted for.

## Error semantics

| Condition | Response or result | Side effect |
| --- | --- | --- |
| Token has no ZITADEL resource owner | `403 zitadel_tenant_missing` | Handler is not called. |
| Token has no ZITADEL `sub` | `403 zitadel_user_missing` | Handler is not called. |
| Caller supplies forged tenant, user, or role headers | Request proceeds under introspected identity | Forged values are removed. |
| Persisted owner is not a current subject in the same tenant | Preflight exits non-zero | No data is changed and deployment stops. |
| ZITADEL directory cannot be queried | Preflight exits non-zero with a sanitized operational error | No data is changed and deployment stops. |
| Invitation audit has no authenticated identity | Request fails closed instead of recording a header-derived actor | No invitation mutation is attempted after the missing-identity failure. |

## Rollout

1. Verify that ZITADEL no longer injects the legacy business `user_id` claim,
   or that any remaining claim is exactly equal to `sub`. This protects an
   emergency rollback from restoring two ownership meanings.
2. Run the identity preflight against the target environment.
3. Deploy the Go API containing the canonical-subject rule.
4. Deploy the matching ListingKit UI/Auth.js version.
5. Verify a viewer, operator, tenant administrator, and platform administrator
   using real tokens, including owner-scoped list/detail operations and one
   audited platform action.

API and UI are one coordinated release. A partial rollout must not be treated as
acceptance even though the Go API remains the final authority.

## Rollback

A code rollback is safe only after the rollout prerequisite has proved that the
legacy `user_id` claim is absent or identical to `sub`. If that prerequisite is
not satisfied, rollback can restore the old claim preference and split resource
ownership again; deployment must therefore stop before cutover.

The preflight is read-only, so it requires no data rollback.

## Verification

### Go

- Middleware uses `sub` even when `user_id` and username differ.
- Missing `sub` returns `403 zitadel_user_missing` before the handler runs.
- Forged identity headers cannot influence the context or rewritten headers.
- Route authorization uses the canonical subject and introspected roles.
- Detached request contexts and AI identity preserve the subject.
- ListingKit and ListingAdmin owner scope cannot be disabled through config.
- Viewer and operator queries are tenant-and-subject scoped.
- Tenant administrators retain tenant-wide access.
- Member-invitation audit actor comes from authenticated context.
- Preflight covers success, unknown owner, cross-tenant owner mismatch,
  directory failure, and output redaction.

### TypeScript

- Auth.js session identity uses `sub` even when `user_id` differs.
- Missing `sub` does not create an apparently valid ListingKit identity.
- Proxy headers can only be derived from the session subject.
- Page authorization continues to use ZITADEL project roles.

### Configuration and repository boundaries

- Tests prove `ownerScopeRequired` and its environment variable are absent.
- YAML and deployment examples contain no disabled owner-scope setting.
- A repository guard keeps the preflight's owner-scoped table inventory current.
- Relevant Go package tests, frontend Vitest tests, type checking, linting, and
  configuration rendering complete before release.

## Non-goals

- No legacy Yudao user-ID fallback or mapping table.
- No automatic owner-ID rewrite.
- No change to the ZITADEL organization-to-legacy-tenant bridge required by
  business tables that still use numeric tenant IDs.
- No member list, role editing, suspension, removal, or reinvitation UI in this
  phase.
- No change to subscription, entitlement, store-ownership, or asset-isolation
  semantics.

## Follow-up phase

The next independently designed phase adds the application member lifecycle:
member listing, role changes, disable/remove operations, reinvitation, and
ZITADEL state synchronization. It will continue to use ZITADEL as the identity
and membership authority and will not create an application password store.
