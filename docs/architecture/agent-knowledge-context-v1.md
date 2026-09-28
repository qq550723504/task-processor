# Agent + Knowledge Context V1

> Status: **ARCHITECTURE_REVIEW / NOT IMPLEMENTATION_READY**
>
> Product Design Issue: #555
>
> Parent Product Projection: #298
>
> Inspected baseline: `main @ e089ae08dcef9fbc853882293ca50ceb94e184d2` (2026-09-28).
>
> Figma authority: `tg48P46SSXl6TBy9lZwg63 / 31:463`.
>
> Figma revision candidates created for review:
> - `4703:429` — 我的智能体 / 企业状态语义 v1
> - `4703:572` — 我的知识库 / 企业内容状态 v1
> - `4703:694` — Chat 执行确认 / 权限与知识范围 v1
> - `4703:892` — 智能体能力配置 / 当前企业 v1
> - `4711:429` — 任务中心 / 知识引用 Human Review v1
> - `4713:429` — 品牌与表达规范 / 资料处理 v1
> - `4714:429` — POD 模板 / 默认知识不等于执行许可 v1
> - `4722:429` — 商品优化智能体 / 本次标题优化知识确认 v1
>
> Product Gate: **APPROVED FOR ARCHITECTURE REVIEW** under #555.
>
> This document is an Independent Architecture **review candidate only**. It does not authorize
> production code, schema, provider/model calls, real enterprise document uploads,
> deployment, or release enablement.

## 1. Product outcome

首期只证明一条真实、有边界的用户路径：

```text
当前企业
  -> 创建企业知识库
  -> 上传品牌/表达规范
  -> 资料处理完成并可预览
  -> 选择一个已经存在的商品
  -> 启动现有标题优化能力
  -> 本次主动引用企业知识
  -> 生成带来源引用的标题建议
  -> deterministic validation
  -> existing Human Review
  -> explicit Apply
```

目标不是一次性建设完整 Chat、Multi-Agent、训练平台、企业搜索、工作流编辑器或
RAG Platform，而是证明“企业资料可以被当前受控 Agent 正确、安全、可追溯地消费”。

## 2. Approved Product Gate

The product-design phase under #555 is frozen for this V1 Architecture Review with the
following decisions:

1. Agent enablement configuration, KnowledgeBases, templates and model-capability
   configuration are scoped to the current Effective Organization. Creator/updater identity
   remains audit metadata. Private/personal Knowledge space is Later.
2. Enabled Agent is a configuration state, never a perpetual AgentRun. Task/run state stays
   in the existing execution/task owners.
3. Knowledge is opt-in. A conversation/task explicitly selects it, or a template supplies a
   default selection that is re-authorized at execution time.
4. Temporary attachments are per-execution context by default. Saving them into enterprise
   Knowledge is a separate explicit action.
5. “Available/favorited”, “selected for this run”, and “saved as a template default” are
   distinct product actions.
6. Knowledge is supplemental untrusted context. It cannot become canonical Product fact,
   authorization, hidden system instruction, validator bypass or approval.
7. V1 keyword import uses **snapshot copy + original KnowledgeRevision provenance**, not a
   live reference that silently changes historical behavior.
8. V1 admitted document types are UTF-8 text, Markdown, PDF with extractable text and DOCX.
   Image-only/scanned OCR, PPT/XLS, arbitrary archives and VLM document understanding are
   Later.
9. Apache Tika 4.x is the preferred bounded document-parser candidate when PDF/DOCX remains
   in V1. It is private integration infrastructure, not a Knowledge owner. If V1 is narrowed
   to text/Markdown, do not deploy Tika for future compatibility.
10. V1 reuses the existing governed model capability. Arbitrary custom API URL/Key is Later.
11. Templates save default configuration, not permission. Every execution rechecks current
    Organization access, Knowledge access and applicable quota/budget, then freezes the
    exact Knowledge revision set actually used.
12. Human Review keeps canonical Product/source evidence separate from supplemental
    Knowledge provenance. If a human edits the title, the original AI citations remain
    audit/explanation for the AI proposal and must not be represented as supporting the
    human-edited text.
13. Slices A–C keep the current Product Agent admission and add only the minimum Knowledge
    read/manage permissions. Generic agent.use/agent.configure permissions are deferred to
    Slice D if persistent “我的智能体” management actually requires them.
14. The following Figma revision candidates are the Product Gate evidence for V1:
    `4703:429`, `4703:572`, `4703:694`, `4703:892`, `4711:429`,
    `4713:429`, `4714:429`, and `4722:429`.

