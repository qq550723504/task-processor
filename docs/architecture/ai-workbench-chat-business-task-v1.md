# AI Workbench Chat + BusinessTask V1 — Slice E

> Status: **DRAFT / NOT_IMPLEMENTATION_READY**
>
> Design Basis: **Independent Architecture**.
> Issue: #576. Product parent: #298.
> Upstream product/architecture: #555, #570.
> Delivered implementation dependency: #573 / PR #574, plus the #580 title-model increment.
> Original baseline: `main @ df41c7863d7b152d7e826753d08c52810f5dc9df` (2026-09-29).
> Integration baseline: `main @ 7377f61d0f56a795f67df34e2ec5f637a4d8c8ce` (2026-10-02).
> Matching #574 push/main CI `36593853258`: **SUCCESS**, historical dependency evidence only.
> Provider-portability decision: user explicitly requires no single LLM vendor lock-in
> and approved Eino model interfaces + eino-ext on 2026-09-30; see §1.2 and §6.
>
> This document is a D0 review candidate. It changes no production schema/API/UI.
> Production Writer remains blocked until this architecture reaches
> `APPROVED / IMPLEMENTATION_READY`, is merged to main, and an execution Issue is admitted.
> Merge, deployment, real customer data and paid provider use remain separately authorized.

## 1. Product outcome and authority

Slice E turns the already delivered bounded title Agent into one user-facing AI Workbench
path without creating a second Agent, Workflow, Review or business-fact system.

```text
当前企业成员
  → 硕米Chat 新建/打开会话
  → 选择已保存商品 + 目标平台，并表达自然语言目标
  → Chat 只做无工具澄清/规划
  → server emits immutable typed execution proposal
  → 用户显式确认 exact Agent/template/Knowledge/limits/Human Review
  → durable BusinessTask
  → existing product.title.agent Start
  → Task Center projects existing AgentRun + Product Review
  → existing Human Review / explicit Apply
```

Applicable product authorities:

- `docs/product/final-ui-ia-authority.md`;
- Figma `tg48P46SSXl6TBy9lZwg63 / 31:463`;
- Chat `427:2467`;
- Task Center `427:3005`;
- #298 product relationship:
  `BusinessTask -> AgentRun -> AgentStep -> ToolCall / ModelCall -> Temporal/Queue/Internal Task`;
- `organization-agent-configuration-v1.md`: enterprise enablement, exact templates,
  configuration snapshots and guarded Product Agent admission;
- `agent-knowledge-context-v1.md`: optional exact Knowledge context/citation/permit;
- `product-agent-runtime-contract.md`: AgentRun/checkpoint/budget/UNKNOWN ownership;
- `product-agent-text-provider-neutral-v1.md` / #580: current organization-only credentials,
  operator provisioning, per-organization title policies and compatible-protocol transport;
- `project-boundaries.md`, greenfield baseline and Issue-driven delivery rules.

Figma governs UI/IA and interaction semantics. It does not create a capability, permission,
business fact or production-readiness claim.

### 1.1 V1 scope

V1 has exactly one executable business-task kind:

```text
product.title.optimize -> product.title.agent / v1.0.0
```

In scope:

- new/recent/favorite Chat conversations;
- durable owner-scoped Conversation/message history;
- one no-tool Chat planning capability;
- provider-neutral model access for Planner and the existing title Agent through Eino
  model components and selected eino-ext implementations, governed by current AI owners;
- immutable typed execution proposals;
- explicit execution confirmation;
- durable BusinessTask intent + exact execution identity;
- Product Agent Start/Resume/Review consumption through existing owners;
- deterministic Task Center projection;
- existing Product Review/Apply;
- truthful coexistence with current source-specific Task Center slices.

Out of scope:

- Project as a new persistent owner;
- Report as a new persistent owner;
- Multi-Agent/router/planner selecting arbitrary Agents;
- arbitrary Tool calls from Chat;
- prompt/Tool allowlist/AgentDefinition editing;
- BYOK/custom endpoint management UI, user/model-supplied credentials or arbitrary model routing;
- image Agent orchestration;
- platform remote write/publish;
- embeddings/vector search;
- temporary attachment auto-save to enterprise Knowledge;
- shared/team conversations;
- enterprise-wide task visibility;
- new Temporal/queue/retry/Saga/recovery platform;
- new IAM/Review/audit/billing owner;
- legacy Task migration/backfill/dual-write/synchronization.

### 1.2 Provider portability is a current requirement

The user explicitly rejected a permanent dependency on GRSAI or a fixed model. The approved
product direction is **Eino standard model interface + eino-ext provider implementations +
existing AI Capability governance**, not another vendor-specific Planner.

For the Slice E implementation, §6 supersedes the earlier proposal to create
`internal/integration/aiworkbench/grsaitext.Planner` and to require the same GRSAI/model route
as the Product Agent. The original `grsaitext` baseline has since been replaced on main by
`internal/integration/agent/titletext` in PR #580. Its approved
`product-agent-text-provider-neutral-v1.md` already removes single-vendor restrictions within
OpenAI-compatible text and owns organization-only credentials, restricted operator provisioning,
per-organization policy selection and capability readiness. Reuse those changes; do not implement
or review the old hardcoded-vendor removal a second time.

The remaining Slice E increment is Eino/eino-ext component reuse for both consumers, qualified
native protocol support and exact run-level model-profile binding. This draft proposes to
supersede only the compatible-protocol implementation restriction in that document's §2.1 and
its §4 permission to reselect a current route between steps, for new profile-bearing Slice E
runs. Such runs retain one exact profile for all steps/Resume (§6.3); a changed profile requires
new confirmation, not a silent switch. Its credential, accounting, UNKNOWN and authorization
contracts remain mandatory. This draft does not retroactively change old runs or claim that
its new native-protocol contract is already approved or implemented.

Planner uses Eino's model component directly, without an Eino execution graph or fake AgentRun.
Product Agent keeps its existing Eino graph and `agent.GovernedModel` contract. Both consume the
same governed model integration; they may select different admitted models. GRSAI may remain
one qualified configured service, never the sole supported service or an implicit fallback.

Managed route configuration is not BYOK. Only operator-approved, organization-scoped route and
credential facts may select an implementation. No model key, endpoint or SDK option is accepted
from a Conversation, AgentTemplate or browser execution payload.

## 2. Verified current-state map

| Current source / behavior | Architectural implication |
| --- | --- |
| Console navigation exposes `/workbench/ai/chat/*` as unavailable | Chat is a real product slot but has no current implementation owner. |
| Task Center shell and `pending` / `completed` are connected | Reuse UI primitives, not their source-specific facts as BusinessTask facts. |
| Product Review pending page explicitly says “不创建 BusinessTask” | Existing Review remains its own owner and must not be silently relabeled. |
| Completed-work projection explicitly says it is not BusinessTask/AgentRun | Historical/source-specific records remain truthful projections during cutover. |
| `agent.Store` owns exact run identity and bounded checkpoint | BusinessTask cannot copy runtime lifecycle/retry. |
| `agentconfig.Guard` wraps exact Product Agent Claim with activation/template/ceiling checks | Chat confirmation must ultimately consume this same guarded runtime path. |
| Agent D stores exact configuration snapshots and exposes exact recent-run identity | Proposal/Task must carry exact refs, never “current default” as durable execution meaning. |
| Product Agent route requires `workbench.agent.use` plus current `listingkit.admin.write` | Chat permission never replaces domain execution authorization. |
| Product Review owns pending/accepted/rejected/applied + Apply receipt | Task state is a projection over Review, never a copied approval state machine. |
| Current `titletext` model selects per-organization policies; #580 replaced `grsaitext` | Preserve the delivered provider identity, limits, quoting and usage behavior; component reuse is the remaining increment. |
| Current title resolver uses only the organization's empty-UserID credential row; operator provisioning has a separate writer connection | The new model factory must not regress to user-first lookup, global keys or a serving role with credential writes. |
| Current Manager/provision validation is OpenAI-compatible-protocol-specific | Native eino-ext protocols require the bounded credential/provision extension in §6.2, not passage through `ResolveTextRouteDetails`. |
| Repository pins Eino `v0.9.21`; provider components are not yet qualified for this path | Use the standard model interface; lock and test selected eino-ext modules during implementation, not `@latest` in production. |
| Legacy Task-first Product UI / generic Task Dashboard is RETIRE | No BusinessTask ↔ legacy Task adapter, migration or fallback. |

## 3. Ownership and dependency direction

### 3.1 New bounded product owner

Create one bounded product domain:

