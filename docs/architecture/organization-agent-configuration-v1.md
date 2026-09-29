# Organization Agent Configuration V1 — Slice D

> Status: **DRAFT / NOT_IMPLEMENTATION_READY**
>
> Design Basis: **Independent Architecture**, a bounded increment over #556.
> Issue: #570. Product parent: #555. AI Workbench parent: #298.
> Inspected code: `main @ 204aef668ac4f8a4361da229895417ebb78a1fdc` (2026-09-29).
>
> This is a review candidate, not an approved implementation or release gate.
> D0 changes documentation only. Independent Architecture Review, applicable CI,
> explicit IMPLEMENTATION_READY admission and architecture merge precede production work.
> Merge, Issue closure, deployment, real enterprise data and paid providers are not authorized.

## 1. Product outcome and authority

An administrator enables the existing Product title Agent for the current enterprise,
maintains a versioned default template, and members explicitly confirm the effective
configuration before using the existing title → Knowledge → Human Review → Apply path.
“我的智能体” means **当前企业的智能体**, not private per-user agents.

```text
智能体市场 → 企业启用关系 → 我的智能体
                              ├─ 默认模板（精确版本）
                              ├─ 当前能力与不可用原因
                              └─ 最近任务（原有授权范围内）
执行确认 → exact configuration snapshot → existing Product Agent Start
         → existing Knowledge bundle / governed model → Human Review → explicit Apply
```

Applicable authorities:

- [Final UI / IA](../product/final-ui-ia-authority.md): Figma file
  `tg48P46SSXl6TBy9lZwg63`, page `31:463`, current non-archived product authority.
- [Agent + Knowledge V1](./agent-knowledge-context-v1.md), #555/#556 §2:
  enterprise scope, opt-in Knowledge, configuration != permission, exact run references.
- [Product Agent runtime](./product-agent-runtime-contract.md): run/checkpoint/UNKNOWN owner.
- [Project boundaries](./project-boundaries.md),
  [HTTP assembly boundaries](./httpapi-assembly-boundaries.md),
  [greenfield baseline](../product/greenfield-no-legacy-migration.md),
  [dispatch rules](../engineering/issue-driven-development.md), and [AGENTS.md](../../AGENTS.md).

Figma mapping inherited from the admitted #555/#556 evidence: market `428:323`,
我的智能体 `4703:429`, capability configuration `4703:892`, template `4714:429`,
execution confirmation `4722:429`, Review `4711:429`. These references establish product
semantics, not proof of newly inspected screenshots or implemented screens. This D0 does
not edit Figma. Implementation must check the then-current non-archived authority.

### 1.1 Scope and non-goals

V1 has one executable catalog entry: `product.title.agent / v1.0.0`. Enablement, templates,
capability display and run consumption must form one usable path, not a fake-save settings UI.
A template supports only existing title-consumer parameters: target platform and optional
KnowledgeBase selection. It does not introduce editable prompts or arbitrary parameter maps.

Out of scope: full Chat/BusinessTask/Project/Report; custom AgentDefinition or Tool allowlist;
Multi-Agent orchestration; BYOK/custom endpoints/credential rotation; arbitrary model routing;
image execution; platform remote writes; embeddings/vector retrieval; new IAM, Review,
billing, cancellation, scheduler, Saga or provider-recovery platforms.

Only technical contracts needed for this scope are proposed below. No new accepted risk,
production acceptance, administrator override or relaxation of #556 is implied.

## 2. Verified baseline and reuse map

| Actual source at the inspected SHA | Existing fact / implication |
| --- | --- |
| `internal/app/httpapi/product_agent_application.go` | Constructs `product.title.agent/v1.0.0`, four bounded read/compute tools, governed text model, current Knowledge and Review services. |
| `internal/app/httpapi/product_agent_routes.go` | Start/Read/Resume/Review use verified identity, LiveWrite Organization resolution and **listingkit.admin.write**. Not merely `local_agent.write`. |
| `internal/app/httpapi/product_agent_knowledge.go` | Optional `knowledgeSelection.knowledgeBaseId`; absent means no Knowledge. Pre-Start materialization, claimed replay and complete prompt preflight already exist. |
| `internal/agent/contracts.go` | Request is a comparable value; Scope, Binding, versions, limits and opaque Knowledge ref are persisted. State limit is 2 MiB. |
| `internal/integration/agent/eino/runtime.go` | Fingerprints Scope + complete Request + code Definition; compares Request with `!=`; Store.Claim determines acquired work versus receipt replay. |
| `internal/integration/persistence/agent/store.go` | PostgreSQL Claim/Commit, exact scope/key, CAS and one stored checkpoint. Constructors do not install schema. |
| `internal/commercetool` | Code-owned AgentDefinition and exact ToolRef allowlist, not enterprise-editable settings. |
| `cmd/current-application/main.go` | ProductAgentDependencies.RunDB is supplied from current ProductAgentDB; assembly owns pools. |
| `web/listingkit-ui/src/lib/workbench/console-navigation.ts` | Existing market/mine/custom navigation paths are product entry locations, not activation facts. |