These decisions are product scope for this architecture review. A reviewer may identify an
implementation blocker, security contradiction or impossible contract, but should not reopen
them merely as preference alternatives without a newly demonstrated blocker.

## 3. Current implementation reality

### 3.1 Existing reusable owners

当前 main 已存在并应复用：

| Capability | Current owner / evidence | V1 treatment |
| --- | --- | --- |
| Verified actor / Effective Organization | `internal/authidentity` + `internal/workbenchcontext` | reuse; no Knowledge IAM |
| Permission decisions | `internal/authz` + current HTTP route policies | reuse; knowledge permissions are new named decisions only if required |
| Agent execution / checkpoint | `internal/agent` + `internal/integration/agent/eino` | reuse; do not create another runtime |
| Agent tool allowlist | `internal/commercetool` | reuse for existing Product tools; **V1 Knowledge context does not enter through a Commerce Tool**. It uses the pre-`Start` local materializer/reader contract; model-directed Knowledge Tool retrieval is Later. |
| Governed model invocation | current AI Capability / Product Agent composition | reuse; knowledge does not call provider directly |
| Product facts | `internal/product/catalog` | canonical; knowledge cannot override |
| Source evidence | `internal/product/sourcing` | canonical source evidence; separate from enterprise knowledge |
| Approved assets | `internal/product/asset` | canonical; knowledge files are not Product assets |
| Candidate validation | `internal/product/enrichment` / current Agent validator | reuse |
| Human Review / Apply | `internal/product/review` | reuse; knowledge does not create a second approval system |
| Current Product Agent | `internal/app/httpapi/product_agent_application.go` / routes | first consumer to extend, not a universal Chat backend |

Current Product Agent already binds a run to verified Organization/actor, exact acquisition
publication/Catalog version, bounded tools, governed model, budget and current Product Review.

That path is the preferred first consumer.

### 3.2 Knowledge implementation is not currently delivered

At the inspected baseline there is **no concrete `internal/knowledge` implementation
directory** in main. Architecture documents reserve `internal/knowledge` as a target
domain root, but that target placement is not implementation evidence.

Therefore the first Knowledge slice needs a new current owner, persistence and HTTP/UI
contract. It must not wrap a hidden legacy Knowledge service or infer that a target folder
name means the product is already built.

### 3.3 Current UI capability state

Current Console navigation still marks:

- 硕米Chat: unavailable;
- 项目中心: unavailable;
- 知识库: unavailable;
- 智能市场 / 智能体市场 / 我的智能体 / 智能体定制: unavailable.

Task Center has limited connected slices, and the acquisition detail page has a bounded
Product Agent title-diagnosis consumer.

The final Figma target must not be projected as runtime availability until corresponding
capability gates are proven.

## 4. Proposed ownership

### 4.1 Knowledge domain

Candidate current owner:

```text
internal/knowledge
```

It owns only durable enterprise knowledge facts:

- KnowledgeBase identity and lifecycle;
- KnowledgeSource metadata;
- immutable or append-only KnowledgeRevision identity;
- ingestion/processing state;
- immutable, bounded KnowledgeContextBundle materialized for one execution;
- citation/source identity contained by that bundle;
- disable/delete visibility semantics.

It does **not** own:

- user/session identity;
- business permission policy for other domains;
- Product facts;
- Agent runtime;
- model routing;
- BusinessTask lifecycle;
- marketplace rules;
- Human Review decisions;
- billing;
- arbitrary file/object storage infrastructure.

Object bytes may be held by the existing object-storage integration selected during
implementation design. Knowledge stores safe references and bounded metadata rather than
inventing another blob platform.

### 4.2 Agent runtime remains generic

The existing `internal/agent` runtime remains the execution/checkpoint/budget owner.

V1 may require one **opaque context-snapshot reference** to be added to the generic Agent
request/model input contract so the run fingerprint can bind an already-materialized
KnowledgeContextBundle. The runtime must not import `internal/knowledge`, perform
retrieval, interpret knowledge permissions or own citation content.

The intended shape is semantic, not frozen Go:

```text
ContextSnapshotRef {
  kind
  id
  digest
}
```

The Product Agent application resolves this ref before `Start`; the governed model adapter
consumes the ref through a narrow Knowledge reader. The core runtime only validates bounded
identity/digest shape, fingerprints it and carries it unchanged.

### 4.3 Product Review keeps decision ownership, with additive provenance

The existing Product Review owner continues to own pending/accept/edit/reject/Apply and
canonical Product/source revalidation.

To preserve knowledge-backed explanation through Human Review, its internal candidate
intake/record/view needs a **bounded supplemental context-provenance reference**. This is an
additive provenance extension, not a second review state machine and not a change to what
counts as canonical Product evidence.