```text
internal/aiworkbench
  owns:
    Conversation
    ConversationMessage
    PlanningCommand receipt
    ExecutionProposal
    BusinessTask
    BusinessTask projection policy
```

HTTP belongs under:

```text
internal/aiworkbench/httpapi
```

PostgreSQL adapter belongs under:

```text
internal/integration/persistence/aiworkbench
```

`internal/app/httpapi` only assembles dependencies/routes into current application.
It does not own Chat or BusinessTask rules.

### 3.2 Owners that remain unchanged

`aiworkbench` does **not** own:

- AgentRun/AgentStep/checkpoint/budget/Resume;
- AgentDefinition/AllowedTools;
- OrganizationAgent/template/configuration snapshot;
- Knowledge bundle/revision/permit/citation;
- provider credentials/routing/model invocation/usage/cost/UNKNOWN;
- Product/Catalog/Sourcing/Asset facts;
- Product Review decision/CAS/Apply;
- Temporal/queue/internal task lifecycle;
- Organization identity/RBAC;
- Project or Report.

Dependency direction:

```text
aiworkbench -> narrow planning / AgentConfig / Product / Review / Agent ports
integration/persistence/aiworkbench -> aiworkbench contracts
integration/aiworkbench/einoplanner -> aiworkbench.Planner + aicapability.GovernedText
integration/agent/einomodel -> agent.GovernedModel + aicapability.GovernedText
integration/aicapability/einomodel
  -> aicapability contracts + Eino model.BaseChatModel + selected eino-ext components
app/httpapi -> feature HTTP + explicit owner/adapter injection
```

These new names are proposed implementation contracts, not claims that the packages already
exist. `aicapability.GovernedText` is a bounded extension of the current AI owner, not a new model
platform. Domain packages do not import Eino or vendor SDK types. The generic `internal/agent`
package never imports `aiworkbench`.

## 4. Persistence placement and runtime role

Use schema:

```text
ai_workbench
```

V1 deploys it in the **same logical PostgreSQL database used by ProductAgent RunDB**, but
through a distinct `ai_workbench_runtime` login and independent bounded pool.

Reasons:

1. no new database lifecycle is needed for this first Workbench product slice;
2. Chat message contents should not be readable merely because a process has
   `product_agent_runtime` privileges;
3. no cross-owner atomic transaction is required, so sharing a runtime SQL role is unnecessary;
4. same logical database does not grant permission to join or mutate another owner's tables.

The serving role receives only explicit `ai_workbench.*` DML privileges. Schema installation is
explicit and uses the schema owner; constructors and HTTP traffic never create/migrate schema.

No SQL join from `ai_workbench` tables to `product_agent_runs`, Product Review tables,
Knowledge tables or AgentConfig tables is an admitted domain contract. Projection consumes
narrow owner ports.

Greenfield install starts with zero Conversations/BusinessTasks. There is no migration/backfill
from old Task records, Product Review rows or completed-work rows.

## 5. Conversation contract

### 5.1 Canonical Conversation

```text
Conversation {
  id
  organization_id
  owner_user_id
  title
  favorite
  lifecycle          ACTIVE | ARCHIVED
  metadata_revision
  next_sequence
  created_at
  updated_at
}
```

Rules:

- Organization and actor come only from freshly verified current identity;
- V1 visibility is exact `(organization_id, owner_user_id)`;
- admin/configure permission alone does not grant another member's Chat;
- no hard delete in V1;
- archive is reversible metadata but blocks new messages and new confirms;
- archive/favorite/title changes do not alter Task/Agent/Review history;
- initial title is deterministic from the first user message, bounded and editable;
  no separate model call exists only to name a conversation.

### 5.2 Append-only messages

```text
ConversationMessage {
  message_id
  conversation_id
  sequence
  author_kind       USER | ASSISTANT
  content
  created_at
}
```

Messages are immutable. “Edit” means append a new user message; old input is never rewritten.

Bounds:

- user content: max 8 KiB UTF-8;
- assistant content: max 16 KiB UTF-8;
- list page: max 50 messages;
- one planning call receives at most 64 KiB of bounded recent Conversation text;
- no raw system prompt/provider request/response/credential/Knowledge document/Product blob
  is persisted as message content.

Sequence allocation locks the Conversation row and monotonically increments `next_sequence`
for both USER and ASSISTANT messages. Proposal freshness compares only the latest USER sequence
in that same org/owner/conversation, never `next_sequence - 1` or the latest arbitrary message.
The latest USER sequence is derived from the bounded/indexed owner query while holding that row
lock; it is not a second editable conversation fact. An ASSISTANT append cannot invalidate its
own proposal. Metadata-only favorite/title operations do not invalidate a proposal either.

### 5.3 Message idempotency and durable pre-dispatch state

Every user-message mutation requires one canonical UUID `Idempotency-Key`.

The first transaction locks the Conversation and atomically appends the USER message plus a
planning command. The command freezes a deterministic planner invocation identity **before any
provider dispatch can be claimed**:

```text
PlanningCommand {
  organization_id
  actor_id
  idempotency_key
  operation                chat_message_plan
  conversation_id
  request_fingerprint
  user_message_id
  source_sequence           # exact Conversation sequence assigned to this USER message
  planner_invocation_id     # deterministic SHA-256 over scoped command identity
  planner_input_hash        # canonical history/work-scope envelope through source_sequence
  planner_model_profile     # non-secret exact route/policy snapshot described in §6.3
  planner_started_at        # database time frozen once
  planner_deadline          # frozen bounded attempt deadline
  state                     READY_TO_DISPATCH | COMPLETE |
                            FAILED_BEFORE_DISPATCH | PLANNER_UNKNOWN
  assistant_message_id?
  proposal_id?
  committed_at?
}
```

`planner_invocation_id` is derived from the exact
`(organization, actor, conversation, idempotency_key, request_fingerprint)` tuple. It is not a
browser/provider ID. Same key + same fingerprint therefore always addresses the same AI
invocation identity. Same key + changed content/scope is `IDEMPOTENCY_CONFLICT`.

The T0 transaction allocates the message sequence under the Conversation row lock and builds the
canonical planner input from immutable messages with `sequence <= source_sequence` plus the
exact selected work-scope fields. It stores the resulting `planner_input_hash`. Later messages
are never incorporated into a replay of this command. A same-key replay reconstructs that exact
bounded history prefix and must match the frozen hash before it can approach the AI claim.

The trusted model profile is resolved without provider I/O and persisted with T0 before a
command can become READY_TO_DISPATCH. If no admissible profile is available, persist the user
message with FAILED_BEFORE_DISPATCH; do not silently choose a fallback. A retry first adopts the
stored command/profile rather than selecting a new current route. Invocation input identity
also binds the exact profile digest and fully serialized model input; see §6.3.

The durable dispatch algorithm is:

```text
T0 transaction:
   create/adopt USER message + source_sequence
   + PlanningCommand(READY_TO_DISPATCH, deterministic invocation, exact input/profile)
→ outside transaction call Planner with that exact invocation identity
→ Planner calls the shared governed text executor
→ executor atomically ClaimInvocation in the existing AI invocation owner
     acquired=true  => this caller alone may reserve + attempt provider transport
     acquired=false => zero provider transport; inspect exact existing invocation
→ Workbench terminal CAS:
     known success             => append one ASSISTANT message + optional proposal + COMPLETE
     authoritative no-dispatch => FAILED_BEFORE_DISPATCH
     unresolved dispatch after bounded deadline/grace => PLANNER_UNKNOWN
```

Important replay rules:

- if T0 never committed, there is no message/command;
- if T0 committed but no AI invocation fact exists, the call is **definitely undispatched** and
  same-key replay may safely attempt the existing AI `ClaimInvocation`, using its original
  profile and only while its deadline and current authorization still permit dispatch;
- if an exact AI invocation fact exists with outcome `dispatched`, no Workbench path may issue
  another provider call. Before `planner_deadline + terminal-write-grace` the request projects
  `PLANNER_PENDING`; after that bound it CAS-terminalizes the Workbench command as
  `PLANNER_UNKNOWN`;
- if the AI invocation becomes terminal but the Workbench assistant/proposal commit was lost,
  Workbench still does **not** redispatch because the AI ledger does not persist provider text.
  Until the same bounded grace expires it remains PENDING so the original writer can finish;
  afterwards it becomes `PLANNER_UNKNOWN`;
- terminal Workbench receipts replay exactly and never call the planner;
- a user may explicitly send a new message/new key after UNKNOWN; that is a new paid planning
  intent, never a retry of the old invocation.