This increment does not replace current identity, domain permissions, Product evidence,
Knowledge permit, AI invocation/point settlement, or Review/Apply. Source inspection is not
execution evidence. Existing #559 browser evidence remains bound to its original code/image.

## 3. Ownership and dependency direction

Proposed new bounded domain: **`internal/agentconfig`**.

It owns OrganizationAgent, AgentTemplate (including revisions), immutable start-configuration
receipts and config-command audit/idempotency. The latter two are internal records, not new
product objects, queues or execution lifecycles. AgentCapabilityProjection remains a read
model. No mutable universal AgentConfig is introduced.

| Contract → implementation → injection → consumer | Responsibility |
| --- | --- |
| `agentconfig.Repository` / pure configuration policy → `internal/integration/persistence/agentconfig` → current application assembly → configuration service | Enterprise activation, exact defaults, template versions, command transactions. |
| `agentconfig.CatalogReader` → code-owned Product title registration → current Product Agent assembly → market/mine and Start resolver | One approved definition source, metadata, parameter-schema version and capability descriptors. |
| `agentconfig.StartConfigResolver` → same owner/service and store → existing Product Agent application → pre-Start resolution | Immutable effective configuration, no provider call or run creation. |
| `agent.Store` → transaction-aware existing run store plus bounded config-claim adapter → existing Eino Config.Store → Start/Resume | Atomic activation admission and existing Claim, never a second runtime. |
| Narrow readiness/Knowledge-metadata ports → adapters to existing owners → configuration read service → market/mine/confirmation | Safe read projection, never permission or paid probing. |
| Existing run read contract + bounded recent-run query → current run persistence owner → current Product Agent application → mine detail | Original actor/Organization/domain authorization, not enterprise-wide access. |
| Configuration service → feature-local `internal/agentconfig/httpapi` handlers → descriptor registration in current app → existing Console/BFF | Transport validation/projection; no business logic added to assembly. |

New names above are proposed placement, not existing implementation claims. Domain core
imports neither GORM/SQL nor Gin, provider clients, `internal/app`, Knowledge implementation
or legacy roots. Adapters import the narrow owner contracts. Generic `internal/agent` and
Eino do not import `internal/agentconfig`. The claim adapter composes existing Store behavior;
it does not duplicate checkpoint, run uniqueness, phase, deadline, retry or UNKNOWN logic.

The code catalog must be derived from the same registration that builds the runtime
AgentDefinition. A separate hand-maintained database or JSON copy of AllowedTools is forbidden.
Each entry has agent ID, exact definition version, safe display metadata, supported parameter
schema and required/optional/not-supported capability descriptors. Only supported registered
versions are executable. Reading an old receipt is not contingent on code still executing it.

## 4. Persistence and local transaction boundary

### 4.1 Database and privileges

Use a dedicated PostgreSQL schema **`agent_configuration` in the existing ProductAgentDB /
ProductAgentDependencies.RunDB logical database**. It must be the same database as
`product_agent_runs`, not merely the same PostgreSQL server. No new logical database,
message broker or distributed transaction is needed for activation versus Claim.

Assembly injects this exact existing serving pool into both stores. The bounded admission
adapter uses one outer SQL transaction and transaction-bound existing run-store operations.
Different databases/pools that cannot share that transaction fail construction; do not fall
back to a check-then-call sequence. This is a local configuration/run boundary only:
Knowledge, Product, Review and monetary ledgers retain their existing databases/owners.

The explicit empty-install schema path creates `agent_configuration`, owned by a non-login
`agent_configuration_owner` role. An operator/schema connection installs it and grants the
existing application serving role only the needed schema/table/sequence privileges. The
serving connection is not owner and has no CREATE/ALTER/DROP or business-record DELETE.
Revisions and start snapshots are insert/read-only. Constructor/read/enable/Start never
AutoMigrate or bootstrap roles. Backup/restore includes this schema with its run database;
restoring an inconsistent pair fails closed rather than regenerating configuration history.

### 4.2 Records, keys and invariants

All Organization and actor IDs are exact verified opaque IDs, nonempty UTF-8 strings ≤128
bytes; never browser authority or legacy numeric mappings. Generated IDs are canonical UUIDs.
Versions are positive PostgreSQL bigint, exposed as canonical decimal **strings**, not JS
numbers. Names are trimmed UTF-8, 1–120 characters and ≤512 bytes, without control characters.
Timestamps are UTC timestamptz. No hard-delete API exists in V1.