Product Review must not copy raw enterprise documents. It may persist only the immutable
ContextBundle ref/digest and validated citation IDs required to reconnect the review with
the exact Knowledge revisions that informed the original AI suggestion.

### 4.4 Agent enablement/configuration projection

“我的智能体” needs a product configuration fact only if the product review confirms
persistent enterprise enablement/templates are required.

Candidate responsibility:

```text
Agent catalog definition       -> existing product/config authority
Organization enablement        -> narrow agent configuration owner
Template                       -> versioned configuration reference
AgentRun                        -> existing internal/agent runtime
BusinessTask                    -> #298 projection owner
```

An enabled Agent must never be represented by a perpetual AgentRun.

This draft does not yet select the final package for Organization agent enablement/template
facts. That is intentionally blocked on #555 product review.

## 5. Minimal knowledge fact model

The following is a semantic model, not an authorized schema.

### KnowledgeBase

Required semantics:

- opaque ID;
- Organization ID from server scope;
- name;
- state: active / disabled / deleted;
- creator / updater audit identity;
- created/updated timestamps;
- version for safe mutation.

No browser-supplied Organization ID is authoritative.

### KnowledgeSource

Represents a user-visible source inside a KnowledgeBase:

- source ID;
- KnowledgeBase ID;
- display name;
- source kind;
- safe object/content reference;
- content type / size;
- processing state;
- latest revision reference;
- failure category safe for UI.

### KnowledgeRevision

A revision is the immutable content unit that can enter an execution context.

Semantic requirements:

- immutable revision ID;
- source ID;
- content digest;
- created timestamp;
- processing result;
- retrieval/index revision reference where applicable.

Changing a file creates a new revision. Existing AgentRun / BusinessTask evidence must not
silently move to the newest revision.

### KnowledgeContextBundle

V1 resolves the user's explicit Knowledge selection **before the first model dispatch**
into one immutable, bounded execution bundle.

The bundle contains:

- bundle ID + digest;
- Organization ID;
- selected KnowledgeBase identities;
- exact Source ID / Revision ID / content digest entries;
- stable citation IDs;
- bounded model-visible excerpts or structured values;
- source/location metadata needed for later citation display;
- materialization time and any PARTIAL warnings.

The bundle is not “latest knowledge”. It is one frozen execution snapshot.

For the first brand-guideline/title slice, content size and source count must be bounded
so the bundle can be materialized deterministically before Agent start. Large-corpus,
model-directed retrieval is explicitly deferred from V1.

### Knowledge citation

A Knowledge citation is a validated reference to an entry inside one exact
KnowledgeContextBundle.

It is **supplemental provenance**, not `enrichment.Evidence`, not a Product fact and not a
permission grant.

### Processing state

User-visible first version:

```text
PROCESSING
AVAILABLE
PARTIAL
FAILED
DISABLED
```

“uploaded” is not equivalent to “available to AI”.

## 6. Authorization boundary

V1 Slices A–C should add only the Knowledge permissions actually needed by the first user
path:

```text
workbench.knowledge.read
workbench.knowledge.manage
```

Exact strings are candidate names until Architecture Review freezes them.

The existing Product Agent title flow keeps its current Agent/Product permission and
LiveWrite admission. Knowledge V1 does **not** rename or broaden that permission as part of
this work. A caller must satisfy both:

```text
current Product Agent admission
+
current Knowledge read admission for every selected KnowledgeBase
```

`agent.use` / `agent.configure` are deferred to Slice D, when #555 has frozen persistent
enterprise Agent enablement/template ownership. Do not expand RBAC now merely because the
final Figma contains “我的智能体”.

Candidate semantics:

- `knowledge.read`: list/read allowed enterprise Knowledge metadata/content and materialize
  an execution context;
- `knowledge.manage`: create/update/disable sources and KnowledgeBases; it does not grant
  Agent execution by itself.

Role mapping is still a Product Review decision. In particular, this draft does not
automatically grant all Organization viewers access to potentially confidential enterprise
documents.

Rules:

- every Knowledge list/read/context-materialization is scoped to current Effective Organization;
- template binding does not grant access to a KnowledgeBase;
- context materialization performs fresh authorization;
- the governed model adapter rechecks access when it loads the exact bundle for each new
  paid/model dispatch rather than trusting a browser/template/cached document;
- revoked access after bundle creation blocks the next model dispatch/resume that needs the
  bundle;
- browser-provided user/org/role/template ownership is never authority;
- knowledge text cannot grant a ToolRef, model route, marketplace permission or write action;
- review/history may retain opaque provenance refs after access loss, but source labels,
  excerpts or document content are only resolved for a caller still authorized to read them.