This intentionally preserves the existing AI owner's conservative dispatch boundary: once
`ClaimInvocation` exists, a crash cannot prove whether transport crossed the process boundary,
so the old invocation is never re-sent.

Provider invocation, reservation, usage settlement and dispatch UNKNOWN remain owned by current
AI Capability/commercial owners. The Workbench command owns only message/product replay and the
bounded PENDING→UNKNOWN decision above.

## 6. Chat planner and provider-neutral model contract

### 6.1 No tools and no execution authority

Planning is:

```text
bounded Conversation text
+ fixed capability description
+ explicit user-selected work scope
  -> governed Eino model component
  -> Clarification | ReadyPlan
```

The planning model receives **no Commerce Tool Registry**, no arbitrary tool schema and no
execution credential. It does not invoke AgentRuntime or Product mutation. Using Eino's
`model.BaseChatModel` does not require an execution graph or AgentRun.

V1 explicit work scope is browser-selected then server-authorized:

```text
operation_id          # existing acquisition/Product binding identity
target_platform       shein | temu | amazon
template_selection?   exact template_id + revision, or explicit no-template
knowledge_selection?  exact KnowledgeBase ID, omission = no Knowledge
```

The model is not allowed to select Product, Organization, Agent ID, Tool, template ID,
KnowledgeBase ID, platform, provider or model. Those are explicit user/server facts.

The model returns only a strict bounded shape:

```text
PlanningDecision {
  mode             CLARIFY | READY
  assistant_text
  goal_summary?
}
```

Server code constructs the typed `ExecutionProposal` by combining a READY decision with exact
freshly authorized work-scope facts.

A `BaseChatModel`-typed variable is not proof that the concrete model is tool-free. Planner
construction must use an unbound instance, never a mutable instance shared with Agent tool
binding. Reject tool messages, ToolCalls, multimodal output and non-admitted structured output;
never pass browser/model-controlled `model.Option`, provider extras, WithTools or BindTools.
The V1 path uses non-streaming Generate; streaming support in the interface is not an additional
product commitment. Framework callbacks must not export prompts, Knowledge, secrets or raw
provider bodies; existing metadata-only audit remains the authority.

### 6.2 Contract → implementation → injection → consumer

The following are proposed bounded extensions, not claims of existing implementation:

| Contract / owner | Concrete implementation and injection | Consumer |
| --- | --- | --- |
| `aiworkbench.Planner` | `internal/integration/aiworkbench/einoplanner`: bounded input mapping and strict PlanningDecision parsing; receives `aicapability.GovernedText` | Chat message service |
| `agent.GovernedModel` | `internal/integration/agent/einomodel`: extract current `titletext` prompt/evidence/citation/Quote and per-organization policy semantics; receives the same governed text service | Existing Product Agent Eino runtime |
| `aicapability.GovernedText` | `internal/integration/aicapability/einomodel`: Quote/Generate orchestration over current route, identity, invocation and resource ports; Eino message mapping stays here | Both bounded consumer adapters |
| Eino `model.BaseChatModel` | Code-owned factory in that Integration package constructs selected `eino-ext/components/model/*` implementations with scoped credentials and guarded HTTP transport | Governed text executor only |
| Current credential/configuration owner | Extract #580's organization-only credential lookup/versioning into a provider-neutral port over the same `ai_client_credentials`; preserve the restricted operator writer | Route resolution and model construction |
| Existing invocation/resource owners | Same `ai_invocations`, dispatch claim, observed usage and ResourceAIPoint reserve/finalize | Governed text executor |
| Existing Knowledge owner | Exact bundle reads/citation validation and final dispatch permits; no retrieval in Planner | Product Agent model adapter + guarded handoff |

The factory is a small code-owned constructor allowlist, not a new provider registry service,
plugin platform, provider-management schema or credential database. Provider-specific imports,
serialization and SDK configuration stay below the business interfaces. Eino and eino-ext
already provide the model abstraction/implementations; do not recreate a parallel ChatModel SDK.

Selected component families include `eino-ext/components/model/openai` for qualified
OpenAI-compatible protocols and native components such as `claude`, `gemini` or `ark` when
admitted. Protocol compatibility does not imply common service ownership: keep actual
provider/service identity separate from adapter kind and model identity. A native provider
must not be forced through GRSAI or an OpenAI-compatible intermediary.

The existing `openai.Manager.CompleteText` / `titletext.AgentTextModel` are **not** mandatory
wrappers around the new factory. Extract still-valid configuration/credential, bounded queue,
fresh authorization, dispatch/usage and Knowledge behavior to the current owners' narrow
ports. Do not stack a second SDK call, queue, ledger or fallback under those wrappers.

#### Current credential and provisioning seam

Both consumers select a deployment-owned profile by verified Organization and capability, never
by a member-controlled override. Retain the current `ProductAgent.TextPolicies[org]` map for the
title capability; add a bounded `PlanningTextPolicies[org]` map in the Workbench composition for
the planning capability. Each capability has at most one admitted route per organization in V1.
Both may intentionally refer to the same `ClientName`, but neither silently borrows the other's
policy when its own entry is missing. This is typed deployment configuration, not a runtime
routing directory or new persistent registry.

The resolver reads exactly `ai_client_credentials` in the existing ProductAgentDB, keyed by
`(TenantID = verified EffectiveOrganizationID, UserID = "", ClientName = profile.client_name)`.
Preserve `NewOrganizationOnlyCredentialResolver` semantics; the older organization resolver's
user-first lookup is not sufficient. Missing/disabled organization rows do not fall back to a
member row, process environment, global key or SDK default credential chain. A read-only owner
port is injected using the existing credential-serving pool; `ai_workbench_runtime` does not
gain direct credential-table privileges. Member identity still belongs in authorization and
usage accounting, not in credential selection.

Reuse the owner and restricted writer behind `cmd/product-agent-credential-provision` and
`SaveTitleCredential`, rather than adding a new secret store or browser endpoint. The bounded
implementation extension adds a typed consumer selector (title or planning) that validates
against the corresponding deployment map before opening the writer connection. Preserve the
same target-database check, independent restricted writer role, private-input handling,
forced empty UserID, safe output, and serving-role SELECT-only boundary.

For a newly admitted native protocol, row validation/version computation and provisioning
read-back must use the provider-neutral credential port and code-owned adapter policy; they
must not invoke `Manager.ResolveTextRouteDetails`, whose current text path admits only
compatible protocols. Reuse the existing row fields and configuration-version behavior. The
adapter/protocol discriminator is validated by the small factory allowlist; the real provider
identity still comes from the deployment profile. Initial qualification covers direct API-key
OpenAI-compatible and native Claude routes only; ambient cloud credentials, arbitrary headers,
provider-specific extra fields and credential auto-discovery are not admitted.

The first-write sequence stays: validate deployment target and policy shape → privately write
the existing organization row → return only non-secret version/route identity → bind that
version in deployment policy → enable execution. No native-client constructor or read-back
may call a provider. A missing admitted version leaves execution unavailable. Rotation uses
the same controlled writer and invalidates old profiles without rewriting them.

Application assembly explicitly injects the scoped resolver, frozen policy, current recorder,
resource adapter, bounded concurrency control, guarded transport and factory into GovernedText;
then injects consumer-specific adapters into Chat and Product Agent. Constructors do not
contact a model, open a second accounting database or install schema.

Capability projection must consume the same per-capability organization policy and credential
readiness check as the executor, preserving #580's `UNAVAILABLE` versus
`NEEDS_CONFIGURATION` distinction. A route merely resolving is not proof of admission. This
read-only projection neither probes balances/provider health nor grants execution permission.

Existing-owner extensions remain bounded:

```text
internal/aicapability
  + CapabilityAIWorkbenchChatPlanning = "aiworkbench.chat_planning"
  + OperationAIWorkbenchChatPlan      = "aiworkbench_chat_plan"
  + provider-neutral GovernedText Quote/Generate and non-secret model profile contracts
  + existing InvocationDispatchClaimer / Recorder / ReplayReader / usage interfaces

internal/aicapability/store
  same ai_invocations table
  priced-text validation explicitly admits product_agent_decision | aiworkbench_chat_plan
  planner requires AgentRunID == "" and BusinessTaskID == ""
  Product Agent still requires its real AgentRunID
  identity/fingerprint includes the frozen profile, not a mutable route alias
  observed-usage invalid output is billable for either admitted operation

internal/integration/orgresource
  same ResourceAIPoint/member-limit owner
  readFact and native fact reader validate the two operation-specific identities
  Product Agent BusinessScope = real AgentRunID
  Planner BusinessScope = "chat-plan:" + invocation_id
  one operation-aware BusinessScope function for reservation and replay validation
  no new Chat ledger; no invented credit or free call
```