| Table in `agent_configuration` | Columns / constraints |
| --- | --- |
| `organization_agents` | PK `(organization_id, agent_id)`; activation `ENABLED/DISABLED`; `activation_epoch` bigint; `revision` bigint; nullable paired `default_template_id/default_template_revision`; created/updated actor/time. |
| `templates` | PK `(organization_id, agent_id, template_id)`; lifecycle `ACTIVE/ARCHIVED`; `head_revision`; independent metadata `revision`; created/updated actor/time. FK to same OrganizationAgent. |
| `template_revisions` | PK `(organization_id, agent_id, template_id, version)`; FK to exact template; immutable `name`, `schema_version=title-config-v1`, `target_platform`, nullable `default_knowledge_base_id`, creator/time. No arbitrary JSON parameters. |
| `start_snapshots` | UUID PK; unique `(organization_id, actor_id, context_kind, context_id, request_key)` matching existing run identity; agent ID/version, command fingerprint, immutable bounded effective-config payload/digest, OrganizationAgent revision/activation epoch, nullable exact template ref, created_at. |
| `commands` | Unique `(organization_id, actor_id, idempotency_key)`; command ID, operation/target, fingerprint, before/after revision references, bounded mutation receipt, committed_at. This is the config audit record and replay owner. |

`target_platform` is exactly `shein | temu | amazon`, selecting existing inventory dimensions,
not publishing permission. A template KnowledgeBase field is either absent/null at storage
or one canonical UUID. Revision content is immutable; rename also appends a revision.

Use full Organization/agent-qualified foreign keys for revisions, snapshot template refs,
and default-template refs. Nullable default fields have an explicit both-null-or-both-set
CHECK; use a composite FK with ordinary nullable semantics, not a MATCH FULL FK containing
non-null Organization columns. Cross-domain KnowledgeBase IDs have no cross-database FK:
validate via the Knowledge owner and recheck at actual use. No source names/text, raw prompts,
credentials, Tool allowlists, permissions, cost results or AgentRun phases are copied here.

Required indexes: templates `(organization_id,agent_id,lifecycle,created_at,template_id)`;
revisions on their PK; snapshots `(organization_id,actor_id,agent_id,created_at,id)`;
commands `(organization_id,committed_at,command_id)` in addition to uniqueness. Snapshots
contain at most 16 KiB serialized metadata; receipts at most 2 KiB. Full Request/checkpoint
continues to fit the existing 2 MiB bound. JSON payloads have one typed, versioned serializer
and size/shape checks; they are not a user-extensible settings bag.

## 5. Lifecycle, defaults and command consistency

### 5.1 Activation versus availability

No row means **未启用**, not ENABLED and not an instruction to create a default row on read.
First explicit enable creates ENABLED/revision 1/epoch 1. Actual enable↔disable transitions
increment revision and activation_epoch. A no-op desired-state command records a receipt but
does not increment either. Default-pointer changes increment revision only.

Read projection is deterministic:

| Persistent / readiness facts | Display |
| --- | --- |
| No OrganizationAgent | 未启用 in market; absent from mine. |
| DISABLED | 已停用, regardless of runtime readiness. |
| ENABLED + required supported capabilities ready | 已启用. |
| ENABLED + required supported configuration missing and repairable | 待配置. |
| ENABLED + required definition/dependency/rollout unavailable | 暂不可用; safe reason. |

Required unavailable takes precedence over repairable missing configuration. Optional
Knowledge/default-template warnings are separate from Agent readiness. The current title
Agent works without Knowledge, so a missing/disabled default KnowledgeBase cannot make every
no-Knowledge invocation unavailable. Actor-specific `canUse/canConfigure` is also separate:
one viewer's lack of permission is not a persisted enterprise outage.

Enable is allowed for a registered supported agent even if temporarily unavailable; the
result honestly shows readiness. Unknown/unregistered agents cannot be enabled. A removed
registered definition may remain in mine as a safe unavailable tombstone, without synthetic
runtime data. Disabling remains possible when the runtime/provider is down.

### 5.2 Template and default behavior

Create template → ACTIVE with immutable revision 1. Update with expected metadata revision
appends vN+1 and advances head/metadata revision atomically. Old revisions remain readable
under current authorization. New starts may explicitly select a retained revision of an
ACTIVE template if its parameter schema and exact agent definition remain supported; they
never silently upgrade it. Archive is irreversible in V1; make a new template to reuse values.

The enterprise default is an **exact `(template_id, template_revision)` pointer**. Updating
a template head does not move that pointer. “设为默认” is a separate explicit CAS command;
UI can offer it after Save, but must not claim both writes succeeded if only one committed.
Clearing the default is explicit. Archiving a currently defaulted template returns
`TEMPLATE_IS_DEFAULT`; clear or replace the pointer first. Both operations lock the same
OrganizationAgent before the template, so concurrent default assignment/archive cannot leave
an archived default. No default pointer exists without its exact same-org/agent revision.