The implementation may extend the existing Casbin authorizer with these current Workbench
permissions. It must not create a second IAM, role directory or Knowledge-specific user
membership system.

## 7. Knowledge ingestion and V1 context materialization

### 7.1 Upload/ingest

Candidate flow:

```text
verified user / Effective Organization
  -> bounded upload admission
  -> durable KnowledgeSource + revision intent
  -> object/content write through approved integration
  -> processing
  -> AVAILABLE / PARTIAL / FAILED
```

Requirements:

- explicit size/content-type limits;
- no external URL fetch in v1 unless separately designed;
- uploaded content remains untrusted input;
- parser/index failures do not create fake AVAILABLE state;
- retries use the original revision/operation identity where an external side effect may
  already have occurred;
- no hidden auto-save from Chat attachment into enterprise Knowledge.

### 7.2 V1 execution context materialization

The first Product Agent slice does **not** use model-directed Knowledge retrieval.

Before `runtime.Start`:

```text
explicit KnowledgeSelection
  -> fresh Organization + knowledge authorization
  -> resolve only AVAILABLE/PARTIAL admitted revisions
  -> bounded materialization
  -> persist immutable KnowledgeContextBundle
  -> return ContextSnapshotRef{id,digest}
  -> bind that ref into the Agent request fingerprint
```

This resolves the current-runtime ordering constraint: the Eino graph calls the model
before any model-selected Commerce Tool. The first model call can only happen after the
eligible Knowledge revisions and model-visible bounded content are frozen.

“Actual citations used by the suggestion” are a subset of entries in the frozen bundle and
can be discovered by the model later. The run never resolves a citation against `latest`.

### 7.3 Context loading during model execution

The governed model adapter may read the exact immutable bundle by
`ContextSnapshotRef{id,digest}` through a narrow Knowledge reader after fresh access
checking.

It must fail closed when:

- the ref/digest does not match;
- the bundle belongs to another Organization;
- current access has been revoked;
- required bundle content is unavailable/corrupt;
- the model tries to cite an ID not present in that bundle.

No knowledge text is placed in the browser-authoritative Agent request.

### 7.4 Later model-directed retrieval

Large-corpus or model-directed retrieval is **not V1**.

If later needed, it requires a follow-up architecture decision defining an atomic
pin/checkpoint protocol: the exact candidate revision set must be frozen before the model
can choose searches, and every retrieval observation must bind to that frozen set.

Do not add a Knowledge Commerce Tool to V1 merely because the current Agent runtime has a
Tool Registry.

### 7.5 Technology selection

No vector database, embedding provider or external RAG platform is admitted by this draft.

Selection is postponed until accepted content types and retrieval needs justify it.

For example:

- exact keyword groups may use structured deterministic storage/search;
- long documents may later justify embedding + vector retrieval;
- small brand/policy documents can be bounded and materialized directly;
- first-party deterministic platform rules may belong in structured domain policy rather
  than RAG at all.

Do not adopt pgvector, a hosted vector service or a generic Knowledge platform merely
because the product is called “知识库”.

### 7.6 Document parsing candidate

The inspected repository has no current PDF/DOCX parsing dependency. If Product Review
accepts the Figma-level first content set, the current recommendation is:

```text
V1 admitted source content:
- UTF-8 text
- Markdown
- PDF with extractable text
- DOCX

Explicitly not V1:
- image-only/scanned-document OCR
- PPT/XLS ingestion
- arbitrary archive ingestion
- VLM-based document understanding
```

Preferred mature parser candidate: **Apache Tika 4.x**, behind a local
`DocumentParser` port and a private integration adapter. Tika is not a Knowledge fact
owner and never receives actor/Organization/permission authority.

Official references:

- https://tika.apache.org/
- https://tika.apache.org/docs/
- https://tika.apache.org/license.html

The exact release is pinned at implementation time after dependency/security review. The
current product-design investigation observed Apache Tika 4.0.0 as the stable 4.x release;
the project is Apache-2.0.

Candidate call boundary:

```text
internal/knowledge DocumentParser port
  -> internal/integration/documentparser/tika
  -> private Tika process/service
```

Requirements:

- no public Tika ingress;
- fixed accepted MIME allowlist, checked from bytes rather than filename alone;
- bounded upload bytes, output bytes, pages/embedded objects and parser deadline;
- extracted text/metadata remain untrusted content;
- parsing failure becomes PARTIAL/FAILED rather than fabricated text;
- no remote-URL fetch initiated from document content;
- OCR, VLM parsers and any parser path that can create a paid model/provider call are
  disabled in V1;
