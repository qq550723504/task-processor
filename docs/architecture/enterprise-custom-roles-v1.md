# Enterprise custom roles v1 — Issue #598

Status: IMPLEMENTATION_READY. Base: `b28e65f6c5deb6f30b440637362aca6696014a88`.
Independent high-risk review of `a09cd146eb11ae9033525756662d5d6c388509a0`
confirmed no current Must BLOCKER. The four implementation checks below remain
required; this is architecture admission, not product acceptance or merge authority.

## Product outcome and authority

An enterprise administrator opens My Account → Enterprise → Members & Permissions,
creates a named role, selects available modules, saves it, and assigns it to an
invitation or member. A member can use only the selected modules through both the
UI and business APIs. This is one Delivery Batch and one main PR.

Authority: [Issue #598](https://github.com/qq550723504/task-processor/issues/598),
the user's implementation authorization, and current Figma file
`tg48P46SSXl6TBy9lZwg63`, page `31:463`:

- members `1532:359` / light `1532:691`;
- roles `1536:473` / light `1536:653`;
- invite `1541:359`, member detail `1541:386`;
- new role `1652:379` / light `1652:394`;
- invitations tab `1676:363`.

The Figma role rule is: only Administrator is a system role; other roles are
enterprise-defined. `listingkit_admin` is the assignable Administrator. Platform
roles and configured protected subjects remain protected and are never options.
The old Viewer/Operator options are retired from this feature; no migration,
mapping, or fallback is introduced. Existing unrecognized assignments are shown
as protected provider facts, never silently rewritten.

Scope includes the two tabs, real aggregate counts, filters and pagination,
invitation list, invitation and member drawers, role creation/configuration,
authorization projection and affected current consumers. Unsupported Figma
modules are visible as unavailable and cannot be selected or grant permissions.
No role deletion, bulk assignment, user/enterprise deletion, account suspension,
new business module, resource allocation, financial semantics, provider calls,
legacy migration, shared-instance deployment, or product acceptance is added.

## Current facts and the new boundary

ZITADEL is the sole identity, enterprise membership, native role-key assignment,
and assignment-state owner. Membership's dedicated PostgreSQL database already
owns command receipts, invitations and member audit events. Casbin v2 already
owns the application's permission vocabulary and fixed system policy.

The new Membership role-definition fact contains a name, module IDs and version
for one `(project, organization, native role key)`. It contains no users or member
directory. Module-to-permission policy is a bounded code catalog in `authz`.
Application permissions are calculated from a verified native assignment and
this role-definition fact; neither browser input nor native role display labels
are policies. No role assignment is copied into the role database.

Native dynamic registration during each Create is deliberately avoided:
ZITADEL v4.17.1 UpdateProjectGrant replaces the complete granted-role list and can
cascade removed roles to user authorizations. Its API has no expected-version
argument. A runtime read/append/write would risk overwriting independent native
administration. Instead, greenfield bootstrap creates **64 enterprise-specific
native role keys**, registers them in the exact project, and grants those keys
when creating that enterprise's initial project grant. A deterministic key uses
an organization digest and slot number; full organization IDs remain in local
scope constraints. Slots have no application permissions until bound to a role.
They are never assigned during bootstrap, recycled, or shared between enterprises.

This is a fixed request/data bound for v1, not a runtime allocator platform.
Exhaustion is a visible conflict. Raising it requires explicit initialization;
there is no runtime native project-grant writer or new privileged credential.
Bootstrap verifies exact native project roles and the active enterprise grant
before installing its slot inventory through the Membership schema owner.
Unexpected populated grants are not replaced to install slots. Existing retained
instances are not mutated by this batch. Fresh isolated instances exercise this
bootstrap path. If slots are absent, the API exposes role creation unavailable
and the UI explains the missing initialization; it never claims success.

Native source: [Project service v4.17.1](https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/project/v2/project_service.proto).
Reuse: [Casbin domain RBAC](https://casbin.apache.org/docs/rbac-with-domains/).

## Contracts, injection and consumers

`authz.RolePolicyReader` reads active role modules for exact project/organization
and supplied native keys. `ListingKitAuthorizer.AuthorizeScoped(ctx, userID,
organizationID, roles, permission)` and its batch permission projection combine
the existing protected/system Casbin policy with enterprise role policies.
Scoped current workbench consumers accept Administrator/protected platform roles
and verified custom keys only. Viewer/Operator aliases do not provide new scoped
workbench access. Unscoped authorization never evaluates custom keys.

The policy reader is explicitly attached to the shared authorizer before serving
requests. The workbench context handler must use that same instance, not its
present separately constructed authorizer. Current request handlers, agent/tool
admission, execution and recovery checks consume their verified enterprise ID.
Each authorization boundary loads current local policy without a process/TTL
policy cache. Existing native grant caching/live-write semantics remain as
documented by #410 and WorkbenchContext; this design does not claim native CAS or
instantaneous native membership revocation beyond those contracts.

`GET /api/v1/account/roles` returns actor/organization scope, catalog, system and
custom roles, version and creation availability. `POST .../roles` creates a role;
`POST .../roles/:role_id/permissions` atomically saves module IDs with an expected
version. Writes require a live MemberManage authorization, an idempotency UUID,
and exact scope. Read uses MemberRead. Raw provider keys are API identifiers;
display names are returned separately. Role management is not grantable to a
custom role. The existing membership-v1 list/command contracts gain scoped role
definitions/options with coordinated strict frontend schemas. Role strings are
validated against a current active definition in the exact enterprise at
admission and immediately before native assignment, including invitation accept.

Workbench context projects actual permission IDs for each enterprise. Frontend
capability checks consume them. It no longer synthesizes `platform_admin` from
StoreDelete, or infers custom permissions from role labels. Tenant-admin-only
operations remain tenant-admin-only. A menu grant cannot impersonate an admin.

## Catalog and safety invariants

Only currently implemented modules are selectable. An explicit catalog maps each
module to existing API permissions, and is the sole mapping for authorization,
role UI and context projection. Initial mapping:

| Module | Existing permission bundle |
| --- | --- |
| AI conversation | ChatRead, ChatUse, TaskRead |
| My Agents | LocalAgentWrite, AgentRead, AgentUse, TaskRead (existing Product Agent consumer; no AgentConfigure) |
| Knowledge | KnowledgeRead, KnowledgeManage |
| Product acquisition | ProductSourcingWrite, LocalAgentWrite, AgentRead, AgentUse, TaskRead |
| Product images | ImageAgentRead, ImageAgentWrite |
| My stores | StoreRead, StoreCreate, StoreUpdate, StoreLifecycle |
| Source accounts | SourceAccountRead, SourceAccountManage |
| Members | OrganizationMemberRead only |
| Plans and benefits | CommercialRead only |

Platform administration, AgentConfigure, MemberManage/role management, StoreDelete,
CommercialPurchase and WalletTopUp are reserved. Module grants do not change
existing ownership, entitlement, budget, resource, paid-provider or Human Review
gates. Unsupported order fulfillment, finance, marketplace and other prototype
modules cannot create permissions. Detailed catalog must bind the exact existing
permission constants and current route/tool consumers before Ready.

Foreign enterprise keys, unknown keys, unused slots, malformed policies, missing
policy reader and policy-store failure cannot grant access. A definition cannot
contain arbitrary permission strings, platform role names or foreign slot keys.
Role names are trimmed, bounded (1–40 Unicode characters), reject controls and
are unique within an enterprise. Module IDs are unique, catalog-only and bounded.
64 roles, 64 supplied native roles, 32 module IDs and a 16 KiB body are hard bounds.
System Administrator is readonly in the role editor. Protected member actions
retain the current server-enforced restrictions. No feature deletes identities,
enterprises, durable business assets, plans, balances or entitlements.

## Persistence, idempotency and failure

Membership adds three explicitly initialized tables: slot inventory, role
definitions and role-mutation receipts. Keys and foreign keys contain project and
organization. Runtime receives SELECT/INSERT/UPDATE only where necessary, no
DDL/DELETE or other-owner privileges. Startup verifies the schema and effective
privileges; request handling never installs schema.

Create and Save each use one database transaction, including mutation receipt.
Command identity is `(project, organization, actor, operation UUID)`. A canonical
fingerprint contains action, role ID/name, selected modules and expected version.
Same key/same fingerprint returns the recorded result; different payload conflicts.
Create exclusively claims an unused native slot in the transaction. Concurrent
creation cannot reuse a slot or duplicate a name. Save uses expected-version CAS.
Response loss/restart/retry reads the same committed receipt, with no external
effect. Transaction failure leaves neither definition nor receipt. Permissions
removed by Save deny subsequent authorization reads; issued browser projections
are display-only. In-flight operations retain existing final admission checks.

The existing member/invitation durable command protocol (#410/#551/#575) remains
the only native assignment writer. UNKNOWN, explicit recovery, no post-dispatch
resend, scope/key/fingerprint, protected targets and live authorization checks
remain unchanged. Selecting a custom role changes validation and key payload,
not the member protocol or its ownership. No new outbox, Saga, runner or reconciler.

## Legacy decision

Legacy decision: RETIRE (fixed Viewer/Operator UI options and role-name-derived
current workbench capability projection). Reusable behavior: existing Casbin
system policy, verified native grants, Membership receipts/invitations, account
shell, existing dialogs/drawers and bounded query components. Current owner:
Membership definitions + authz catalog + ZITADEL assignments. Cutover: coordinated
current backend/BFF/frontend update on this branch. No fallback, dual role policy,
old-data conversion or legacy service dependency is added.

## Verification and delivery

TDD targets scoped isolation/unknown/unused keys, reserved permissions, policy
removal, schema/privileges, create/save transaction and idempotency conflicts,
concurrent slot claim and stale save, live assignment validation and invitation
recipient scope. Existing command UNKNOWN/protected-target tests are reused.
Provider tests verify bootstrap never rewrites a populated project grant and
native custom role assignment works in a fresh disposable local instance.
Affected execution/recovery tests prove a removed module cannot be used there.
Frontend tests cover typed custom role contracts, scope switch/cancellation and
retained durable recovery. Typecheck/build and relevant existing checks run once
at a stable candidate, with exact-head CI recorded in the main PR.

Visual review compares requested Figma nodes at matching viewport, both themes,
and responsive behavior. No prototype people/counts/success states are seeded
into production. Independent high-risk architecture review must classify
findings and explicitly mark IMPLEMENTATION_READY before production edits.
One final independent delivery check follows the complete path. Deliver a retained
isolated runtime or normal startup instructions and known limits for user trial;
developer checks/CI are not product acceptance. No merge, deploy, Issue closure
or shared/real account mutations are authorized.

### Concrete initialization and permission mapping

`internal/zitadelprovision` extends the existing local project/enterprise bootstrap
with an opt-in role-slot initialization, using the existing bootstrap credential
only. Existing `ensureProjectGrant` is not used to add slots to populated grants:
the slot path reads the exact grant, creates a new grant when absent, or verifies
an exact already-initialized grant. A mismatched existing grant returns an error
without UpdateProjectGrant. All native role/grant reads are fully paginated and
bounded; the existing default role search must not truncate the slot inventory.
The home project's owner organization also receives slots and uses project roles
directly, without a self project grant. Bootstrap emits non-secret exact-scope
slot inventory after native read-back. `organization-membership-schema-init`
accepts this explicit inventory file, validates deterministic keys and scope, and
inserts slots idempotently in its schema-owner transaction. It cannot install an
alternate key, rebind a populated slot or import users/assignments. Compose's
existing initialization owner wires this file only for a fresh instance.
Every slot-enabled bootstrap entry and repeated initialization uses the same exact
Administrator + enterprise slot set and refuses an existing mismatched grant;
the previous fixed-role helper cannot run first and remove the slots.

Product acquisition does not grant ListingKitAdminRead/Write. Those constants
currently collide with unrelated optional SHEIN record and Commercial overview
routes. The current Product Agent execution/routes, Product Review read/write
routes/service and product catalog/asset/readiness inspect tool definitions use
the existing LocalAgentWrite permission instead. Each retains its independent
enterprise, owner, TenantAdmin, Human Review, budget and provider gates. Agent
configuration's domain-availability probe uses LocalAgentWrite; writes still
require AgentConfigure. Commercial overview uses CommercialRead in both route
and service. No SHEIN record authorization or financial semantics are changed.

The HTTP module attaches the Membership role-policy reader to the one shared
authorizer, including its Workbench context handler. Product Agent tool/model
admission and billing recovery receive that same instance. Standalone Image
worker assembly explicitly opens the same Membership policy owner through an
opt-in dedicated read-only `organization_role_policy_reader` pool (SELECT on
slots/definitions only, no receipts/users or mutation privileges), injects the
reader into its own authorizer and closes the pool with its existing dependency
lifecycle. The existing image execution authorizer always evaluates the exact
native member/project/enterprise and current scoped policy. Missing reader or
failed reads deny a custom role; no worker falls back to role-name heuristics.
Initialization creates this bounded read role using the existing schema owner;
credentials remain server-only. Configured platform-admin subjects are also
protected in member eligibility and immediately before member dispatch, using
the existing platform authorization check; no new last-admin protocol is added.

Role mutation writes revalidate live actor identity after bounded body parsing,
immediately before beginning the policy transaction. The DB serializes each
enterprise's create/save through a transaction-scoped advisory lock and checks
the receipt before applying expected-version CAS. The local policy read has no
network side effects. The existing provider assignment writer remains a separate
protocol with the approved #410 no-provider-CAS limitation; slots eliminate any
new runtime provider registration/replacement protocol.