Knowledge selection is stored only by the template revision, never independently duplicated
on OrganizationAgent. Saving a nonempty default requires current Knowledge read authorization
and an eligible same-Organization base; it does not require Knowledge manage or materialize
content. Later revocation/disabling yields a protected warning, not automatic fallback.
The user must explicitly choose another base or no Knowledge before execution.

### 5.3 Idempotency, CAS and audit

Every configuration mutation requires one canonical UUID Idempotency-Key. Fingerprints cover
versioned normalized operation, target, typed payload and expected revision. Same org/actor/key
with different input is 409 `IDEMPOTENCY_CONFLICT`, even on a different configuration route.

For a committed matching command, reauthorize the current caller, then return the original
bounded **mutation receipt** before current CAS/lifecycle checks. Do not reapply it or present
its old revision as the current resource. Client subsequently GETs the current resource.
Another actor cannot adopt a receipt by guessing the key.

For new work, transaction order is command-key claim → OrganizationAgent → template(s) →
mutation + final receipt. The command-key claim is an uncommitted insert/unique constraint,
not a durable pending job. A concurrent duplicate waits for commit/rollback and then compares
the fingerprint; no uniqueness error becomes permission to perform the command twice.
Unknown outcome retries the same key. Any failure rolls back object, revision and receipt
jointly. No provider, Knowledge content read, HTTP call or long-running work occurs inside
these transactions. External identity and Knowledge metadata checks happen beforehand;
local constraints and eligibility are checked under the owner locks.

Audit stores actor/Organization, operation, target, request identity and before/after refs,
not sensitive configuration contents. No-op commands are distinguishable. Config audit is
not automatically wired into unrelated Account totals; that requires an explicit read adapter,
not another event copy. Do not create an audit platform for D.

## 6. Permissions and disclosure

Use current ZITADEL/Workbench resolution and Casbin; no role lists in OrganizationAgent.
Add the following named decisions to the existing policy owner:

| Permission | Meaning | Default scoped roles |
| --- | --- | --- |
| `workbench.agent.read` | Read safe catalog, enterprise activation/template metadata. Not Product run content or Knowledge text. | listingkit_viewer, listingkit_operator, listingkit_admin, platform_admin |
| `workbench.agent.use` | Additional gate for new Start/Resume and authorized Product run interactions. | listingkit_operator, listingkit_admin, platform_admin |
| `workbench.agent.configure` | Enable/disable, create/update/archive templates, set/clear default. | listingkit_admin, platform_admin |

These are independent decisions, not implied privilege inheritance. Config GETs also accept
an explicit configure grant so a custom configure-only role can maintain configuration;
use checks do not infer configure, and configure does not infer use. Standard mappings grant
the intended combination explicitly. Platform admins still need the effective Organization
grant. Custom Casbin grants continue through the existing owner.

Actual Product Start/Resume = current fresh Organization/member access + agent.use + existing
`listingkit.admin.write` + exact acquisition/Product binding + enabled admission + existing
rollout/limits + Knowledge read when selected + current AI/point governance. Missing any one
fails closed. Keep existing Product read/Review restrictions, including actor ownership;
agent.read is never enough to read another member's run or product.

Configuration mutations and execution/receipt reads use LiveWrite resolution, no cached
actor authority. Ordinary safe catalog/config reads use the existing CachedRead policy and
its existing bounded revocation rules; no new cache promises. Return no-store. Knowledge
metadata/citation/content uses its own authorized reader; without permission return a safe
unavailable marker, not the base name/ID/excerpt. Never use a service credential to expand
what the viewer may learn. Cross-org resource lookup is indistinguishable from unknown (404)
once the current Organization itself is authorized. Organization authorization failure remains
403. Logs exclude bearer tokens, secrets, source contents and untrusted raw payloads.

## 7. Immutable start configuration and explicit selection

### 7.1 Public Start contract

Keep the existing Product Agent route defined by `productAgentBase`, and its strict 8 KiB
JSON boundary. No `/agents/:id/run`, generic Chat executor or template-specific runtime.
Add only optional exact template selection:

```json
{
  "targetPlatform": "shein",
  "templateSelection": {
    "templateId": "8fc227bb-b572-4138-8e2a-5f1a0be98617",
    "revision": "3"
  },
  "knowledgeSelection": {
    "knowledgeBaseId": "57d38884-032b-4806-8e2a-5f1a0be98617"
  }
}
```

`targetPlatform` and `knowledgeSelection` always express the **final explicitly confirmed
choices**, not implicit server defaults. Template selection records the source defaults.
The Console first resolves the exact default/selected template, prefills these existing
fields, displays actual scope and lets the user change them. Server computes the effective
values and whether they override the selected immutable template. It validates all input;
browser refs identify resources and do not authorize them.

- Omitted templateSelection = no template; do not silently use the enterprise default.
- Omitted knowledgeSelection = no Knowledge, **including when a template has a default base**.
  UI must serialize an intentionally cleared choice by omission, not restore the default.