- raw parser error bodies and document content are not written to ordinary logs;
- parser restart/crash cannot mutate Knowledge authority; durable source/revision state
  remains in the Knowledge owner.

This parser choice is still a Product/Architecture candidate. If Product Review narrows V1
to text/Markdown only, no Tika deployment should be added merely for future compatibility.

## 8. Product Agent integration

The first consumer extends the current Product Agent without replacing its runtime,
tool registry, governed model, validator or Product Review decision state machine.

Current:

```text
exact acquisition/Catalog binding
  -> canonical/source/asset/readiness tools
  -> governed model
  -> deterministic candidate validation
  -> Product Review
```

Candidate V1:

```text
exact acquisition/Catalog binding
  + explicit KnowledgeSelection
  -> fresh Knowledge authorization
  -> immutable bounded KnowledgeContextBundle
  -> ContextSnapshotRef bound into Agent Request fingerprint
  -> existing Agent runtime
  -> governed model loads exact bundle
  -> model may return citation IDs from that bundle
  -> validator verifies citation IDs + existing Product/source candidate evidence
  -> HumanReviewRequired
  -> Product Review intake with supplemental ContextProvenanceRef
  -> existing Accept/Edit/Reject/Apply semantics
```

### 8.1 Minimal generic Agent contract extension

V1 requires an additive, provider-neutral context contract. The intended semantics are:

```text
Request.ContextSnapshotRef
  -> copied into fingerprint
  -> copied into ModelInput
  -> opaque to runtime

Model result / validation
  -> ContextCitationRefs
  -> each ref must resolve inside the exact ContextSnapshotRef
```

The exact Go fields are frozen only during Architecture Review. The runtime must not import
Knowledge types or retrieve documents.

### 8.2 Knowledge citation is separate from canonical evidence

Current `enrichment.validateEvidence` intentionally permits only the canonical Product
source evidence ID. V1 preserves that invariant.

Therefore:

- `FieldChange.EvidenceIDs` continues to carry only canonical Product/source evidence;
- Knowledge citation IDs are a separate supplemental context collection;
- a Knowledge citation cannot make a Product claim valid;
- the Product Agent validator must reject unknown/out-of-bundle citation IDs before a
  candidate is reviewable.

A brand writing guide can constrain tone/terms. It cannot prove “machine washable”,
material, dimensions, compliance, price, inventory or platform eligibility.

### 8.3 Product Review receives provenance, not knowledge authority

The current `CreateFromCandidate` path continues to re-read exact Product/source data and
revalidate the candidate.

Its minimal additive input is a bounded `ContextProvenanceRef` from the validated Agent
state, containing the exact bundle ref/digest and validated citation IDs. Product Review
persists/projects that provenance for explanation but does not treat it as
`enrichment.Evidence` and does not call the model.

This is an explicit V1 contract change; the previous statement that Product Review could
remain completely unchanged was incorrect.

## 9. Version binding and template semantics

### 9.1 Per-run binding

Before the **first model dispatch**, freeze:

- selected KnowledgeBase IDs;
- exact eligible Source/Revision/content-digest entries in one KnowledgeContextBundle;
- bundle ID + digest in the Agent request fingerprint;
- agent definition/version;
- template version if used;
- prompt/policy versions;
- Product binding;
- model capability configuration reference.

The **actual citation IDs used by the model** are not required to be known before that
first dispatch. They are discovered later but must be a subset of the already-frozen
bundle and become authoritative only after server-side validation.

A resume must not:

- resolve a KnowledgeBase to newer revisions;
- replace the bundle with latest content;
- accept a citation outside the original bundle;
- silently upgrade agent/template/prompt/model policy.

### 9.2 Template

Candidate first version:

- template stores default configuration selections;
- Knowledge defaults are references, not copied authorization;
- execution resolves and re-authorizes them before materializing a bundle;
- missing/revoked/disabled knowledge makes that part unavailable and is visible to the user;
- template rename/delete does not mutate historical runs or their bundle refs.

### 9.3 Keyword import

Product review still must choose one contract:

A. snapshot copy + source/revision reference; or  
B. live reference.

Current recommendation is **A** for v1 because it makes saved configuration deterministic
while keeping provenance. This recommendation is not yet frozen.

## 10. Prompt-injection and trust model

Enterprise documents, uploaded files, official knowledge and retrieved excerpts are
untrusted content.

The Agent/model layer must distinguish:

```text
system/developer policy
tool contract / permission
canonical Product facts
user goal
knowledge evidence
external source evidence
```

Knowledge content cannot:

- change Organization/actor;
- add ToolRefs;
- alter budget;
- change model/provider;
- skip validator;
- mark a proposal approved;
- trigger marketplace writes;
- suppress audit;
- instruct the runtime to expose secrets.

The model can reason over knowledge, but runtime policy is enforced outside model text.

## 11. Citation and Human Review contract

When a knowledge-backed suggestion is shown to the user, the UI should be able to display:

- knowledge/library name;
- source/document name;
- exact revision or updated-at indicator;
- location/page/section when available;
- whether the source/result was partial;
- unavailable/deleted/revoked state without exposing source content after access is lost.

“引用知识 1” alone is insufficient for result explainability.

### 11.1 Citations survive the Review transition

Current Product Review only projects `enrichment.Proposal.Evidence`, and current
`enrichment.validateEvidence` accepts only the canonical Product source evidence.

V1 therefore adds **supplemental context provenance** to the internal Review intake/record
and user projection. It must not overload `EvidenceIDs`.

Conceptually:

```text
ContextProvenanceRef {
  context_kind
  bundle_id
  bundle_digest
  citation_ids[]
  origin_agent_run_id
}
```

The Review record needs enough immutable reference data to survive restart and preserve the
link to the exact Knowledge revisions used by the original AI suggestion. Raw knowledge
content is not copied into Review storage.

At read time, display metadata/excerpts are resolved through a narrow Knowledge citation
reader under current authorization. If access is gone, the Review can still say that the
original suggestion used an unavailable source without disclosing its protected content.

### 11.2 Human edit semantics

Knowledge citations describe the **original AI suggestion**.

If the reviewer edits the title:

- the original provenance remains for audit/explanation;
- the UI must not claim the same citations support the human-edited title;
- existing Review/Product validation and Apply semantics remain authoritative.

### 11.3 No citation authority escalation

A valid Knowledge citation proves only “this exact context was available to the model and
was cited”. It does not prove a Product attribute, policy compliance or approval.

## 12. Chat / BusinessTask boundary

#298 remains the product projection authority.

Chat may later collect:

- business goal;
- Store/Project scope;
- temporary attachments;
- explicit Knowledge selections;
- Agent/capability preferences.

But Chat is not:

- an authorization source;
- a durable Agent retry owner;
- canonical Product storage;
- Knowledge persistence by default;
- a substitute for BusinessTask when durable task tracking is required.

“仅保留会话” may omit creation of a BusinessTask/Project projection, but any actual model,
tool, payment or external-effect operation keeps its existing durable execution/audit facts.

## 13. Model capability configuration

Figma candidate `4703:892` intentionally changes the model-settings concept into
capability visibility.

First implementation should prefer existing governed configuration and expose status:

- text generation;
- image generation;
- knowledge retrieval;
- marketplace write capability.

A single successful “test connection” cannot imply all four are available.

Custom provider URL/API key is deferred from the first Knowledge slice. If later approved,
it requires:

- enterprise credential owner;
- masked secret projection;
- configure permission;
- provider allowlist/validation;
- test-call fee semantics;
- rotation/revocation;
- no secret copy into templates or browser persistence.

## 14. Public API shape — candidate responsibilities only

Exact routes/DTOs are **not frozen** during PRODUCT_REVIEW.

Likely user-facing responsibilities:

```text
GET    /api/v1/workbench/knowledge-bases
POST   /api/v1/workbench/knowledge-bases
GET    /api/v1/workbench/knowledge-bases/:id
PUT    /api/v1/workbench/knowledge-bases/:id

POST   /api/v1/workbench/knowledge-bases/:id/sources
GET    /api/v1/workbench/knowledge-bases/:id/sources
GET    /api/v1/workbench/knowledge-sources/:source_id
```

V1 Agent integration should use **local narrow ports**, not make the Agent call its own
public HTTP API:

```text
KnowledgeContextMaterializer
  explicit selection + fresh scope
  -> immutable ContextSnapshotRef

KnowledgeContextReader
  ContextSnapshotRef + fresh scope
  -> bounded exact bundle

KnowledgeCitationReader
  ContextProvenanceRef + fresh scope
  -> safe display metadata/excerpts
```

There is no generic public `/knowledge-retrieval` endpoint in the first slice.

Delete/disable/reprocess semantics require explicit review before adding corresponding
mutation routes.

## 15. Persistence, object storage and consistency

### 15.1 Current database reality

At the inspected baseline, the standard local current-application composition uses one
application PostgreSQL instance (`business-db`) with six logical application databases:

- `source_accounts`;
- commercial;
- `referrals`;
- `membership`;
- `product_acquisition`;
- `image_agent`.

