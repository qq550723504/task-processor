# AI Workbench Chat + BusinessTask V1 — Slice E

> Status: **DRAFT / NOT_IMPLEMENTATION_READY**
>
> Design Basis: **Independent Architecture**.
> Issue: #576. Product parent: #298.
> Upstream product/architecture: #555, #570.
> Delivered implementation dependency: #573 / PR #574.
> Inspected baseline: `main @ df41c7863d7b152d7e826753d08c52810f5dc9df` (2026-09-29).
> Matching #574 push/main CI `36593853258`: **SUCCESS**.
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
  → Chat/Task detail show current truthful result
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
- BYOK/custom endpoint management;
- image Agent orchestration;
- platform remote write/publish;
- embeddings/vector search;
- temporary attachment auto-save to enterprise Knowledge;
- shared/team conversations;
- enterprise-wide task visibility;
- new Temporal/queue/retry/Saga/recovery platform;
- new IAM/Review/audit/billing owner;
- legacy Task migration/backfill/dual-write/synchronization.

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
aiworkbench
  -> narrow AgentConfig/Product/Review/Agent read/action ports
  -> planning port

integration/persistence/aiworkbench
  -> aiworkbench contracts

integration/aiworkbench/<provider adapter>
  -> existing AI Capability/provider/usage owners

app/httpapi
  -> aiworkbench/httpapi + concrete adapters
```

The generic `internal/agent` package never imports `aiworkbench`.

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

Sequence allocation locks the Conversation row and monotonically increments `next_sequence`.
A metadata-only favorite/title operation does not change message sequence and therefore does not
invalidate an execution proposal.

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
  planner_invocation_id     # deterministic SHA-256 over scoped command identity
  planner_input_hash
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

The durable dispatch algorithm is:

```text
T0 transaction:
   create/adopt USER message + PlanningCommand(READY_TO_DISPATCH, deterministic invocation)
→ outside transaction call Planner with that exact invocation identity
→ Planner atomically ClaimInvocation in the existing AI invocation owner
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
  same-key replay may safely attempt the existing AI `ClaimInvocation`;
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

## 6. Chat planner contract

### 6.1 No tools and no execution authority

Planning is:

```text
bounded Conversation text
+ fixed capability description
+ explicit user-selected work scope
  -> governed planning model
  -> Clarification | ReadyPlan
```

The planning model receives **no Commerce Tool Registry**, no arbitrary tool schema and no
execution credential. It does not invoke AgentRuntime or Product mutation.

V1 explicit work scope is browser-selected then server-authorized:

```text
operation_id          # existing acquisition/Product binding identity
target_platform       shein | temu | amazon
template_selection?   exact template_id + revision, or explicit no-template
knowledge_selection?  exact KnowledgeBase ID, omission = no Knowledge
```

The model is not allowed to select Product, Organization, Agent ID, Tool, template ID,
KnowledgeBase ID or platform. Those are explicit user/server facts.

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

### 6.2 AI governance — exact existing-owner extension

Implement `aiworkbench.Planner` as a narrow provider-neutral port. Do **not** reuse
`grsaitext.AgentTextModel`: that adapter is deliberately AgentRun/tool-shaped and rejects an
empty Tool set.

The minimal admitted extension is:

```text
internal/aicapability
  + CapabilityAIWorkbenchChatPlanning = "aiworkbench.chat_planning"
  + OperationAIWorkbenchChatPlan      = "aiworkbench_chat_plan"
  + existing InvocationDispatchClaimer / Recorder / ReplayReader
  + existing usage reservation/settlement

internal/aicapability/store
  GormInvocationRecorder
    → same ai_invocations table
    → ClaimInvocation / FindInvocation / RecordInvocation
    → priced-text validation accepts exactly:
         product_agent_decision
         aiworkbench_chat_plan
    → planner operation requires AgentRunID == "" and BusinessTaskID == ""
      while existing Product Agent operation still requires its real AgentRunID
    → observed-usage structured-output failure is admitted for planner so real token usage
      is billed even when PlanningDecision JSON is invalid

internal/integration/orgresource
  GormModelInvocationRepository
    → same ResourceAIPoint/member-limit owner
    → ReadModelInvocation accepts only the two admitted priced-text operations
    → Product Agent reservation BusinessScope = real AgentRunID
    → Chat planner reservation BusinessScope = "chat-plan:" + invocation_id
    → existing reserve/finalize/idempotency/resource events remain unchanged

internal/integration/aiworkbench/grsaitext
  Planner
    → uses existing openai.Manager + Organization credential resolver
    → uses the same admitted text route/model and a trusted PlannerPolicy
    → no Eino, no Commerce Tool definitions, no Knowledge reader
    → ClaimInvocation
    → existing ResourceAIPoint reservation
    → one CompleteText transport
    → RecordInvocation terminal observation/usage
    → strict bounded PlanningDecision

internal/app/httpapi current application assembly
  existing AI manager/recorder/resource adapter + fresh identity
    → aiworkbench Planner adapter
    → aiworkbench Service
    → aiworkbench/httpapi
```

`PlannerPolicy` is trusted deployment configuration, not browser/model input. It freezes the
same style of admitted route, point tariff, currency cost estimate, input/output bounds,
configuration version and code-owned prompt version used by the current governed text path. V1
may use the same configured provider/model as the Product title Agent, but it has a distinct
operation/capability/prompt identity and cannot inherit Agent tools.

The planner's exact `InvocationRecord` binds:

- deterministic Workbench `planner_invocation_id`;
- current Organization/user/effective member;
- `OperationAIWorkbenchChatPlan`;
- `CapabilityAIWorkbenchChatPlanning`;
- exact `planner_input_hash`, prompt/policy/configuration versions and admitted route;
- frozen point tariff and token maxima;
- the T0 `planner_started_at`.

This is enough for the existing AI invocation and Resource owners to reject identity drift and
to reserve/settle the same enterprise/member point pool without inventing a Chat ledger.

The adapter must preserve:

- one provider send maximum per message command;
- no silent retry/fallback;
- current Organization-scoped credential resolution;
- current route/configuration recheck before transport;
- conservative ResourceAIPoint reservation before transport;
- observed usage settlement or existing durable dispatch UNKNOWN;
- current Organization/member accounting;
- bounded timeout/input/output;
- no provider-specific types in `aiworkbench`.

Planning inability never creates a BusinessTask or AgentRun.

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
  source_sequence
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
  knowledge_selection_ref? {
    knowledge_base_id
    revision_set_digest
  }
  created_at
}
```

The proposal is immutable.

A new user message makes an older proposal stale for confirmation because
`source_sequence` is no longer the latest Conversation message sequence. Metadata-only changes
(title/favorite) do not.

Before confirmation, the server freshly validates:

- current identity/effective Organization/owner;
- `workbench.chat.use`;
- exact Product binding + existing Product execution authorization;
- `workbench.agent.use`;
- existing `listingkit.admin.write`;
- Agent is still enabled and its activation epoch/revision has not invalidated the proposal;
- exact template is still active when present;
- exact optional Knowledge selection is currently readable/active **and has the same observed
  revision-set digest**;
- current AI/point/resource prerequisites.

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
  kind                  product.title.optimize
  title
  goal_summary
  operation_id
  product_key
  target_platform
  agent_id
  agent_version
  execution_request_key
  configuration_snapshot_ref
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
`(conversation_id, proposal_id, proposal_digest, execution_request_key)` request identity.

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
        → verify Conversation ACTIVE + proposal is latest/exact
        → existing Product + workbench.agent.use + listingkit.admin.write authorization
        → existing AgentConfig Prepare using exact proposal + request key
        → existing Knowledge Materialize with ExpectedRevisionSetDigest if selected
        → existing complete-prompt Quote preflight (no reservation/provider send)
        → build exact agent.Request and digest
        → T1 create BusinessTask
        → T2 existing Runtime.Start
             → existing AgentConfig Guard
             → existing Store.Claim
             → current runtime/model/tools
```