- Explicit null, partial/empty refs, duplicate keys, unknown fields or non-canonical IDs and
  revisions are invalid. TemplateSelection is Start-only, never accepted on Resume/Review.
- Empty enterprise default is normal. Unavailable default Knowledge must be visibly resolved
  by the user; the server must not silently omit it on failure.
- Request must not contain Organization/actor authority, Definition, AllowedTools, budget,
  provider settings, snapshot refs, Source/Revision content or trusted citation IDs.

This retains the current explicit no-Knowledge request semantics while adding enterprise
admission to **all** new starts, including callers that omit templateSelection.

### 7.2 Internal start snapshot

Define an immutable `ResolvedStartConfig` under agentconfig. It stores exact scope and Product
Binding, normalized public selection, code agent ID/version, source template ref or absence,
resolved target/base or none, existing server policy/prompt versions and hard limits, observed
OrganizationAgent revision/activation epoch, serializer version and digest. No Knowledge
Source revisions or provider secret/model outcome are included.

Add to generic `agent.Request` a comparable bounded value:

```text
ConfigurationSnapshotRef { Kind, ID, Digest }   # strings only; fixed kind agent-configuration-v1
```

The UUID/digest point to the immutable configuration receipt. Core validates its bounded
shape, fingerprints it with the complete Request and carries it unchanged; it never resolves
it. Keep Request comparable: no map/slice/pointer identity or mutable parameter bag. ContextSnapshotRef
continues to refer exclusively to Knowledge. Existing Binding, PolicyVersion, PromptVersion
and Limits are populated from ResolvedStartConfig; they are not reconstructed from current
template defaults on retry. Configuration metadata is not hidden model-prompt material.

### 7.3 Start ordering and crash boundary

```text
fresh identity/use/domain authorization + exact Product binding
  → existing claimed-run lookup / normalize explicit user command
  → T1: create or adopt immutable configuration snapshot (RunDB)
  → T2: existing Knowledge Materialize, if selected (Knowledge DB)
  → existing complete-input Quote preflight, with no reservation/dispatch
  → build complete agent.Request including both opaque refs
  → T3: activation-guarded existing Store.Claim (RunDB)
  → existing Eino/model/tools/KnowledgeDispatchPermit/Review flow
```

T1 unique identity is exactly `(org, actor, context_kind, context_id, request_key)`; agent ID and
all selected bindings/parameters are fingerprint content, not a second uniqueness namespace.
This matches the current Store uniqueness even though it does not have an agent_id column.
T1 locks OrganizationAgent, checks ENABLED, resolves code definition and exact ACTIVE template
revision, inserts immutable metadata, and commits. Concurrent matching attempts adopt that
same row; different normalized commands conflict. No AgentRunID is needed to create it.

Adoption first compares the normalized command and exact binding, then uses stored effective
values; no re-resolution of latest defaults, template head or deployment limits. Fresh security
checks remain mandatory. A missing/corrupt snapshot is an integrity error, not permission to
reconstruct one from current settings. A durable snapshot is **not** an execution permit.

T2 uses the stored selected Base ID and existing #558 identity/selection contract. Knowledge
alone materializes/adopts its exact immutable bundle. T3 occurs after both receipts exist and
initial input bounds are checked. No transaction spans T2, provider I/O or another database.

Snapshots are immutable retained metadata, with no ACTIVE/RUNNING/FAILED lifecycle, lease,
outbox or recovery worker. A pre-Claim crash resumes through the same explicit Start key;
unused snapshots do not initiate work. Cleanup/retention beyond existing data obligations is
not invented in D. Existing Knowledge recovery and AI UNKNOWN remain their owners.

### 7.4 Claimed replay, Resume and definition drift

A Start that already claimed a run is a **receipt replay**, not a new execution. Freshly
authorize the same actor/org/domain, compare incoming normalized command to its original
configuration snapshot and Knowledge selection, then return the original run/request. Do not
use current template/lifecycle/default/limits to rebuild Request or call provider work.
Changed command under the same key is 409. Disabled/archived configuration does not erase
receipts. Protected Knowledge display still follows current permission/lifecycle fences.

Resume loads the original run and Request, references the original snapshot/bundle, rechecks
current execution permissions and enabled admission, and invokes only the existing Resume
with its exact expected revision/feedback. It does not materialize current Knowledge or apply
new defaults. Template archive does not invalidate an already-admitted run's stored config.

Code catalog must resolve the exact stored definition/prompt/policy contract for new work.
If that version is no longer supported, return `AGENT_DEFINITION_UNAVAILABLE`; never route to
latest, another provider, a new template or a second runtime. Read-only receipt/Review access
remains separate. Actual per-invocation model/route/pricing facts stay with AI Capability;
D does not claim one provider is pinned for an entire run when the current owner does not.