ZITADEL remains on its separate identity PostgreSQL instance.

None of the six current application databases is the semantic owner of enterprise
Knowledge:

- `source_accounts` owns Source Account / account-profile and selected account verification
  facts, not enterprise document knowledge;
- `product_acquisition` owns sourcing evidence and Product Catalog, not enterprise policy or
  brand documents;
- `image_agent` owns ImageAgent, ApprovedAsset and AI invocation facts;
- commercial/referrals/membership have their own unrelated canonical owners.

Putting Knowledge tables into one of those databases merely to avoid a new pool would
create an ownership dependency that the domain model does not justify.

### 15.2 Database candidate for Architecture Review

The current recommendation is:

```text
same application PostgreSQL instance: business-db
new logical database: knowledge
schema owner: dedicated install/migration owner
serving role: knowledge_runtime
current application: separately injected bounded pool
```

This adds a **logical database**, not another PostgreSQL server.

The recommendation is not an implementation authorization. Architecture Review must still
confirm connection-budget impact, schema installation path, least-privilege grants and
backup/restore coverage before it becomes final.

If review instead chooses an existing logical database, it must document why that database
is the legitimate fact owner and how the runtime role remains least-privileged. “Fewer
config fields” is not sufficient justification.

No cross-database transaction is assumed. Product Agent and Product Review store only
immutable Knowledge bundle/provenance references; Knowledge remains the only writer of
Knowledge facts.

### 15.3 Object storage candidate

The repository already has a provider-specific S3 integration with bounded immutable put,
inspect and read behavior. V1 should reuse that integration implementation behind a
Knowledge-owned narrow port rather than import AWS/S3 types into `internal/knowledge`.

Candidate domain port responsibilities:

```text
KnowledgeObjectStore
  PutImmutable(revision object identity, bounded bytes)
  Inspect(exact object identity)
  ReadBounded(exact object identity, max bytes)
```

Integration:

```text
internal/knowledge local port
  -> internal/integration/s3 adapter
  -> private S3-compatible storage
```

Knowledge storage requirements:

- private objects only; no public URL is part of the Knowledge contract;
- deterministic/content-addressed revision object keys;
- digest + size verification before a revision becomes usable;
- dedicated bucket or credentials/policy restricted to the Knowledge namespace;
- do not reuse ImageAgent public artifact projection or expose its `PublicBase` behavior;
- object bytes are never returned solely because a caller knows an object key.

### 15.4 Upload / metadata transaction boundary

A database transaction cannot atomically commit PostgreSQL and S3.

Candidate V1 protocol:

```text
1. authorize + create durable revision/upload operation identity
2. compute deterministic object identity/digest for admitted bytes
3. immutable object put
4. if response is unknown, Inspect exact key/digest before any resend
5. persist confirmed object reference + processing state
6. parse/materialize derived Knowledge revision content
7. mark AVAILABLE / PARTIAL / FAILED
```

A lost response never creates a second revision identity merely to retry an object write.

For bounded local deterministic parsing, retry may reuse the confirmed immutable input.
If a future parser/index provider introduces its own external side effect or charge, that
provider needs a separate durable dispatch/UNKNOWN contract; it is not silently inherited
from this S3 protocol.

### 15.5 Delete / disable and historical provenance

Product Review still needs to explain historical AI suggestions without leaking deleted or
revoked enterprise content.

V1 candidate semantics:

- disable/delete stops new selection, context materialization and content reads;
- Agent runs already bound to a bundle retain opaque bundle/revision/citation identity;
- Review records retain opaque provenance;
- current unauthorized callers cannot resolve labels/excerpts after access loss;
- physical object/index retention and purge timing must be frozen before implementation;
- purge must not mutate historical Product/Review/Agent records into a different factual
  claim.

AgentRun does not copy full Knowledge content into its state.

## 16. Failure / retry / UNKNOWN

### Read/retrieval

Safe reads may retry within bounded policy.

A Knowledge dependency failure produces explicit unavailable/partial evidence; it does not
silently fall back to unrestricted model-only behavior when the user required that
knowledge.

### Upload / processing

If an upload or provider/index dispatch outcome is unknown:

- preserve the original operation/revision identity;
- verify/read back when the provider supports it;
- do not create a new revision and blindly resend an external side effect.

If processing is deterministic and entirely local after a confirmed durable input, local
reprocessing may be safe; exact policy depends on implementation choice.

### Agent model dispatch

Existing Product Agent unknown/usage/budget semantics remain authoritative. Knowledge does
not add a second retry owner.

## 17. Legacy decision