The existing-task replay still freshly authorizes the caller as the original
Organization/actor and `workbench.chat.use`/task ownership. It does **not** require the Agent,
template or Knowledge to remain executable merely to return an already committed receipt.
Protected Product/Knowledge/Review details in the projection are independently reauthorized and
redacted when unavailable.

Why T1 and T2 are deliberately **not** one transaction:

- a confirmed business task that fails before Agent Claim is a truthful user-visible failed
  attempt, not database corruption;
- making Task creation part of `agentconfig.Guard` would invert ownership and couple product
  lifecycle to runtime persistence;
- exact request key + immutable proposal/config/Knowledge refs make retry safe without a Saga;
- no cross-owner transaction or background reconciler is needed.

T1 has a unique `(organization_id, owner_user_id, execution_request_key)` identity and stores
the proposal/request fingerprint.

Same key + same exact fingerprint adopts the same BusinessTask. Same key + changed proposal or
execution identity is `IDEMPOTENCY_CONFLICT`.

### 8.3 Crash / lost-response semantics

| Failure point | Required behavior |
| --- | --- |
| before config/Knowledge preflight completes | no BusinessTask, no Agent Claim/provider work |
| after immutable config/Knowledge refs, before T1 | no BusinessTask; same confirm key adopts exact refs on retry |
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
- commands: PK `(org, actor, idempotency_key)`, unique command UUID;
- proposals: UUID + immutable digest; FK to exact Conversation/user+assistant messages;
- tasks: UUID; unique `(org, owner, execution_request_key)`; FK proposal/conversation;
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
- current capability/limit summary;
- Human Review required before Apply.

Assistant prose is never treated as the execution contract. The typed proposal card is.

Task detail displays:

- user goal/title;
- current projected state + reason;
- exact Product/platform;
- current next action;
- Agent summary/usage status;
- Review/result when currently authorized;
- protected Knowledge citations through existing safe projector only.

No fake task count, progress percentage, elapsed estimate, Store association or success state.

## 16. Failure / replay matrix

| Scenario | Required outcome |
| --- | --- |
| duplicate Conversation create | same key/same fingerprint replays same Conversation |
| same key changed create/message | 409 IDEMPOTENCY_CONFLICT |
| duplicate user message | one USER message; deterministic planner invocation; existing AI ClaimInvocation grants at most one provider attempt |
| crash after USER/command commit but before AI Claim | exact AI invocation absent proves no dispatch; same-key replay may safely claim once |
| concurrent planner request loses AI Claim | zero provider send; returns PENDING/terminal replay from same invocation identity |
| planner no-dispatch failure | durable user message + failure receipt; no assistant/proposal |
| planner invocation remains dispatched past deadline/grace | durable user message + PLANNER_UNKNOWN; no automatic re-dispatch |
| terminal AI fact but assistant/proposal commit was lost | bounded PENDING then PLANNER_UNKNOWN; provider output is not fabricated or re-sent |
| new user message after READY proposal | old proposal becomes PROPOSAL_STALE |
| selected Knowledge readable revision set changes before confirm | Knowledge Materialize rejects expected revision-set digest before BusinessTask |
| first-time Agent/template/Knowledge changes before confirm | fail before BusinessTask or return stale/owner error |
| same-key confirm after BusinessTask already committed | existing Task/fingerprint is resolved before proposal freshness; return receipt/projection even if later message/disable/change occurred |
| task T1 commit then process crash before Claim | task projects START_NOT_CLAIMED; explicit start uses same exact handoff |
| Guard rejects after T1 | task ERROR; zero new provider work |
| Agent terminal Commit exists then HTTP response lost | task resolves same terminal AgentRun; retry adopts receipt |
| process crash after Agent Claim leaves RUNNING | no Start/Resume redispatch; RUNNING until deadline+grace, then explicit Agent-owner CAS finalizes execution_outcome_unknown |
| Agent interrupted | PAUSED; existing Resume action only |
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
- mount Chat/BusinessTask routes only when dependencies are complete.

Fail closed:

- missing schema/pool/planner/Agent D dependency keeps Chat execution unavailable;
- existing Agent/Review/direct Product paths remain unchanged;
- no fallback to legacy Task UI or generic Task table.

Rollback:

- disable/unmount new Chat/BusinessTask routes;
- preserve Conversation/BusinessTask rows;
- existing Product Agent/Review facts remain authoritative;
- do not delete or translate tasks to legacy records;
- later re-enable reads the same durable Workbench facts.

## 18. Threat model and accepted risk

Must address:

- cross-tenant/actor Conversation or Task disclosure;
- natural language becoming implicit Tool/side-effect authority;
- stale proposal executing a different Product/template/Knowledge selection;
- duplicate paid planning/model dispatch under idempotent retry;
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
| Domain contracts | Conversation append-only/CAS/bounds; proposal immutable/digest/staleness; Task exact identities. |
| PostgreSQL | real isolated PG schema/ACL; org+actor isolation; command idempotency; concurrent sequence allocation; task same-key replay. |
| Planner | deterministic pre-dispatch invocation identity; concurrent same-key claim; crash before/after AI Claim; no tools; max one provider attempt/key; no-dispatch vs PENDING/UNKNOWN; new planning operation through current AI invocation + ResourceAIPoint owners; strict structured output. |
| Knowledge proposal fence | revision-set observer digest; source promotion after READY; Materialize expected-digest check under lifecycle locks; no-Knowledge unchanged. |
| Confirm | existing-task replay before stale checks; changed same-key conflict; stale Product/Agent/template/Knowledge on first confirm; config/Knowledge exact refs; T1 task then T2 Claim crash windows. |
| Agent integration | no duplicate run; crash after Claim leaves RUNNING; before deadline no redispatch; after grace stale-running CAS finalizer; late normal Commit race; ceiling/disable/Knowledge fences unchanged. |
| Projection | exact precedence for running/interrupted/review states/stopped/no-run/UNKNOWN; protected detail redaction. |
| Review | run -> existing review operation correlation; pending/accepted/rejected/applied mapping; Apply remains current owner. |
| HTTP/RBAC | strict JSON/limits/ETag/idempotency; other actor/org unknown-equivalent; read vs use vs execution permissions. |
| UI | new/recent/favorite; explicit proposal card; no-Knowledge; Task filters; A→B→A late-response fence; desktop/narrow. |
| Cutover | source-specific pending/completed remain labeled non-BusinessTask; no legacy Task dependency/backfill. |
| Restart | durable Conversation/Task, no-run retry-start, claimed-run read and Review projection after restart. |

Real customer data, paid provider trials, production deployment and final user acceptance remain
NOT_RUN unless separately authorized.

## 20. Architecture admission checklist

- [x] Product outcome and one executable V1 kind are bounded.
- [x] Conversation/messages idempotency has a deterministic durable pre-dispatch planner identity and bounded PENDING/UNKNOWN replay.
- [x] Planner no-tool path has an implementable contract → existing AI invocation/resource owners → adapter → application injection → consumer map.
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
- [ ] Independent Architecture Review completed and findings classified/resolved.
- [ ] Exact final HEAD applicable CI completed.
- [ ] Explicit `APPROVED / IMPLEMENTATION_READY` admission recorded.
- [ ] Architecture PR merged to main before production Writer starts.

## 21. Delivery after admission

Default implementation delivery is one user-result batch:

```text
Conversation
→ planning
→ typed proposal
→ confirm
→ BusinessTask
→ current Product title Agent
→ Task Center
→ current Human Review / Apply
```

Conversation tables, Task tables, API and UI are internal milestones, not separate products or
automatic separate PRs. Split only for an independently usable result or independent lifecycle/risk.

Project Center and Report follow this closed loop and consume references/results; they do not
become execution owners.

## 22. Authorization boundary

This D0 authorizes documentation, read-only investigation, an independent architecture branch/PR
and review maintenance within #576. It does not authorize production code/schema mutation,
architecture merge, Issue closure, deployment, real business data or paid provider calls.