## 8. Disable, Claim and in-flight semantics

**Proposed V1 meaning: stop admitting new Start/Resume work, not emergency cancellation of
an already-claimed bounded execution.** UI must say this before disable. It is not the stronger
Knowledge disable/dispatch-permit contract and must not reuse that wording inaccurately.

T3 is one local transaction. A bounded persistence adapter acquires the OrganizationAgent
row lock, validates the immutable snapshot scope/definition, and delegates to the existing
run owner with the same transaction. If required, expose a transaction-bound form of the
existing Claim implementation; do not copy its SQL/state machine. Nested helpers cannot
commit the outer transaction early. Run acquisition is true only after the outer commit.

For a new Start: verify ENABLED and snapshot.activation_epoch == current activation_epoch;
lock/check the selected template is still ACTIVE, then execute existing Claim. Updates of
safe template/default preferences do not change the snapshot. Template archive before Claim
blocks new use. Resume checks current ENABLED but does **not** require the old start epoch;
an explicitly resumed admitted run may continue after disable then re-enable. Existing run
revision/phase/checkpoint conditions still apply. Lock order: OrganizationAgent → template
(for new starts) → run. Configuration writers use the same local owner order after their
command-key claim. Run outcome Commit does not require an activation lock or current enablement.

A duplicate Start discovering an already-existing matching run returns acquired=false before
checking current activation/template status; it cannot be converted to Resume implicitly.
Its fresh identity/domain authorization and exact input comparison still execute.

| Order / operation | Required result |
| --- | --- |
| Disable commits before a new Claim | Claim denied, zero provider work. Prepared snapshot/bundle cannot override it. |
| New Claim commits before disable | That already-admitted bounded execution may finish under its original deadline/budget and ongoing domain/Knowledge checks. No new run is admitted afterward. |
| Snapshot prepared → disable → re-enable → first Claim | Epoch mismatch; 409 `CONFIGURATION_CHANGED`. Explicit new confirmation/key required, no silent snapshot regeneration. |
| Disable versus Resume | Same row-lock ordering: Resume-claim first may execute its bounded continuation; disable first denies new work. |
| Disable after model dispatch | Existing AI invocation usage/UNKNOWN/point settlement and final run Commit still converge; no automatic retry/refund/cancellation here. |
| Read result / submit an existing reviewable result / existing Review / Apply | Keep original fresh domain authorization and exact binding. Disable alone does not revoke product evidence or create a new approval gate. |
| External Organization/member/permission revoke | Existing fresh auth checks govern; no claim of an atomic transaction with external ZITADEL. |

New work cannot proceed from a stale “AVAILABLE” page. A rollback or UI-only feature flag
must not remove this admission guard while exposing an unguarded execution route.

## 9. API contract

New configuration routes are beneath `/api/v1/workbench/agents`; literal routes precede
`:agent_id`. Agent IDs must match registered canonical IDs; route decoding is performed once.
All request bodies reject duplicate/unknown fields and trailing JSON. Config writes ≤8 KiB;
reads accept no body. Replies ≤128 KiB; metadata is bounded before serialization. Default
pageSize 20, max 100 (recent runs max 20), opaque bounded cursor, no unbounded include=all.
Normal config request timeout 10 seconds; no external model call or paid connection test.

| Method / suffix | Input / concurrency | Permission / result |
| --- | --- | --- |
| GET `/market` | cursor/pageSize | read or configure; code catalog + activation + safe readiness. |
| GET `/mine` | cursor/pageSize, optional activation filter | read or configure; existing org rows including disabled. |
| GET `/:agent_id` | none | read or configure; activation revision, exact default, capability reasons and permitted actions. |
| POST `/:agent_id/enable` | `{}`; Idempotency-Key; first row If-None-Match `*`, existing row If-Match | configure; mutation receipt, no run/model. |
| POST `/:agent_id/disable` | `{}`; key + If-Match | configure; lifecycle receipt with in-flight explanation. Missing row is 404. |
| PUT `/:agent_id/default-template` | `{templateId,revision}` OR `{templateId:null,revision:null}`; key + org-agent If-Match | configure; pin exact same-agent ACTIVE revision or clear. |
| GET `/:agent_id/templates` | cursor/pageSize/lifecycle | read or configure; safe template metadata. |
| POST `/:agent_id/templates` | `{name,targetPlatform,defaultKnowledgeBaseId}`; key; no client template ID | configure (+ Knowledge read if nonempty base); create v1. |
| GET `/:agent_id/templates/:template_id` | none | read or configure; head and metadata ETag; protected Knowledge redaction. |
| GET `/:agent_id/templates/:template_id/revisions/:version` | canonical version string | read or configure; exact immutable revision, not latest substitution. |
| PUT `/:agent_id/templates/:template_id` | same typed full payload; key + template metadata If-Match | configure (+ Knowledge read if selected); append revision. |
| POST `/:agent_id/templates/:template_id/archive` | `{}`; key + If-Match | configure; reject default template, append no config revision. |
| GET `/:agent_id/recent-runs` | cursor/pageSize≤20 | use + original Product domain authorization; exact actor scope only. |