```text
Legacy decision: EXTRACT | RETIRE

EXTRACT:
- any still-current generic read/compute behavior only if concrete code is found and
  validated against this design.

RETIRE / DO NOT RESTORE:
- ListingKit Task-first knowledge/context assumptions;
- implicit tenant fallback;
- hidden browser/global knowledge binding;
- old service wrapper/fallback patterns;
- any design where Agent memory becomes Knowledge authority.
```

No migration/backfill from an old Knowledge system is planned under the greenfield policy.

## 18. Implementation sequence after product + architecture approval

### Slice A — Knowledge owner + content lifecycle

User result:

- create one enterprise KnowledgeBase;
- add one bounded text/document source;
- see PROCESSING / AVAILABLE / PARTIAL / FAILED;
- view safe source metadata and content preview.

No Agent integration yet.

### Slice B — authorized context materialization + citation

User result:

- explicitly select one enterprise KnowledgeBase;
- server materializes one immutable bounded KnowledgeContextBundle;
- exact Source/Revision refs and citation IDs survive restart;
- cross-Organization/revoked access fails closed;
- no model-directed retrieval is introduced.

No Product mutation.

### Slice C — Product Agent title optimization consumption

User result:

- select one enterprise KnowledgeBase for an existing Product Agent run;
- ContextSnapshotRef is bound before first model dispatch;
- generated title suggestion can cite only entries from that frozen bundle;
- canonical Product/source evidence remains separate;
- Product Review receives the validated supplemental provenance and keeps existing
  Accept/Edit/Reject/Apply state semantics.

This slice may require the small generic Agent context-ref/citation contract and the
additive Product Review provenance extension described above. Those are explicit
architecture changes, not hidden implementation details.

### Slice D — enterprise Agent enablement/template projection

Only after #555 freezes enabled-Agent/template ownership.

Do not block Slice A–C on a generic marketplace, Multi-Agent coordinator, Project Center,
full Chat implementation or vector database.

## 19. Review gates

Before changing this document to `IMPLEMENTATION_READY`:

### Product gate — SATISFIED

- #555 Product Gate is frozen for this V1 review.
- Figma candidates `4703:429`, `4703:572`, `4703:694`, `4703:892`,
  `4711:429`, `4713:429`, `4714:429`, and `4722:429` are the accepted
  candidate evidence for Architecture Review.
- V1 content types, snapshot-copy keyword semantics, enterprise ownership, governed-model
  boundary, template semantics and Human Review provenance rules are frozen in §2.
- Product Gate completion does **not** imply Architecture Gate completion or implementation
  authorization.

### Architecture gate

- Knowledge owner and database/persistence boundary;
- authorization model;
- ingestion state and durable operation semantics;
- immutable V1 KnowledgeContextBundle contract and limits;
- generic Agent ContextSnapshotRef / validated ContextCitationRef contract;
- exact bundle binding before first model dispatch;
- additive Product Review ContextProvenanceRef persistence/projection;
- citation read authorization and human-edit semantics;
- prompt-injection boundary;
- deletion/disable semantics;
- fee/provider boundary;
- current application composition;
- validation plan.

### Implementation gate

Only after Architecture Review explicitly records `IMPLEMENTATION_READY`.

The first-review findings about Review citation loss and pre-first-model revision pinning
are architecture BLOCKERs until these contracts are accepted; they must not be reclassified
as ordinary implementation tests.

## 20. Validation plan candidate

Risk-matched tests should cover:

- Organization A/B isolation;
- read/manage/use permission separation according to the final product rule;
- revoked access between template selection and context materialization;
- revoked access after bundle creation but before a later model dispatch;
- first model dispatch is impossible before a valid immutable ContextSnapshotRef is bound;
- same source update creates a new revision while an old run remains pinned to its bundle;
- processing FAILED/PARTIAL cannot appear AVAILABLE;
- knowledge text cannot alter tool allowlist, actor, org, budget or approval;
- model-returned citation ID outside the frozen bundle is rejected;
- Product fact conflict wins over knowledge suggestion;
- canonical `FieldChange.EvidenceIDs` remains Product/source-only;
- validated citations survive Agent -> Product Review -> restart without copying raw documents;
- Review read after knowledge-access revocation does not leak protected source content;
- a human-edited title does not inherit a false claim that AI knowledge citations support it;
- template rename/delete does not change prior run;
- model outcome UNKNOWN keeps existing Product Agent recovery semantics;
- restart reloads durable Knowledge bundle/revision/provenance references;
- browser scope switch does not render stale knowledge from previous Organization.

Real enterprise documents, paid model calls and production provider operations remain
separate explicit authorization gates.