Use the existing `ModelInvocationAuthorizer` seam. Product Agent retains its current domain/
Agent permissions; Planner fresh-resolves the same org/user/member and requires
`workbench.chat.use`. The shared executor cannot infer one consumer's permission from the other.
Planning inability never creates a BusinessTask or AgentRun.

### 6.3 Frozen model profile, not a fixed vendor

AI Capability owns a typed, non-secret snapshot of an admitted route and policy:

```text
ModelProfile {
  profile_id / configuration_version
  client_name
  provider_id / adapter_kind / model_id
  endpoint_identity_digest
  credential_reference / credential_version
  routing_policy_version / adapter_policy_version
  prompt_version / output_schema_version
  usage_mapping_version / cost_pricing_version
  point_tariff
  maximum_prompt_tokens / maximum_completion_tokens
  maximum_input_bytes / maximum_output_bytes / deadline_bound
}
```

This is an immutable use-time snapshot of existing configuration facts, not a second editable
configuration owner. Credential references are opaque; secret values and raw endpoint URLs
never enter Conversation, BusinessTask, AgentRun payloads or ordinary logs.

New calls may resolve a different admitted provider/model by configuration. A command already
prepared or claimed cannot change provider, model, endpoint identity, credential version,
pricing, bounds, prompt or output schema under the same invocation identity. Planner stores
its exact profile with the T0 command. InputHash binds org/user/member, operation, exact
history/work-scope input, profile digest and complete serialized model input. Native invocation
metadata preserves these exact facts for observation, settlement and owner recovery. Extend
safe metadata/fingerprint validation in the current invocation owner where a field is not yet
represented; do not infer it later from mutable defaults or store provider payloads there.

Existing invocation/Workbench receipts are looked up and fingerprint-checked before resolving
a new route. A matching receipt can be read even if its provider is now unavailable, subject
to current read authorization. This does not authorize new dispatch. Before every new transport
handoff recheck current organization/member permission, route/credential version, availability
and applicable hard ceilings. A mismatch fails closed; do not silently clamp or choose another
model. Relaxing current limits does not enlarge a prepared request.

For Product Agent, resolve and show the exact execution model profile at execution confirmation.
Store it within the existing AgentConfig start snapshot, so ConfigurationSnapshotRef/digest and
the existing Agent Request fingerprint bind it for the whole run, including Resume. Carry the
existing opaque ConfigurationSnapshotRef into `agent.ModelInput` as a narrow additive field;
the model adapter loads the profile from AgentConfig, rather than importing provider fields
into the generic runtime or consulting latest defaults at every model step. Template tables do
not become a model/key store. Both direct title execution and Chat-confirmed title execution
use this same snapshot/adapter path.

The proposal's execution profile is distinct from the already-used planning profile. Confirmation
may not substitute a different execution profile after the user saw the proposal. Store its
non-secret profile/digest in ExecutionProposal, validate it before first-time confirmation, and
include it in the resulting configuration snapshot. A changed profile requires new explicit
confirmation/proposal; an already committed task still replays first as specified in §8.2.

### 6.4 One invocation, at most one transport attempt

`BaseChatModel.Generate` alone is not a dispatch, billing or retry contract. The shared governed
executor, not the component or callback framework, owns this sequence:

```text
exact immutable input/profile + fresh consumer authorization
→ existing native ClaimInvocation (only acquired=true grants work)
→ existing ResourceAIPoint reservation
→ existing bounded rate/concurrency admission
→ final transport gate: fresh auth + exact route/credential + deadline/byte bounds
→ Product Agent only: acquire current Knowledge dispatch permit for exact bundle
→ one outbound generation attempt through the selected eino-ext component
→ typed usage/dispatch observation
→ existing native terminal fact and resource settlement/release
```

Quote is side-effect-free with respect to model dispatch, reservations and invocation claims.
It accounts for the complete actual message envelope, current context and configured token/
byte limits, including bounded SDK serialization overhead; it cannot be a provider-name check.
Existing exact Quote equality/preflight semantics still apply.

All selected components must use an injected, invocation-bound transport wrapper at the actual
HTTP handoff. It rechecks after SDK/application queueing, allows at most one underlying model
request per acquired invocation, rejects redirects/retries before any additional network send,
and bounds outbound/inbound bytes. Disable SDK retries where the pinned version exposes the
setting; the transport guard remains required so hidden retries cannot bypass the contract.
An OnStart callback or one Generate call is not a substitute. No automatic cross-provider
fallback, speculative parallel generation or output-repair model call is admitted.

The existing Knowledge permit must be acquired immediately before that guarded handoff and
released through its current bounded cleanup on every outcome. Keep the existing maximum
provider deadline compatible with the current Knowledge permit lease; do not lengthen it as a
side effect of changing SDKs. Waiting past a deadline or losing permissions permits no send.

Before handoff, a proven rejection is NOT_DISPATCHED; release only the current invocation's
reservation through the existing owner after a durable no-send fact. After handoff, an
ambiguous network/SDK/parse result is OUTCOME_UNKNOWN, even when no response was received.
A rejected second SDK attempt never erases uncertainty from the first. Only current owner
recovery with real evidence may settle that uncertainty; Workbench must not refund, reset an
invocation ID or switch providers. No component lacking controllable transport can be admitted.

Callbacks/tracing are metadata-only. No prompt, full model response, hidden reasoning, provider
error body, credential or Knowledge text is exported to an extension observability backend.
Model instances/options are never shared mutably across organizations or consumers.

### 6.5 Capability and usage normalization

The factory admits a route only when its adapter policy proves bounded text generation,
strict output validation, complete input/output accounting, zero additional transport attempts,
explicit scoped credentials and the final-handoff gate. Features such as JSON Schema, native
tools, reasoning and cached-token reporting differ by component; a common interface alone
proves none of them. JSON mode may help, but server-side strict schema/size/role validation
remains required and invalid output never causes an automatic repair call.

`schema.Message.ResponseMeta.Usage` is optional. A non-nil struct or integer zero is not enough
to prove raw counters were present. Normalize provider usage with a versioned policy and, where
needed, bounded in-memory transport observation of counter presence. Persist only safe typed
usage/metadata, never the raw body. Missing/inconsistent counters remain UNKNOWN, not free.

Preserve separately:

- provider-reported usage with its known/missing semantics;
- canonical input/output counts accepted by the current invocation/resource owner;
- provider cost estimate under the frozen cost policy;
- customer AI points under the frozen PointTariff.

Cache read/write and reasoning tokens must not be double-counted or dropped. Do not blindly
assume every native provider's total has the same meaning. V1 admits only configurations whose
observed billable categories can be mapped correctly to the existing input/output tariff and
cost contracts. Unmodeled features are disabled or the route is unavailable; adding a native
adapter does not authorize changing billing rules. Invalid structured output with trustworthy
usage remains observed_usage_failed and is settled once by the current owner.

### 6.6 Product Agent cutover and validation scope

Do not solve portability only for Chat while leaving actual title execution on a separate model
integration. #580's `integration/agent/titletext` already removed its GRSAI/fixed-model checks;
do not repeat that work or preserve stale tests asserting a vendor-only baseline. In the admitted
Slice E batch, extract that current adapter's valid behavior into `integration/agent/einomodel`,
backed by the same governed executor and selected eino-ext components as Planner.
Extract the existing prompt/evidence projection, deterministic candidate/citation validation,
Quote/limits, organization-only credential selection, identity and UNKNOWN logic; do not delete
those checks to make another SDK work. The Eino graph, Store.Claim/Commit, Tool Registry, domain
validation and Human Review remain their current owners.
Native provider tool execution remains disabled; the existing graph alone interprets validated
action JSON against its code-owned allowlist.

Current main demonstrates qualified compatible-protocol substitution, not native eino-ext
portability or run-level profile freezing. Cutover must cover both direct and Chat-originated
execution. Retire the replaced model consumer after callers switch; do not retain a second
implicit fallback path. GRSAI can be an explicitly qualified route via an appropriate component.
No historical run/snapshot is silently rewritten.
Missing new profile metadata on an old prepared request requires fresh explicit confirmation;
old claimed records remain readable but cannot acquire new model work through an unprofiled
compatibility path. Actual environment/data changes require separate authorization.