ETags encode the resource's own revision, are not global sequence numbers and accept only
one strong validator. Missing precondition 428; malformed 400; stale first attempt 412.
A matching committed command replay precedes that stale check (§5.3). First-enable absence
and uniqueness are checked atomically; simultaneous different-key creation cannot both win.
Forbid simultaneous If-Match and If-None-Match. A nonempty template base ID must be canonical;
null represents no default only in template/default-management schemas, not KnowledgeSelection.

Use bounded stable error codes: INVALID_REQUEST (400), FORBIDDEN (403), NOT_FOUND (404),
IDEMPOTENCY_CONFLICT / CONFIGURATION_CHANGED / AGENT_NOT_ENABLED / TEMPLATE_ARCHIVED /
TEMPLATE_IS_DEFAULT / AGENT_DEFINITION_UNAVAILABLE (409), REVISION_MISMATCH (412),
PRECONDITION_REQUIRED (428), DEPENDENCY_UNAVAILABLE (503). Preserve existing Knowledge,
input-size and Product runtime error mappings on execution routes. Do not turn uncertain
identity/store errors into not-enabled or no-Knowledge success.

## 10. Capability and recent-task projections

Capability descriptor contains support (`REQUIRED | OPTIONAL | NOT_SUPPORTED`), readiness
(`AVAILABLE | NEEDS_CONFIGURATION | REQUIRES_AUTHORIZATION | UNAVAILABLE`), safe reason and
observedAt. NOT_SUPPORTED is a support fact, never a configurable readiness problem.
Permissions and applicable execution-time checks are separate action flags. No status is
persisted as a replacement for the source owner.

| Current title Agent capability | Source / honest V1 projection |
| --- | --- |
| text.generate — REQUIRED | Exact runtime/catalog wiring and existing governed-text readiness. AVAILABLE means configured, not a price quote, reserved points or a provider health guarantee. |
| knowledge.context — OPTIONAL | Existing Knowledge context wiring; chosen base status through authorized metadata reader. Describe as “企业知识引用”, not delivered vector search. |
| image.generate — NOT_SUPPORTED | No title-Agent image execution contract. A separate existing image feature does not make this agent capable. |
| platform.write — NOT_SUPPORTED | Current title Agent has no remote-write tool. A connected Store is insufficient; do not show “authorize to enable” as if the tool already existed. |

Readiness adapters must not call a model, reserve quota, write secrets or materialize documents.
When their bounded source cannot answer, report UNAVAILABLE/unknown with a safe reason, not
AVAILABLE. Configuration module mounts independently of text-runtime readiness so a missing
provider does not prevent disabling or inspecting configuration.

Recent tasks use a narrow read method owned by existing Agent persistence/application.
Join configuration snapshots to runs on the full existing org/actor/context/key identity,
filter agent ID from snapshots, and use the bounded snapshot creation index for “最近发起”.
There is no fake BusinessTask ID, second run table, completion counter or runtime phase in
OrganizationAgent. A snapshot without a run is not a task. Reauthorize original Product
binding before exposing a row; hide unavailable rows safely and maintain a stable cursor.
Admin/configure permission does not bypass existing actor ownership. Enterprise-wide run
visibility is not admitted by D. A saved title proposal is not “已应用/已发布”.

## 11. Browser / BFF behavior

Reuse the current Console layout, navigation, BFF and current Organization selector.
Market/mine/detail/template editor render real loading/empty/denied/unavailable/conflict states.
No fake metrics, sample execution history or success toast before a committed mutation receipt.

Execution confirmation states exact enterprise/Product/action, selected template version,
actual Knowledge scope/version preview, current applicable budget information and Human Review
requirement. Defaults are prefill only. Clearing Knowledge is sticky through validation/retry;
changing effective selections after a submitted command requires a new execution key.
A server configuration conflict must never trigger an automatic different-key model retry.

Organization change/logout/role loss clears selection, cached details, source names, task
results, mutation responses and generated request keys. Key query/mutation identity by org
and auth-generation; discard late responses after switch, including switch A→B→A. Do not
render old protected Knowledge on 403. Mutation success invalidates/refetches both market
and mine through the same owner; it does not start an AgentRun. Desktop/narrow screen checks
cover these interactions using existing components; no new verification framework is required.

## 12. Failure / replay matrix