A minimal portability proof uses two real eino-ext component implementations against isolated
synthetic transports, not two fakes of our own interface. The initial contract-test pair is an
OpenAI-compatible component and a native Claude component; neither requires live credentials
or paid calls. The same Planner and title-model consumer code must pass with both, without
vendor conditionals. Other components may be added only through the same bounded qualification;
this does not require connecting every provider before delivery.

Pin each selected component module/version and dependency graph compatible with the repository's
Eino version. Compilation, transport-request counts, counter-presence mapping and scoped
credential/Knowledge gates must be tested at those exact versions. No dependency upgrade or
multi-provider runtime success is claimed by this D0.

Primary upstream references checked for this design (not runtime qualification evidence):

- [Eino model interface at v0.9.21](https://github.com/cloudwego/eino/blob/v0.9.21/components/model/interface.go): BaseChatModel and separate tool-binding interfaces.
- [Eino response metadata](https://github.com/cloudwego/eino/blob/v0.9.21/schema/message.go): optional Usage and token detail fields.
- [eino-ext component overview](https://github.com/cloudwego/eino-ext): official model implementations.
- [OpenAI-compatible component](https://github.com/cloudwego/eino-ext/blob/main/components/model/openai/chatmodel.go) and [module](https://github.com/cloudwego/eino-ext/blob/main/components/model/openai/go.mod): component construction and injected HTTP client.
- [Claude native component](https://github.com/cloudwego/eino-ext/blob/main/components/model/claude/claude.go) and [module](https://github.com/cloudwego/eino-ext/blob/main/components/model/claude/go.mod): separate native adapter/configuration, not a GRSAI requirement.

## 7. Immutable ExecutionProposal

A READY planning turn creates:

```text
ExecutionProposal {
  proposal_id
  digest
  organization_id
  owner_user_id
  conversation_id
  source_user_message_id
  assistant_message_id
  source_sequence          # the originating USER sequence, not the ASSISTANT sequence
  kind                 product.title.optimize
  goal_summary
  operation_id
  Product binding      product_key/catalog_version/publication_id
  target_platform
  agent_id             product.title.agent
  agent_version        v1.0.0
  observed_agent_revision
  observed_activation_epoch
  template_ref?
  execution_model_profile   # exact non-secret profile/digest from AI Capability, §6.3
  knowledge_selection_ref? {
    knowledge_base_id
    revision_set_digest
  }
  created_at
}
```

The proposal is immutable. If Knowledge is selected, READY construction calls the Knowledge
owner's `ObserveSelection` after the planner returns and before the assistant/proposal
transaction commits. If the selection is not currently readable, the assistant may explain the
failure but no READY proposal is stored. The observer returns only the revision-set identity,
never document text.

A proposal's `source_sequence` must equal its command's originating USER sequence and identify
`source_user_message_id` in the same org/owner/conversation. It is fresh only while that is the
latest USER sequence. Its own ASSISTANT message, another delayed assistant reply, or a
metadata-only title/favorite change cannot invalidate it. A later USER message makes it stale.
A delayed response to an older turn remains attached to its original command; it never rewrites
its source sequence or binds to the latest turn. It may be shown as historical, but any READY
proposal from that older turn is non-confirmable.

For a first confirmation, the server freshly validates:

- current identity/effective Organization/owner;
- `workbench.chat.use`;
- exact Product binding + existing Product execution authorization;
- `workbench.agent.use`;
- existing `listingkit.admin.write`;
- Agent is still enabled and its activation epoch/revision has not invalidated the proposal;
- exact template is still active when present;
- exact execution model profile remains admissible without changing the user's confirmed route;
- exact optional Knowledge selection is currently readable/active **and has the same observed
  revision-set digest**;
- current AI/point/resource prerequisites.

The local ACTIVE/latest-USER predicates are checked again inside the BusinessTask T1 transaction
under the Conversation row lock (§8.2); a preflight read alone is not sufficient. Existing-task
receipt replay is resolved before these first-create checks.

Knowledge owner adds one narrow metadata contract:

```text
KnowledgeSelectionObserver.ObserveSelection(scope, base_id)
  -> SelectionRevisionSetRef{BaseID, Digest}
```

The digest is computed by the Knowledge owner over the current active/readable set, sorted by
Source ID, including Base fence, Source ID + fence, current-readable Revision ID and parsed-text
content digest. The observer returns no document text.

For Chat-backed execution, `knowledge.ContextRequest` also carries the proposal's
`ExpectedRevisionSetDigest`. The existing Knowledge `Materialize` transaction, while holding
the same Base/Source lifecycle locks used to choose readable revisions, recomputes the
revision-set digest and rejects a mismatch with `KNOWLEDGE_SELECTION_CHANGED` **before** it
creates/adopts a bundle. The expected digest participates in the materialization fingerprint.
Existing non-Chat Product Agent callers may omit the expected digest and keep their current
“materialize current readable set” semantics.

Therefore there is no observe→materialize TOCTOU: the proposal stores the observed identity for
display/staleness, and the actual Knowledge owner enforces that exact identity while selecting
the revisions that enter the bundle.

Any material difference returns `PROPOSAL_STALE`, `KNOWLEDGE_SELECTION_CHANGED` or the
current specific authorization/availability error. The server never silently rewrites the
proposal to “latest”.

Template default Knowledge remains only UI prefill. If the proposal has no Knowledge selection,
execution remains no-Knowledge.

## 8. BusinessTask contract

### 8.1 Canonical durable facts

```text
BusinessTask {
  task_id
  organization_id
  owner_user_id
  conversation_id
  source_message_id
  proposal_id
  proposal_digest
  confirmation_fingerprint
  kind                  product.title.optimize
  title
  goal_summary
  operation_id
  product_key
  target_platform
  agent_id
  agent_version
  execution_request_key
  configuration_snapshot_ref   # also binds execution model profile, §6.3
  context_snapshot_ref?
  execution_request_digest
  created_at
}
```

BusinessTask owns **business intent and exact handoff identity**. It does not persist copies of
Agent phase, step, retry count, Review state or Apply state.

V1 is exactly one BusinessTask -> one Product Agent execution identity. Multi-run orchestration
is Later.

### 8.2 Confirmation and handoff ordering

The explicit confirm operation uses its `Idempotency-Key` as the Product Agent
`Request.Key`. Its confirmation fingerprint is the exact
`(conversation_id, proposal_id, execution_request_key)` **wire request identity**. The
immutable `proposal_digest` is stored on the BusinessTask and integrity-checked only on the
first-create path; same-key replay therefore does not need to load or freshness-check the
proposal before it can identify the existing receipt.

**Replay lookup precedes proposal/dependency freshness checks.**

```text
fresh verified identity / Effective Organization / workbench.chat.use
  → lookup BusinessTask by (org, actor, execution_request_key)
      found:
        compare stored confirmation_fingerprint
          mismatch → IDEMPOTENCY_CONFLICT
          match    → return the existing BusinessTask + current safe projection
                     (no proposal-stale / Agent-enabled / template / Knowledge preflight)
      not found:
        → verify Conversation ACTIVE + proposal matches latest USER/exact source
        → existing Product + workbench.agent.use + listingkit.admin.write authorization
        → existing AgentConfig Prepare using exact proposal/model profile + request key
        → existing Knowledge Materialize with ExpectedRevisionSetDigest if selected
        → complete-prompt governed Quote preflight (no reservation/provider send)
        → build exact agent.Request and digest
        → T1 create/adopt BusinessTask under local confirmation ordering below
        → only a new T1 receipt continues to T2 existing Runtime.Start
             → existing AgentConfig Guard
             → existing Store.Claim
             → current Eino runtime + governed eino-ext model access + current tools
```

The existing-task replay still freshly authorizes the caller as the original
Organization/actor and `workbench.chat.use`/task ownership. It does **not** require the Agent,
template or Knowledge to remain executable merely to return an already committed receipt.
Protected Product/Knowledge/Review details in the projection are independently reauthorized and
redacted when unavailable.

T1 uses a Workbench-local transaction only. Serialize the scoped execution key using the
existing PostgreSQL transaction-lock pattern, then lock the Conversation row. Recheck the
same-key Task receipt first: a concurrent winner is fingerprint-compared and returned, never
reclassified as stale. When no receipt exists, verify the proposal's immutable org/owner/source
binding, Conversation ACTIVE and latest USER sequence again, then insert Task. Message append
and archive use the same Conversation row lock, so the ordering is observable and deterministic:

- later USER append/archive commits first → first confirmation fails before Task/Agent Claim;
- T1 commits first → the user's confirmation is durable; a subsequent message/archive cannot
  erase it or invalidate same-key receipt replay;
- concurrent same-key confirmation → one T1 insert; losers return that receipt with zero T2 work.

No external owner call, provider I/O or Knowledge materialization occurs while holding these
Workbench locks. Configuration/Knowledge snapshots already prepared before a rejected T1 remain
in their existing owners; they do not authorize a Task or dispatch. The existing Agent/Knowledge/
model gates still freshly protect T2 and every model handoff. This local ordering is not a
cross-database transaction or a promise to freeze all external facts at the T1 commit instant.

Why T1 and T2 are deliberately **not** one transaction:

- a confirmed business task that fails before Agent Claim is a truthful user-visible failed
  attempt, not database corruption;
- making Task creation part of `agentconfig.Guard` would invert ownership and couple product
  lifecycle to runtime persistence;
- exact request key + immutable proposal/config/Knowledge refs make retry safe without a Saga;
- no cross-owner transaction or background reconciler is needed.

T1 has a unique `(organization_id, owner_user_id, execution_request_key)` identity and stores
both the wire confirmation fingerprint and the exact proposal/request digests.

Same key + same exact fingerprint adopts the same BusinessTask. Same key + changed proposal or
execution identity is `IDEMPOTENCY_CONFLICT`.

### 8.3 Crash / lost-response semantics

| Failure point | Required behavior |
| --- | --- |
| before config/Knowledge preflight completes | no BusinessTask, no Agent Claim/provider work |
| after immutable config/Knowledge refs, before T1 | no BusinessTask; same confirm key adopts exact refs on retry, subject to first-create freshness |
| later USER message or archive wins before T1 | no BusinessTask/Agent Claim; return stale/archived, preserving prepared owner refs without dispatch |
| after T1, before Agent Claim | BusinessTask remains; projection = `ERROR / START_NOT_CLAIMED`; explicit retry-start may continue exact handoff |
| Agent Claim committed and terminal Commit exists, HTTP response lost | same-key confirm/task read resolves exact terminal run; no duplicate run |
| process crashes after Agent Claim while durable row is still RUNNING | never call Start again for that claimed run. Before the original run deadline + grace, project RUNNING/uncertain. After the bound, use the Agent owner's stale-running finalizer (§8.5); terminalize to `execution_outcome_unknown` with zero redispatch. |
| Guard rejects activation/template/ceiling after T1 | task remains ERROR with safe reason; no Claim/provider work |
| model outcome UNKNOWN after Claim | Agent owner remains UNKNOWN authority; task projects ERROR/unknown, no Chat retry owner |
| process restart | Conversation/Task survive; exact Task refs reconstruct the same Request and existing Store remains run authority |

No automatic background retry/reconciler is introduced.

### 8.4 Explicit start/reconcile action

Provide one bounded action:

```text
POST /api/v1/workbench/tasks/{task_id}/start
```

It accepts no replacement Product/template/Knowledge selection and no new execution key.

The server fresh-authorizes the exact Task scope, rebuilds/verifies the exact stored Request, and
then branches on the current Agent owner:

1. **no exact AgentRun exists**: call existing `Runtime.Start`; all current Guard checks still
   apply;
2. **exact AgentRun is terminal/interrupted/review-required**: return its current projection;
   never call Start again;
3. **exact AgentRun is RUNNING and still inside its original deadline + grace**: return RUNNING /
   operation-in-progress; zero redispatch;
4. **exact AgentRun is RUNNING past deadline + grace**: call the Agent stale-running finalizer
   in §8.5; zero redispatch.

Changing scope/config requires a new Chat proposal and new BusinessTask.

### 8.5 Existing Agent owner: stale RUNNING terminalization

The current Runtime persists `RUNNING` in `Store.Claim` before synchronous graph execution and
persists its terminal/checkpoint state only at final `Store.Commit`. A process crash can
therefore leave a durable RUNNING row forever. Slice E does not reinterpret that row as safe to
resume or Start again.

Add one bounded owner operation to `internal/agent` / existing PostgreSQL Agent store:

```text
StopReason:
  execution_outcome_unknown

RunningFinalizer.FinalizeExpiredRunning(
  scope, exact binding, request_key, observed_revision, now
) -> Record
```

Contract:

- it never invokes model/tools/provider and never reconstructs an Eino graph;
- it is eligible only when the exact durable row is still `RUNNING`;
- `now` must be later than the persisted `Deadline + 30s` commit grace;
- it locks/CASes the same run row at the observed revision;
- it preserves RunID, Scope, Request, Fingerprint, StartedAt, Deadline and all durable fields;
- it writes `STOPPED / execution_outcome_unknown`, increments Revision and stores no checkpoint;
- it does not invent a `PendingInvocationID` or provider outcome that was never durable;
- if the normal Runtime terminal Commit won first, the finalizer returns/adopts that terminal
  record;
- if the finalizer wins, any late Runtime Commit loses CAS and cannot rewrite the terminal fact.

This is a minimal repair of the existing Agent owner's crash window, not a Workbench recovery
engine. It is triggered only by an explicit Task start/reconcile action (and may also be tested
directly); V1 adds no background scanner/reconciler.

A task terminalized this way is `ERROR / EXECUTION_OUTCOME_UNKNOWN`. The same BusinessTask may
never execute again because provider/tool side effects during the lost process are not safely
known. The user must create/confirm a **new BusinessTask/new execution key** if they decide to
try the business goal again. Existing AI invocation/provider recovery, when available, remains
its own owner and does not grant Agent redispatch.

## 9. BusinessTaskProjection — no second runtime state machine

The Task Center state is computed from BusinessTask + exact AgentRun + Product Review facts.

Projection enum:

```text
RUNNING
WAITING_CONFIRMATION
COMPLETED
ERROR
PAUSED
```

Deterministic precedence:

1. exact Review `applied` -> COMPLETED / applied;
2. exact Review `rejected` -> COMPLETED / rejected;
3. exact Review `pending|accepted` -> WAITING_CONFIRMATION;
4. Agent `human_review_required` -> WAITING_CONFIRMATION;
5. Agent `interrupted` -> PAUSED;
6. Agent `running` and now <= persisted Deadline + 30s grace -> RUNNING;
7. Agent `running` past Deadline + grace -> ERROR / EXECUTION_OUTCOME_UNKNOWN with only the
   explicit start/reconcile action; projection itself does not mutate the run;
8. Agent `stopped` -> ERROR with safe stop-reason mapping;
9. BusinessTask exists but no exact AgentRun -> ERROR / START_NOT_CLAIMED;
10. owner facts are corrupt/unavailable -> projection unavailable; never invent success.

An accepted Review without Apply remains WAITING_CONFIRMATION because the explicit Product write
has not happened.

Agent `StopUsageUnknown` / `StopModelUnknown` remains ERROR with an “outcome unknown” action
contract; Task code does not reinterpret it or dispatch again.

Projection freshness never authorizes execution.

### 9.1 Review correlation

Task projection must not SQL-join Product Review storage.

Add a narrow Product Review read port owned by `internal/product/review` that can resolve the
existing idempotent Agent review operation identity `agent:<run_id>` to its current View when
one exists. The Product Review owner remains the only interpreter of Review persistence.

No Review schema ownership moves to Workbench.

## 10. Resume / Review actions

BusinessTask UI may expose actions, but no alternate executor is created.

- retry-start -> §8.4 exact handoff action;
- Resume -> delegates the existing Product Agent Resume path with fresh authorization,
  exact run/request and expected Agent revision;
- Submit Review -> delegates existing Product Agent review creation;
- Accept/Edit/Reject/Apply -> existing Product Review owner/routes.

Every execution action still requires all existing gates, including `workbench.agent.use`,
`listingkit.admin.write`, exact Product binding, Agent admission, Knowledge fences and current
AI/resource rules.

Task read permission does not imply any execution/action permission.

## 11. Permissions and isolation

Additive permissions:

| Permission | Meaning |
| --- | --- |
| `workbench.chat.read` | Read own Conversations/messages in current Organization. |
| `workbench.chat.use` | Create/modify own Chat and request planning/confirmation. |
| `workbench.task.read` | Read own BusinessTask projections in current Organization. |

Initial role mapping should follow current workbench conventions:

- read: current viewer/operator/admin/platform-admin roles;
- use: current operator/admin/platform-admin roles;
- task read: current viewer/operator/admin/platform-admin roles.

These permissions do not imply each other at code level; Casbin policy is explicit.

Additional requirements:

- Organization comes from verified Effective Organization, never request body;
- V1 Chat/Task is exact original actor scope;
- configure/admin status does not grant cross-member Chat/Task visibility;
- task projection must freshly authorize protected Product/Knowledge/Review details before
  emitting them;
- if a linked protected owner becomes unreadable, return safe unavailable/redacted projection,
  not cached names/excerpts;
- A→B→A Organization switching must generation-fence late browser responses.

## 12. HTTP contract

Candidate V1 API:

```text
GET    /api/v1/workbench/chat/conversations
POST   /api/v1/workbench/chat/conversations
GET    /api/v1/workbench/chat/conversations/{conversation_id}
PATCH  /api/v1/workbench/chat/conversations/{conversation_id}
POST   /api/v1/workbench/chat/conversations/{conversation_id}/messages
POST   /api/v1/workbench/chat/conversations/{conversation_id}/proposals/{proposal_id}/confirm

GET    /api/v1/workbench/tasks
GET    /api/v1/workbench/tasks/{task_id}
POST   /api/v1/workbench/tasks/{task_id}/start
POST   /api/v1/workbench/tasks/{task_id}/resume
POST   /api/v1/workbench/tasks/{task_id}/review
```

Rules:

- mutations require canonical UUID Idempotency-Key except pure metadata PATCH, which uses
  If-Match/ETag CAS and a bounded audit receipt;
- strict JSON with unknown fields rejected;
- GET has no model/provider/quote/reservation side effect;
- request body <= 16 KiB unless a stricter per-route bound applies;
- response <= 256 KiB; pagination default 20, max 50;
- no raw provider error/prompt/credential;
- no browser-supplied Organization/actor;
- no automatic different-key retry after timeout/UNKNOWN/conflict;
- cross-org/other-user exact lookup is unknown-equivalent where disclosure matters;
- all responses are private/no-store.

Stable Workbench errors include:

```text
INVALID_REQUEST
FORBIDDEN
NOT_FOUND
REVISION_MISMATCH
IDEMPOTENCY_CONFLICT
CONVERSATION_ARCHIVED
PLANNER_UNAVAILABLE
PLANNER_OUTCOME_UNKNOWN
PROPOSAL_STALE
TASK_START_NOT_CLAIMED
DEPENDENCY_UNAVAILABLE
```

Existing Product Agent/Knowledge/Review errors retain their current owner semantics.

## 13. Schema shape and indexes

Candidate tables:

```text
ai_workbench.conversations
ai_workbench.messages
ai_workbench.commands
ai_workbench.execution_proposals
ai_workbench.business_tasks
```

Required identities/constraints:

- conversations: PK UUID; index `(org, owner, lifecycle, updated_at desc, id)`;
- messages: unique `(org, owner, conversation_id, sequence)`, unique message UUID;
  indexed latest-USER lookup on `(org, owner, conversation_id, author_kind, sequence desc)`;
- commands: PK `(org, actor, idempotency_key)`, unique command UUID;
- proposals: UUID + immutable digest; FK to exact Conversation/user+assistant messages;
- tasks: UUID; unique `(org, owner, execution_request_key)`; immutable confirmation fingerprint;
  FK proposal/conversation;
- all payload columns have DB byte bounds;
- every FK includes Organization/owner qualification where it prevents accidental cross-scope
  linkage;
- no FK into Agent/Review/Knowledge/Product schemas; those are opaque owner references.

No mutable “current task status” column is required in V1.

## 14. Task Center cutover

Current source-specific pages remain truthful while BusinessTask rolls out.

V1 changes:

- `/workbench/ai/tasks`: BusinessTask list becomes the primary list;
- `running`, `errors`: connect only to BusinessTask projection;
- `pending`: primary section = BusinessTasks in WAITING_CONFIRMATION;
- `completed`: primary section = BusinessTasks in COMPLETED.

Existing current projections are retained temporarily as explicitly labeled secondary sections:

- Product Review proposals with no BusinessTask linkage: “其他待确认提案”;
- completed local-preparation records with no BusinessTask: “历史已完成工作记录”.

They keep their existing APIs/types and are **not** assigned fake BusinessTask IDs.

Removal condition:

> after every still-supported producer that belongs in AI Workbench creates BusinessTask at
> intent time, and its source-specific UI has an admitted replacement, remove the secondary
> section/client in that producer's own cutover. No backfill.

The old generic Task-first Product UI remains RETIRE and is never a coexistence source.

## 15. Chat UI semantics

Connect:

- `/workbench/ai/chat`;
- `/workbench/ai/chat/new`;
- `/workbench/ai/chat/recent`;
- `/workbench/ai/chat/saved`.

Before a READY proposal can be confirmed, display real current facts:

- current enterprise;
- selected Product/work scope;
- action: title suggestion only;
- target platform;
- exact template or none;
- selected Knowledge or explicit none;
- admitted execution model/profile and current capability/limit summary, without credentials;
- Human Review required before Apply.

Assistant prose is never treated as the execution contract. The typed proposal card is.

Task detail displays:

- user goal/title;
- current projected state + reason;
- exact Product/platform;
- current next action;
- Agent summary/usage status and actual model identity from authorized owner metadata;
- Review/result when currently authorized;
- protected Knowledge citations through existing safe projector only.

No fake task count, progress percentage, elapsed estimate, Store association or success state.

## 16. Failure / replay matrix

| Scenario | Required outcome |
| --- | --- |
| duplicate Conversation create | same key/same fingerprint replays same Conversation |
| same key changed create/message | 409 IDEMPOTENCY_CONFLICT |
| duplicate user message | one USER message; deterministic planner invocation; existing AI ClaimInvocation grants at most one provider attempt |
| crash after USER/command commit but before AI Claim | exact AI invocation absent proves no dispatch; same-key replay may safely claim once with original admissible input/profile/deadline |
| concurrent planner request loses AI Claim | zero provider send; returns PENDING/terminal replay from same invocation identity |
| planner no-dispatch failure | durable user message + failure receipt; no assistant/proposal |
| planner invocation remains dispatched past deadline/grace | durable user message + PLANNER_UNKNOWN; no automatic re-dispatch |
| terminal AI fact but assistant/proposal commit was lost | bounded PENDING then PLANNER_UNKNOWN; provider output is not fabricated or re-sent |
| route/model/pricing changes after command or execution proposal preparation | retain exact profile for replay; no automatic substitution; new execution must satisfy current gates or require new confirmation |
| SDK retries after a first generation attempt | transport gate prevents an additional send; first-attempt UNKNOWN is preserved |
| raw usage missing or not representable by admitted mapping | no zero-cost inference; existing invocation/Resource owner retains UNKNOWN |
| same-org member credential exists but organization row is missing/disabled | no member/global/environment fallback; both consumers reject before transport |
| native protocol provision/read-back | validate through admitted native adapter policy, not compatible-only Manager; same private writer and organization row |
| wrong organization credential, model override or tool-bound Planner instance | reject before transport; no implicit environment/global credential fallback |
| ASSISTANT response appends its READY proposal | proposal still matches latest USER and remains confirmable; its own assistant sequence does not make it stale |
| new USER message after READY proposal | old proposal becomes PROPOSAL_STALE |
| delayed assistant response to an older USER turn | retains original source identity; cannot become latest-user proposal |
| USER append/archive commits between preflight and T1 | final row-locked T1 check rejects with zero Task/Agent Claim |
| concurrent same-key confirm wins before another T1 freshness check | loser replays the committed Task before stale predicates; no second T2 execution |
| selected Knowledge readable revision set changes before confirm | Knowledge Materialize rejects expected revision-set digest before BusinessTask |
| first-time Agent/template/Knowledge changes before confirm | fail before BusinessTask or return stale/owner error |
| same-key confirm after BusinessTask already committed | existing Task/fingerprint is resolved before proposal freshness; return receipt/projection even if later message/disable/change occurred |
| task T1 commit then process crash before Claim | task projects START_NOT_CLAIMED; explicit start uses same exact handoff |
| Guard rejects after T1 | task ERROR; zero new provider work |
| Agent terminal Commit exists then HTTP response lost | task resolves same terminal AgentRun; retry adopts receipt |
| process crash after Agent Claim leaves RUNNING | no Start/Resume redispatch; RUNNING until deadline+grace, then explicit Agent-owner CAS finalizes execution_outcome_unknown |
| Agent interrupted | PAUSED; existing Resume action only, using the frozen execution profile |
| Review pending/accepted | WAITING_CONFIRMATION |
| Review rejected | COMPLETED / rejected |
| Review applied | COMPLETED / applied |
| provider/usage unknown | ERROR / unknown; no Workbench retry owner |
| Conversation archived with active task | blocks new Chat/confirm; task remains readable/actionable by its own auth |
| permission/Organization loss | protected Chat/Task hidden/denied; no cached authority |
| A→B→A late response | old generation discarded |
| app restart | durable Chat/Task + exact owner refs rebuild current projection |

## 17. Rollout and rollback

Greenfield:

- install `ai_workbench` schema explicitly;
- create `ai_workbench_runtime` with only admitted DML;
- no rows are synthesized from existing Review/completed work;
- qualify/pin selected eino-ext component versions and their guarded transport/usage mappings;
- mount Chat/BusinessTask routes only when dependencies are complete.

Fail closed:

- missing schema/pool/planner/Agent D or qualified model-profile dependency keeps execution unavailable;
- Product Agent runtime/domain/Review semantics remain unchanged except the explicitly proposed
  model-profile wiring and existing-owner crash-window repair in this document;
- direct and Chat-originated title calls must not bypass the new shared model gate;
- no fallback to legacy Task UI, replaced model consumer or generic Task table.

Rollback:

- disable/unmount new Chat/BusinessTask and affected execution admission before switching code;
- preserve Conversation/BusinessTask/AgentConfig/AgentRun/Review rows and exact model provenance;
- never replay a portable-model invocation through an older unprofiled implementation;
- preserve the previously frozen hard-ceiling rollout fence; mixed deployment does not make
  new model/ceiling contracts atomically effective;
- existing Product Agent/Review facts remain authoritative;
- do not delete or translate tasks to legacy records;
- later re-enable reads the same durable Workbench facts.

## 18. Threat model and accepted risk

Must address:

- cross-tenant/actor Conversation or Task disclosure;
- natural language becoming implicit Tool/side-effect authority;
- stale proposal executing a different Product/template/Knowledge/model selection;
- duplicate paid planning/model dispatch through application, SDK or transport retry;
- route/credential/pricing drift and hidden cross-provider fallback;
- lost/misinterpreted provider usage or sensitive callback export;
- duplicate AgentRun under confirm retry;
- cached Product/Knowledge/Review content surviving role/Organization loss;
- BusinessTask becoming a second workflow/retry state machine;
- source-specific current projections being falsely relabeled as BusinessTask.

No new Accepted Risk is asserted by this draft.

V1 intentionally does not provide shared conversations, cross-member administrator task reading,
autonomous agent selection or background task-start recovery.

## 19. Verification plan

Implementation must use existing test infrastructure; do not build a new verification platform.

| Test group | Required evidence |
| --- | --- |
| Domain contracts | Conversation append-only/CAS/bounds; proposal source binds originating USER, assistant append does not stale it, later USER does; immutable digest; Task exact identities. |
| PostgreSQL | real isolated PG schema/ACL; org+actor isolation; command idempotency; concurrent sequence allocation; final confirmation versus USER append/archive ordering; same-key T1 winner adoption. |
| Planner | deterministic pre-dispatch invocation identity; concurrent same-key claim; crash before/after AI Claim; no tools; max one provider attempt/key; no-dispatch vs PENDING/UNKNOWN; new planning operation through current AI invocation + ResourceAIPoint owners; strict structured output. |
| Provider portability | same Planner and title-model adapters run through two actual eino-ext protocol implementations using isolated synthetic transports; no GRSAI/fixed-model consumer checks; exact component/core/module versions recorded. |
| Credential/assembly reuse | #580 organization-only and restricted writer behavior preserved for both consumers; missing org row cannot use member/environment credentials; native provision read-back never reaches compatible-only resolver; readiness and execution select the same capability policy. |
| Transport and usage | underlying send count remains at most one through timeout/429/5xx/SDK retry/redirect; missing raw counters stay unknown; cache/reasoning mapping and invalid-output billing; final auth/config/Knowledge permit at real handoff; no prompt/secret callback export. |
| Profile replay | config switch affects new commands only; prepared/claimed invocation and resumed Agent use original profile or fail closed; no cross-org model-instance reuse; no free or alternate-provider fallback. |
| Knowledge proposal fence | revision-set observer digest; source promotion after READY; Materialize expected-digest check under lifecycle locks; no-Knowledge unchanged. |
| Confirm | existing-task replay before stale checks, including in T1 after a concurrent winner; changed same-key conflict; stale Product/Agent/template/Knowledge/model profile on first confirm; exact configuration refs; T1 task then T2 Claim crash windows. |
| Agent integration | both direct and Chat-originated title paths use shared governed model integration; no duplicate run; crash after Claim leaves RUNNING; before deadline no redispatch; after grace stale-running CAS finalizer; late normal Commit race; ceiling/disable/Knowledge fences unchanged. |
| Projection | exact precedence for running/interrupted/review states/stopped/no-run/UNKNOWN; protected detail redaction. |
| Review | run -> existing review operation correlation; pending/accepted/rejected/applied mapping; Apply remains current owner. |
| HTTP/RBAC | strict JSON/limits/ETag/idempotency; other actor/org unknown-equivalent; read vs use vs execution permissions. |
| UI | new/recent/favorite; explicit proposal card/model identity; no-Knowledge; delayed assistant cannot attach to a newer USER turn; Task filters; A→B→A late-response fence; desktop/narrow. |
| Cutover | source-specific pending/completed remain labeled non-BusinessTask; no legacy Task dependency/backfill; current titletext behavior extracted before replacing its consumer, without losing valid safety behavior. |
| Restart | durable Conversation/Task, no-run retry-start, claimed-run read and Review projection after restart. |

Real customer data, paid provider trials, production deployment and final user acceptance remain
NOT_RUN unless separately authorized. Component compile/runtime/contract checks above are
implementation obligations, not tests executed by this documentation change.

## 20. Architecture admission checklist

- [x] Product outcome and one executable V1 kind are bounded.
- [x] Conversation/messages idempotency has a deterministic durable pre-dispatch planner identity and bounded PENDING/UNKNOWN replay.
- [x] Provider-neutral Planner and title-model paths specify contract → current AI invocation/resource owners → Eino/eino-ext integration → injection → consumers.
- [x] Current #580 title policy, organization-only credential/provisioning and readiness contracts are explicitly reused; native-protocol and run-profile extensions are distinguished from implemented compatible-protocol behavior.
- [x] Frozen model-profile identity, no-tool instance isolation, guarded transport and usage normalization obligations are specified.
- [x] ExecutionProposal source freshness compares latest USER, with local row-locked T1 recheck and existing-receipt precedence.
- [x] ExecutionProposal exact/stale semantics include Knowledge readable revision-set identity enforced by the Knowledge owner.
- [x] BusinessTask fact boundary is intent/handoff only, not runtime lifecycle.
- [x] Same-key confirmation resolves an existing BusinessTask before first-time freshness checks.
- [x] BusinessTask → Agent Start crash/replay includes the existing Agent Claim-before-Commit crash window and non-redispatching stale-running finalization.
- [x] deterministic Task projection precedence is frozen.
- [x] Product Review correlation stays with Review owner.
- [x] Task Center cutover preserves truthful current source projections without migration.
- [x] tenant/actor/RBAC and protected-result behavior are frozen.
- [x] HTTP/schema/package/role/rollout contracts are bounded.
- [x] greenfield/legacy decisions are explicit.
- [x] risk-matched implementation test matrix is defined.
- [ ] Independent Architecture Review completed and findings classified/resolved, including the provider-portability increment.
- [ ] Exact final HEAD applicable CI completed.
- [ ] Explicit `APPROVED / IMPLEMENTATION_READY` admission recorded.
- [ ] Architecture PR merged to main before production Writer starts.

## 21. Delivery after admission

Default implementation delivery is one user-result batch:

```text
Conversation
→ governed no-tool planning through Eino/eino-ext
→ typed proposal
→ confirm exact configuration/model profile
→ BusinessTask
→ current Product title Agent with portable governed model access
→ Task Center
→ current Human Review / Apply
```

Conversation tables, Task tables, model-adapter cutover, API and UI are internal milestones,
not separate products or automatic separate PRs. Split only for an independently usable result
or independent lifecycle/risk.

Project Center and Report follow this closed loop and consume references/results; they do not
become execution owners.

## 22. Authorization boundary

This D0 authorizes documentation, read-only investigation, an independent architecture branch/PR
and review maintenance within #576. The user-approved provider-portability requirement changes
the candidate design, not deployment permissions. This does not authorize production code/schema
mutation, architecture merge, Issue closure, real business data or paid provider calls.