| Boundary | Recovery owner and required observation |
| --- | --- |
| Config command rollback / crash before commit | Nothing durably changes; same key can execute once after rollback. |
| Config commit, response lost, later resource update | Same key returns original receipt; no new revision/event; separate GET returns current state. |
| T1 snapshot committed, T2 never called | Same key adopts exact stored template/effective values. No run or provider operation yet. |
| T2 Knowledge bundle committed, before T3 | Same key adopts original snapshot/bundle; changed template/defaults cannot rebind them. Current disable/archive/epoch fences may deny first Claim. |
| T3 commit confirmation lost | Read original run via existing identity; do not create a new key, steal RUNNING, replay UNKNOWN or infer no provider send. |
| Template head/default changes after run claim | Read/Resume uses exact original configuration; no history drift. |
| Knowledge disabled/revoked before use | Existing #556 materialization/read/permit fence denies use; D does not substitute an empty bundle. |
| Config owner unavailable | New Start/Resume/config writes fail closed. Outcome persistence and read-only Review remain with existing owners; no config outage should discard model usage. |
| Projection stale / paid resources changed | Recheck actual owners at execution; no cached availability permits work or creates entitlements. |

## 13. Greenfield rollout, scope of supersession and delivery

This adds the owner/schema/API decisions intentionally deferred in #556 Slice D; it does not
rewrite that document's historical baseline or reopen admitted A–C Knowledge/Review semantics.
Specific added contract: all new Product title Start/Resume acquisitions require enterprise
activation; claimed receipt access and current Review are separate. Snapshot references extend
the generic comparable Request without importing enterprise configuration into the runtime.

Fresh installation starts with **zero OrganizationAgent rows**. No implicit enable-on-read,
historical-org backfill, auto-created template, old-ID mapping, wrapper, compatibility route,
dual state or default-allow fallback. Existing explicit no-template/no-Knowledge requests still
work after explicit enterprise enablement. A feature readiness mismatch cannot bypass it.

Implement in the existing current-application assembly and install both schema and guarded
consumer before opening the product entry. The configuration-only milestone may remain behind
its feature gate; do not expose a working enable toggle while title execution ignores it.
Rollback disables new execution routes/rollout and preserves records; do not downgrade to an
older unguarded binary serving those routes or drop historical snapshots to make it start.
No actual environment mutation, migration or deletion is authorized by this document.

Legacy decision: reuse qualified current owners; **RETIRE** any candidate parallel activation
or unguarded route, and **EXTRACT** only still-valid current behavior when relocation is
necessary. This is not an exception to the repository's greenfield/no-compatibility policy.

After admission, default to one execution Delivery Batch/primary PR. Internal milestones:
D1 activation + guarded title use + honest mine; D2 immutable templates/defaults/snapshots
and confirmed consumption; D3 shared market/readiness/recent-run projection. These are not
mandatory separate Issues/PRs. Only independently valuable or independently risky work splits.
Full Chat/BusinessTask follows later and consumes this owner rather than inventing another.

## 14. Verification and admission checklist

D0 verification is document/source consistency, indexed references and applicable repository
checks; it does not pretend to execute future SQL/HTTP/UI contracts. Implementation adds
risk-matched tests to the existing owners/isolated PostgreSQL/controlled-model fixtures.

| Must / test group | Concrete evidence required before implementation delivery |
| --- | --- |
| Ownership / comparable Request | Generic Agent/Eino has no config/DB import; Request equality/fingerprint/2 MiB checks hold; only approved definition supplies tools. |
| Isolation / permissions | Cross-org/actor config refs and run reads denied; configure-only cannot execute; viewer cannot inspect Product/Knowledge; real effective-org role mapping tested. |
| Config transactions | Concurrent first enable, update CAS, same-key changed body, lost response after later update, default↔archive race, object/audit atomic rollback. |
| Snapshot / replay | T1→T2 and T2→T3 failure, same-key exact adoption, template/default change, explicit no-Knowledge despite template default, changed selection conflict. |
| Activation ordering | Disable-first zero model work; Claim-first only already-admitted bounded work; disable/re-enable ABA; Resume ordering; receipt/Review still independently authorized. |
| Existing fences | Knowledge revoke/disable/permit, full-prompt quote, point/UNKNOWN non-redispatch and Review/Apply evidence separation retained, reusing unchanged valid evidence. |
| Projection / UI | Optional Knowledge does not disable title generation; image/write NOT_SUPPORTED; no paid GET; org switch/late response isolation; real save/refetch and desktop/narrow path. |
| Rollout | Empty install no implicit activation; missing schema/readiness fail closed; no unguarded alternate route or destructive fallback. |

Architecture admission remains pending independent review of actual diff: new owner and pool
boundary, atomic Claim ordering, permission composition, replay/snapshot identity and default
semantics. Normal review follows the existing maximum-two-round rule; only demonstrated
BLOCKERs reopen frozen contracts. Implementation preferences go to IMPLEMENTATION_TEST/BACKLOG,
not additional governance platforms. No reviewer approval, CI pass, runtime test, Figma visual
verification, browser acceptance, paid model result or production deployment is asserted here.
