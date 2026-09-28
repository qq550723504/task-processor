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
> Architecture Review: **REOPENED FOR NEW BLOCKER** after Ready-triggered review at
> `174d31b0431821368087f24b41b9470e5e17d556` identified the missing public
> KnowledgeSelection transport contract.
>
> This document remains an Independent Architecture **review candidate**. Production
> implementation is closed until that blocker is fixed and targeted verification completes. Real enterprise uploads, production Tika/S3 deployment, paid model
> calls and release enablement remain separate authorization/acceptance gates.

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

V1 Slices A–C add only the Knowledge permissions required by the first user path:

```text
workbench.knowledge.read
workbench.knowledge.manage
```

The current Product Agent title flow keeps its existing admission and LiveWrite checks.
Knowledge V1 does **not** rename or broaden that Agent/Product permission.

A knowledge-backed title run must satisfy both:

```text
current Product Agent admission
+
workbench.knowledge.read for every selected KnowledgeBase
```

### 6.1 Role mapping frozen for V1

Use the existing Organization role model; do not create a Knowledge IAM.

Candidate permission mapping for Architecture Review:

| Current role | Knowledge read/use | Knowledge manage |
| --- | --- | --- |
| `listingkit_viewer` | deny | deny |
| `listingkit_operator` | allow | deny |
| `listingkit_admin` | allow | allow |
| `platform_admin` | allow | allow |

Configured platform-admin users/roles receive the same Knowledge permissions through the
existing Casbin assembly path.

The generic legacy/internal `admin` role is not automatically granted new Workbench
Knowledge permissions merely because it has older ListingKit permissions.

This mapping intentionally prevents all Organization viewers from reading enterprise
documents by default. A later product decision can broaden access, but that is not a V1
implementation detail.

### 6.2 Semantics

- `workbench.knowledge.read`: list/read allowed enterprise Knowledge metadata/content,
  preview allowed source text, and materialize an execution context.
- `workbench.knowledge.manage`: create/update/disable KnowledgeBases and source revisions.
  It does not grant Product Agent execution by itself.

`agent.use` / `agent.configure` remain deferred to Slice D, when persistent enterprise
Agent enablement/template ownership is implemented.

### 6.3 Current-state admission is mandatory on every protected content load

Permission alone is insufficient to read a frozen KnowledgeContextBundle.

Every operation that would reveal or dispatch Knowledge content must revalidate, at the
moment of that operation:

```text
verified actor
+ Effective Organization
+ workbench.knowledge.read
+ KnowledgeBase.state == active
+ KnowledgeSource.state == active
+ exact KnowledgeRevision is still content-readable
+ bundle Organization/source/revision/digest binding still matches
```

This check applies independently to:

- preview reads;
- KnowledgeContextBundle materialization;
- every load of a frozen bundle before model Quote/Decide/dispatch;
- every citation excerpt/source-content resolution in Human Review;
- resume paths after process restart or Agent checkpoint recovery.

If a KnowledgeBase or Source is disabled after a bundle was created, that historical bundle
remains an immutable provenance object but becomes **content-ineligible** for any subsequent
model dispatch or protected excerpt read. The system must not send its previously frozen
excerpts to the model merely because the caller still has `workbench.knowledge.read`.

Historical Agent/Review records may retain only opaque identity/provenance such as bundle
ID, digest, revision IDs and citation IDs. When current content admission fails, UI may show
a safe state such as “引用资料当前不可访问/已停用”, but must not reveal protected document
name, excerpt, object key or cached model-visible text unless the current read path is again
authorized and active.

Rules:

- every Knowledge list/read/context-materialization is scoped to current Effective Organization;
- template binding does not grant access to a KnowledgeBase;
- context materialization performs fresh authorization + active-state admission;
- the governed model adapter repeats that full admission when it loads the exact bundle for
  **every** new paid/model dispatch;
- revoked permission, disabled base/source or non-readable exact revision after bundle
  creation blocks the next model dispatch/resume that needs the bundle;
- browser-provided user/org/role/template ownership is never authority;
- knowledge text cannot grant ToolRefs, change model route, increase budget, change
  Organization or authorize writes;
- Review/history retains opaque provenance after access loss/disable, while protected labels,
  excerpts and document content remain unavailable.

The implementation extends the existing Casbin authorizer and
`WorkbenchPermissions()` display contract. It must not create a second IAM, role directory
or Knowledge-specific membership model.

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

Before `runtime.Start`, the Product Agent application already has a stable request identity
from the verified caller, exact Product binding and HTTP `Idempotency-Key`. V1 uses that
pre-Start identity; it does **not** depend on AgentRunID.

```text
verified Scope (Organization + Actor)
+ exact Agent Binding (context/product/version/publication/platform)
+ Agent request key (Idempotency-Key)
+ explicit KnowledgeSelection
+ materialization policy version
        ↓
KnowledgeMaterializationIdentity + complete fingerprint
        ↓
fresh Knowledge authorization + active-state admission
        ↓
resolve only currently content-readable AVAILABLE/PARTIAL revisions
        ↓
create/adopt immutable KnowledgeContextBundle
        ↓
ContextSnapshotRef{id,digest}
        ↓
construct complete agent.Request including ContextSnapshotRef
        ↓
runtime.Start
```

The materialization identity's uniqueness scope is the same stable pre-Start Agent request
tuple:

```text
Organization
+ Actor
+ Binding.ContextKind
+ Binding.ContextID
+ Agent request key
```

Its stored fingerprint additionally binds the complete Agent Binding, the normalized
client KnowledgeSelection from §14.4 and materialization policy version. The exact
server-resolved Source/Revision set is frozen inside the first committed bundle, not
re-resolved into the command fingerprint on a same-key retry. Therefore:

- first start can materialize before an AgentRun exists;
- same request retry adopts the same bundle/ref;
- same key with changed Product/platform binding or Knowledge selection conflicts;
- a crash after bundle commit but before Agent Store claim can safely retry and adopt the
  same bundle;
- Agent Runtime remains the sole AgentRun/checkpoint owner and keeps its current single
  `Store.Claim` protocol.

After the bundle ref exists, `runtime.Start` independently performs its existing fresh
Agent authorization and hashes/claims the complete `agent.Request`. No Knowledge row is
an Agent execution lease or authorization source.

This resolves the current-runtime ordering constraint: the Eino graph calls the model
before any model-selected Commerce Tool. The first model call can only happen after the
eligible Knowledge revisions and model-visible bounded content are frozen.

“Actual citations used by the suggestion” are a subset of entries in the frozen bundle and
can be discovered by the model later. The run never resolves a citation against `latest`.

A committed bundle proves what was selected/materialized at that time. It does **not**
grant future access to its cached content.

### 7.3 Context loading during model execution

The governed model adapter may read the exact immutable bundle by
`ContextSnapshotRef{id,digest}` through a narrow Knowledge reader only after the full
current-state admission from §6.3.

The reader must:

1. verify ref/digest + Organization binding;
2. re-read current KnowledgeBase and KnowledgeSource states for every referenced entry;
3. verify current caller still has Knowledge read permission;
4. verify every exact Revision is still content-readable;
5. only then return the bounded frozen model-visible content.

It must fail closed when:

- the ref/digest does not match;
- the bundle belongs to another Organization;
- current read permission has been revoked;
- any referenced KnowledgeBase/Source is disabled;
- any exact Revision is no longer content-readable;
- required bundle content is unavailable/corrupt;
- the model tries to cite an ID not present in that bundle.

A failed admission does not delete or mutate the historical bundle. It only blocks content
release/dispatch and preserves opaque provenance for audit.

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
- run Tika as a separately bounded parser process/service rather than in the API process;
- non-root runtime, read-only/rootless filesystem where supported, bounded CPU/memory/PIDs,
  and no general outbound network access;
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

This parser choice is still an implementation candidate under the approved product scope.
If V1 is later explicitly narrowed to text/Markdown, no Tika deployment should be added
merely for future compatibility.

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
  + verified Scope
  + Agent request key
  + explicit KnowledgeSelection
  -> pre-Start KnowledgeMaterializationIdentity
  -> fresh Knowledge authorization
  -> create/adopt immutable bounded KnowledgeContextBundle
  -> ContextSnapshotRef bound into complete Agent Request
  -> existing Agent runtime Start/Claim
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

V1 is frozen to **snapshot copy + source/revision provenance**.

When a keyword set is imported into a saved configuration/template:

- the selected keyword values are copied into that configuration version;
- the originating KnowledgeBase/Source/Revision identity is retained as provenance;
- later Knowledge edits do not silently mutate the saved template or historical run;
- a user must explicitly re-import/re-save to adopt newer keyword content.

Live reference is not an allowed V1 alternative.

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

When a knowledge-backed suggestion is shown to the user and current Knowledge content
admission still succeeds, the UI may display:

- knowledge/library name;
- source/document name;
- exact revision or updated-at indicator;
- location/page/section when available;
- whether the source/result was partial.

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
link to the exact Knowledge revisions that informed the original AI suggestion. Raw knowledge
content, source display names and excerpts are not copied into Review storage.

At Review read time, display labels/excerpts are resolved through
`KnowledgeCitationReader` using the full current-state admission from §6.3. Permission
alone is not sufficient.

If the KnowledgeBase/Source was disabled or access revoked after the AI run:

- Review retains the opaque original provenance;
- UI may state that an original Knowledge citation is currently unavailable/disabled;
- source/document names, excerpts and cached model-visible text are not returned;
- Apply still depends on canonical Product/source validation and current Review
  authorization, never on the availability of Knowledge citation content.

### 11.2 Human edit semantics

Knowledge citations describe the **original AI suggestion**.

If the reviewer edits the title:

- the original opaque provenance remains for audit/explanation;
- the UI must not claim the same citations support the human-edited title;
- existing Review/Product validation and Apply semantics remain authoritative.

### 11.3 No citation authority escalation

A valid Knowledge citation proves only “this exact context was available to the model and
was cited at generation time”. It does not prove a Product attribute, policy compliance or
approval, and it does not create a future right to re-read that Knowledge content.

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

## 14. Public API and HTTP contract

The first API surface is bounded to enterprise Knowledge management and observation.
There is no public generic retrieval endpoint in V1.

### 14.1 KnowledgeBase routes

```text
GET  /api/v1/workbench/knowledge-bases
POST /api/v1/workbench/knowledge-bases

GET  /api/v1/workbench/knowledge-bases/:knowledge_base_id
PUT  /api/v1/workbench/knowledge-bases/:knowledge_base_id
POST /api/v1/workbench/knowledge-bases/:knowledge_base_id/disable
```

Semantics:

- GET routes require `workbench.knowledge.read`;
- create/update/disable require `workbench.knowledge.manage`;
- all use verified identity + `LiveWrite` Organization resolution because enterprise
  document access must not retain the ordinary cached-read revocation window;
- Organization comes only from current server-resolved context, never path/body/query;
- V1 has no delete/restore route.

Create/update JSON bodies are strict, unknown fields rejected, maximum 8 KiB.
Names are UTF-8, trimmed, bounded to 120 Unicode scalar values, and cannot contain control
characters.

Mutations require exactly one `Idempotency-Key` and use request fingerprinting so a lost
response can be safely read/replayed without creating a duplicate KnowledgeBase mutation.

### 14.2 Source routes

```text
GET  /api/v1/workbench/knowledge-bases/:knowledge_base_id/sources
POST /api/v1/workbench/knowledge-bases/:knowledge_base_id/sources

GET  /api/v1/workbench/knowledge-sources/:source_id
POST /api/v1/workbench/knowledge-sources/:source_id/revisions
POST /api/v1/workbench/knowledge-sources/:source_id/disable

GET  /api/v1/workbench/knowledge-sources/:source_id/revisions/:revision_id/preview
```

Source list/detail/preview require `workbench.knowledge.read`.
Upload new source/revision and disable require `workbench.knowledge.manage`.

Upload routes use `multipart/form-data` with exactly one document part plus bounded
metadata fields. V1 limits:

- request body: 10 MiB maximum;
- one file only;
- file name: 255 UTF-8 bytes maximum after normalization;
- accepted content: UTF-8 text, Markdown, PDF with extractable text, DOCX;
- MIME is verified from bytes/allowed parser result, not trusted from extension alone.

Every upload mutation requires `Idempotency-Key`.

After authentication/authorization and before any durable mutation or S3 write, the server
must bounded-read/spool the one admitted file, verify the multipart shape, compute SHA-256
and size, normalize the bounded metadata, and construct the complete upload fingerprint.

Then:

- same key + same complete fingerprint returns/adopts the same operation/source/revision;
- same key + changed file bytes, filename-relevant metadata or other immutable command field
  conflicts;
- concurrent requests with the same key cannot both create revision identities.

Reading/spooling the bounded request before idempotency admission is not a business side
effect. No Knowledge row/object is created until the complete fingerprint is known.

Upload success is **202 Accepted** with the durable source/revision identity and current
processing state. It does not wait for Tika parsing.

### 14.3 Read projection

Source/revision reads expose only bounded product-safe metadata:

- source/revision IDs;
- display name;
- content type/size;
- state;
- digest prefix or non-secret version indicator where useful;
- created/updated times;
- safe failure category;
- bounded preview when current access permits.

They never expose:

- S3 object key;
- bucket/endpoint;
- raw parser response;
- Agent prompt;
- provider credential;
- other Organization identifiers.

### 14.4 Product Agent start KnowledgeSelection

Knowledge-backed title optimization extends the **existing** start endpoint; V1 does not add
a second Agent start route:

```text
POST /api/v1/product-acquisitions/:operation_id/product-agent/runs
Idempotency-Key: <canonical UUID>
Content-Type: application/json
```

The existing strict 8 KiB JSON request contract remains in force
(`readAcquisitionImageJSON`: UTF-8 JSON, no content encoding, duplicate/unknown fields
rejected).

V1 start body:

```json
{
  "targetPlatform": "shein",
  "knowledgeSelection": {
    "knowledgeBaseId": "01234567-89ab-cdef-0123-456789abcdef"
  }
}
```

Rules:

- `knowledgeSelection` is optional; **omitted** means “do not use enterprise Knowledge”.
- Explicit `null`, empty object, empty ID, unknown fields, duplicate fields, non-canonical
  UUID and more than one KnowledgeBase are invalid.
- V1 accepts exactly one `knowledgeBaseId` when Knowledge is selected.
- Browser/BFF never submits Source IDs, Revision IDs, citation IDs, S3 keys,
  `ContextSnapshotRef`, bundle ID or bundle digest.
- The selected KnowledgeBase must belong to the current Effective Organization and be active.
- Server resolves **all active Sources** in that KnowledgeBase. V1 requires 1–4 active
  Sources total.
- Every active Source must have a current content-readable revision in AVAILABLE or PARTIAL
  state. PROCESSING/FAILED/missing current revision makes the explicit selection
  `KNOWLEDGE_NOT_READY`; the server does not silently drop that Source.
- PARTIAL revisions may be admitted only with their existing omission/truncation warnings
  carried into the bundle.
- The existing ContextBundle byte/citation limits still apply after source resolution.
  Exceeding them fails visibly as `KNOWLEDGE_CONTEXT_TOO_LARGE`.

Canonical normalized selection is either:

```text
none
```

or:

```text
knowledge-base:<canonical-lowercase-uuid>
```

The pre-Start MaterializationFingerprint binds:

```text
full Agent Binding
+ normalized KnowledgeSelection
+ materialization policy version
```

It does **not** recompute latest Source/Revision IDs into the request fingerprint on a retry.
On the first admitted materialization, the server resolves and persists the exact Source /
Revision / digest set inside the immutable bundle. A same-key retry compares the original
client command fingerprint and adopts that existing bundle even if the KnowledgeBase has
since gained a newer Revision; it never silently moves that retry to latest Knowledge.

The resulting `ContextSnapshotRef` is created server-side and inserted into the complete
`agent.Request` before `Runtime.Start`. It is not client authority.

Start without `knowledgeSelection` preserves the existing no-Knowledge Product Agent path
and does not create an empty ContextBundle.

### 14.5 Internal local ports

Agent integration uses local contracts, not self-HTTP:

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

Delete, restore, generic search/retrieval and external-URL ingestion are Later.

## 15. Persistence, object storage and consistency

### 15.1 Current database reality at the inspected baseline

The latest main already has a dedicated `store_center` database owner in
`business-db-init.sh`, even though the account-compose README database-count paragraph is
temporarily stale. That Store documentation drift belongs to #552 and is not repaired by
this PR.

Knowledge must not be put into `source_accounts`, commercial, referrals, membership,
`product_acquisition`, `image_agent` or `store_center` simply to avoid another
configuration entry. None is the fact owner of enterprise Knowledge.

### 15.2 Knowledge logical database frozen for V1

Architecture target:

```text
same PostgreSQL instance: business-db
logical database: knowledge
schema owner: knowledge_owner
serving role: knowledge_runtime
current-application pool: knowledge.database
maximum runtime connections: 4
schema-owner install pool: maximum 2
```

This is a new logical database in the existing application PostgreSQL instance, **not a new
PostgreSQL server**.

The cluster bootstrap creates the empty database/roles only for fresh greenfield projects.
A dedicated Knowledge schema-init command installs tables/grants as `knowledge_owner`.
The serving application never runs DDL.

`knowledge_runtime` receives only CONNECT/USAGE plus the exact SELECT/INSERT/UPDATE columns
needed by the Knowledge repository. It receives no CREATE/TEMP, no cross-database CONNECT,
no role membership and no blanket access to other application databases.

No cross-database transaction, FDW or dblink is introduced.

### 15.3 Minimal durable tables

Exact SQL belongs to implementation, but V1 schema ownership is frozen around these facts:

```text
knowledge_bases
knowledge_sources
knowledge_revisions
knowledge_ingest_operations
knowledge_context_bundles
knowledge_context_bundle_entries
```

Required integrity:

- every row carries bounded Organization identity where needed for direct authorization;
- IDs are opaque UUIDs;
- operation idempotency is unique within Organization + operation kind;
- revision content digest is immutable after admission;
- one Source may have many Revisions but only one current/latest pointer;
- ContextBundle is immutable after materialization;
- ContextBundle has one unique pre-Start MaterializationIdentity and immutable
  MaterializationFingerprint;
- ContextBundle entries bind exact Source + Revision + citation ID;
- mutable records use explicit version/CAS for user-visible writes;
- source/base disable cannot mutate existing historical bundle entries.

Historical bundle rows may retain bounded model-visible content required to explain/recover
the exact past run, but that content is **not independently readable**. Every release of
that cached content is mediated by current Knowledge state/authorization (§6.3).

### 15.4 Object storage

The repository already has provider-specific S3 integration with immutable put, inspect and
bounded read behavior. V1 reuses that implementation behind a Knowledge-owned narrow port.

```text
internal/knowledge KnowledgeObjectStore
  -> internal/integration/s3 adapter
  -> private Knowledge bucket/namespace
```

Candidate port:

```text
PutImmutable(object identity, bounded bytes)
Inspect(exact object identity)
ReadBounded(exact object identity, max bytes)
```

Requirements:

- private objects only;
- deterministic/content-addressed revision keys;
- SHA-256 + size verification before processing;
- dedicated bucket or credentials/policy restricted to a Knowledge prefix;
- no ImageAgent `PublicBase` projection;
- no object is readable merely because the caller knows a key;
- raw object identity is never exposed to browser DTOs.

### 15.5 Upload / PostgreSQL / S3 protocol

PostgreSQL and S3 are not one transaction. V1 freezes this protocol:

```text
1. fresh authentication / Organization authorization
2. bounded-read/spool the single file; validate shape/MIME admission and compute SHA-256/size
3. normalize immutable metadata and construct the complete request fingerprint
4. idempotency admission + create/adopt one durable ingest operation/revision intent
5. immutable PutObject using deterministic revision object identity
6. on ambiguous PutObject result, Inspect exact key/digest before any resend
7. confirm stored object reference in PostgreSQL
8. state -> PROCESSING
9. background processor parses the exact immutable object
10. persist normalized parsed result / safe failure + state
11. state -> AVAILABLE | PARTIAL | FAILED
```

The pre-admission spool is bounded and must be cleaned up after request completion/crash
according to the app/runtime temp-file policy. It cannot be treated as a second durable
Knowledge store.

A lost HTTP response or process crash after durable admission reuses the same
operation/revision identity. It does not allocate another revision merely to retry. A crash
before durable admission leaves no Knowledge operation and may safely repeat the bounded
request-read step.

### 15.6 Processing lifecycle and recovery

V1 parsing is asynchronous and restart-safe, but does **not** introduce Temporal or
RabbitMQ.

Reason:

- parsing one confirmed immutable document is a simple background job;
- the parser has no durable remote business side effect;
- re-parsing the same immutable bytes is safe;
- Temporal workflow history is not required;
- current-application already has bounded recovery-loop patterns coordinated with server
  shutdown.

Internal revision processing states:

```text
ADMITTED
OBJECT_STORED
PROCESSING
AVAILABLE
PARTIAL
FAILED
```

`DISABLED` is a Source/KnowledgeBase availability state, not a parser terminal result.

Repository processing fields include:

- lease owner;
- lease until;
- attempt count;
- next attempt time;
- bounded failure category.

Current-application assembly starts the processor/recovery loop only when Knowledge is
enabled. It owns start/stop coordination through the long-lived runtime context and server
shutdown. HTTP handlers do not spawn unmanaged goroutines.

The same Knowledge processing/recovery owner also reconciles stale pre-object
`ADMITTED` intents; they must not remain pending forever.

For a stale `ADMITTED` operation:

1. claim it using the same bounded DB lease/fence;
2. Inspect the deterministic S3 object identity;
3. matching object digest/size -> adopt it and advance the same Revision to
   `OBJECT_STORED`;
4. mismatched object -> terminal integrity failure;
5. confirmed object missing and original request spool unavailable -> mark the same
   Revision `FAILED` with safe `UPLOAD_INCOMPLETE` / re-upload-required category;
6. never invent another Source/Revision or upload bytes the reconciler does not possess.

A later client retry with the **same Idempotency-Key + same complete upload fingerprint**
may resume that exact `UPLOAD_INCOMPLETE` operation/revision using the resent bounded
bytes. It must not create a second visible Source. A changed fingerprint conflicts.
A new key is a genuinely new upload command and must not be used automatically as recovery.

Candidate runtime bounds:

- sweep every 5 seconds;
- claim at most 4 revisions per sweep;
- maximum parser concurrency 2;
- lease 30 seconds;
- one parse call deadline 20 seconds;
- maximum 3 transient parse attempts with bounded backoff.

A deterministic unsupported/encrypted/corrupt-document error becomes `FAILED`.
A parser result with useful bounded text plus declared omissions may become `PARTIAL`.
Infrastructure timeout/crash retries the same immutable revision.

### 15.7 Document parser limits

The V1 parser adapter is private Apache Tika 4.x when PDF/DOCX support remains enabled.

Bounded parse output:

- raw source: 10 MiB maximum;
- normalized extracted UTF-8 text: 2 MiB maximum per Revision;
- parser output beyond the bound is PARTIAL with explicit truncation/omission metadata;
- no embedded-file recursive ingestion in V1;
- no remote URL fetching;
- no OCR/VLM;
- raw document and parser body are excluded from ordinary logs.

### 15.8 Disable, revoke and historical provenance

Disable is an immediate **content-use fence**, not merely a discovery/listing flag.

When a KnowledgeBase or Source transitions away from active:

- it is excluded from all new selection/materialization;
- every future preview/content read fails closed;
- every future load of any already-created ContextBundle that references it fails closed
  before releasing cached excerpts;
- every future model Quote/Decide/dispatch or resume that needs such bundle content is
  blocked before provider dispatch;
- every future citation-detail resolution returns only safe unavailable/disabled state.

The same content-use fence applies when the caller loses current
`workbench.knowledge.read` or Organization access.

What remains durable:

- bundle ID/digest;
- Source/Revision IDs;
- citation IDs;
- Agent run and Product Review provenance relationships;
- safe state that the source is currently unavailable/disabled.

What does **not** remain readable solely because it was once materialized:

- source/document display name if it is protected enterprise metadata;
- excerpt/body;
- object key;
- cached model-visible content;
- parser output.

Disable/revoke does not retroactively change what the original model saw. It only prevents
that protected content from being released again.

V1 has disable, not destructive user delete. Physical purge/retention is not a V1 user
action. Operational retention/purge policy must be added before destructive delete is
admitted.

AgentRun stores ContextSnapshotRef and validated citation refs, not authority to re-read the
content.

## 16. Failure, retry and UNKNOWN

### 16.1 Read/materialization

Safe database/S3 reads may retry only within bounded local policy.

If the user explicitly selected Knowledge and materialization cannot establish a valid
bundle, the title run fails closed with a Knowledge-unavailable result. It must not silently
drop the selected Knowledge and continue as unrestricted model-only generation.

### 16.2 Immutable object write

For S3-compatible immutable Put:

- authoritative pre-dispatch failure may be retried with the same operation/revision;
- timeout/connection loss is outcome-unknown;
- outcome-unknown must Inspect the exact deterministic key + SHA-256 + size;
- matching object means the original write is adopted;
- missing object after authoritative inspection permits resend with the same identity;
- mismatched object is integrity conflict and fails closed.

No second revision identity is created to escape an ambiguous object write.

### 16.3 Parser processing

Tika parsing does not mutate an external business system. Re-executing the same immutable
object is allowed after lease expiry or process crash.

- deterministic unsupported/encrypted/corrupt input -> FAILED;
- valid partial extraction -> PARTIAL;
- transient parser/service failure -> retry same revision up to the bounded attempt limit;
- exhausted transient attempts -> FAILED with safe category;
- manual “re-upload” creates a **new** Revision only when the user intentionally provides a
  new upload command.

### 16.4 ContextBundle materialization

There are two different immutable identities and they must not be mixed:

1. **incoming command identity/fingerprint** — derived only from information available before
   server-side Knowledge resolution;
2. **bundle content identity/digest** — records the exact Source/Revision content frozen by
   the first successful materialization.

For Product Agent V1, materialization uses the pre-Start stable request identity from §7.2:

```text
MaterializationIdentity {
  OrganizationID
  ActorID
  ContextKind
  ContextID
  AgentRequestKey
}

MaterializationFingerprint = SHA256(
  full Agent Binding
  + normalized client KnowledgeSelection
  + materialization policy version
)
```

**MaterializationFingerprint never includes server-resolved Source IDs, Revision IDs,
current/latest pointers, content digests or source counts.**

On the first successful admission only, the materializer resolves the selected
KnowledgeBase under current authorization/active-state rules and persists the exact
Source/Revision/digest/citation set inside the immutable KnowledgeContextBundle. The
bundle's own digest binds that resolved content.

The Knowledge repository atomically enforces uniqueness on MaterializationIdentity and
command-fingerprint equality, analogous to—but independent from—the Agent Store's run
claim.

Retry behavior is therefore unambiguous:

- same identity + same full Agent Binding + same normalized KnowledgeSelection + same policy
  -> return/adopt the originally committed bundle/ref without resolving latest Sources;
- a Source added/removed or a newer Revision created after the first commit does not change
  that retry's command fingerprint;
- same identity + changed Binding, changed normalized KnowledgeSelection or changed policy
  -> conflict;
- if current access/active-state checks later make the frozen bundle content ineligible,
  §6.3/§15.8 block content release/dispatch; the system does not rematerialize latest
  Knowledge under the old Agent key.

A bundle committed before an AgentRun is claimed may remain as an unused durable context
object; it is not execution evidence and cannot authorize model dispatch.

Immutability preserves historical provenance; it does not bypass later active-state or
authorization checks.

### 16.5 Agent model dispatch

Existing Product Agent quote/reservation/UNKNOWN/budget semantics remain authoritative.

Before **every** `Quote`, `Decide` or provider dispatch that needs Knowledge content, the
governed model adapter:

1. loads the exact ContextSnapshotRef;
2. performs the complete §6.3 current-state admission;
3. only after that releases model-visible bundle bytes to quote/token estimation;
4. verifies the same admission again immediately before provider dispatch if the quote and
   dispatch are separated by an authorization/state boundary.

The context bytes actually used for the model are included in the quote/token estimate;
Knowledge must not become an unmetered hidden prompt.

If permission is revoked, KnowledgeBase/Source disabled, or exact Revision becomes
content-ineligible before a later model call/resume:

- no Knowledge bytes are released to the model layer;
- provider dispatch does not occur;
- the run stops/fails under the existing authoritative no-send/dependency/authorization
  semantics;
- opaque provenance remains durable.

Knowledge adds no second model retry owner.

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

## 18. Implementation sequence after Architecture approval

No slice may modify production code before this document becomes
`IMPLEMENTATION_READY`.

### Slice A — Knowledge owner + content lifecycle

User result:

- create one enterprise KnowledgeBase;
- upload one V1-supported document;
- receive durable source/revision identity immediately;
- observe PROCESSING -> AVAILABLE/PARTIAL/FAILED;
- preview bounded extracted content;
- restart current-application without losing processing/recovery state.

Includes:

- Knowledge logical DB/schema-owner/runtime-role;
- S3 KnowledgeObjectStore adapter;
- Tika adapter when PDF/DOCX is enabled;
- DB-lease parser recovery loop;
- Knowledge read/manage permissions;
- Workbench HTTP/BFF/UI first slice.

No Agent integration yet.

### Slice B — immutable execution context + citation

User result:

- explicitly select enterprise Knowledge for a bounded title run;
- server materializes one immutable bounded KnowledgeContextBundle before model dispatch;
- exact Revision/citation identity survives restart;
- cross-Organization/revoked access fails closed.

No Product mutation.

V1 ContextBundle hard bounds:

- maximum 4 selected Sources;
- maximum 16 KiB normalized model-visible content per selected Revision;
- maximum 48 KiB normalized model-visible Knowledge text in one bundle;
- maximum 64 citation entries;
- maximum 96 KiB serialized bundle payload.

If explicit selection exceeds the bound, materialization fails visibly with a
`KNOWLEDGE_CONTEXT_TOO_LARGE`-class result. V1 does not silently truncate arbitrary
documents into a misleading context. Users may select fewer/smaller sources.

### Slice C — Product Agent title optimization consumption

User result:

- start existing Product Agent title optimization with an optional ContextSnapshotRef;
- first model quote/dispatch happens only after exact Knowledge context is frozen;
- generated title can cite only citation IDs from that bundle;
- Product/source deterministic evidence remains canonical and separate;
- Product Review receives bounded supplemental ContextProvenanceRef;
- user can accept, reject or edit, then Apply under existing Product Review rules.

Generic Agent contract adds only provider-neutral opaque context/citation fields; it does
not import Knowledge types.

### Slice D — enterprise Agent enablement/template projection

Only after Slices A–C are proven and #555’s “我的智能体” persistent configuration is
scheduled.

Possible new Agent-specific permissions belong here, not in A–C.

Do not block A–C on Agent marketplace, Multi-Agent coordination, Project Center, full Chat,
vector database or model-directed retrieval.

## 19. Review gates

Before changing this document to `IMPLEMENTATION_READY`:

### Product gate — SATISFIED

- #555 Product Gate is frozen for this V1 review.
- Figma candidates `4703:429`, `4703:572`, `4703:694`, `4703:892`,
  `4711:429`, `4713:429`, `4714:429`, and `4722:429` are the accepted
  candidate evidence for Architecture Review.
- V1 content types, snapshot-copy keyword semantics, enterprise ownership, governed-model
  boundary, template semantics and Human Review provenance rules are frozen in §2.

### Architecture gate — REOPENED FOR TRANSPORT BLOCKER

Architecture Review accepted the following V1 contracts:

- Knowledge owner: `internal/knowledge`;
- dedicated `knowledge` logical database in the existing business PostgreSQL instance;
- `knowledge_owner` schema installer + `knowledge_runtime` serving role/pool;
- Workbench Knowledge read/manage permissions and V1 role mapping;
- strict V1 HTTP/API surface and upload bounds;
- private S3 object boundary and ambiguous immutable-write recovery;
- asynchronous DB-lease parser recovery without Temporal/RabbitMQ;
- bounded Tika integration and source types;
- immutable KnowledgeContextBundle and hard materialization bounds;
- generic Agent ContextSnapshotRef / validated citation extension;
- exact bundle binding before first model quote/dispatch;
- additive Product Review ContextProvenanceRef persistence/projection;
- citation read authorization and human-edit semantics;
- prompt-injection boundary;
- disable/history semantics;
- existing AI usage/budget ownership;
- current-application composition and validation plan.

Review history:

- formal round 1 identified the disable/revoke content-use fence BLOCKER;
- formal round 2 found no new major issue after that fix;
- targeted verification identified and closed upload/materialization idempotency and the
  pre-Start AgentRun/ContextBundle circular-admission BLOCKER;
- targeted review at `96ba7f6549fcaf97ed358f94802919bbb03ea9d2` found no major issue;
- CI `36422741730` completed SUCCESS;
- later Ready-triggered review at `174d31b0431821368087f24b41b9470e5e17d556`
  demonstrated the missing KnowledgeSelection transport P1 BLOCKER; admission was reopened.

The normal review stop rule was reached at `96ba7f6`, but Ready-triggered review on
`174d31b` demonstrated a new P1 BLOCKER: the public Product Agent start request did not
define how explicit KnowledgeSelection enters the application. §14.4 now freezes that
transport contract. The stale ADMITTED ingest recovery obligation is also made explicit as
IMPLEMENTATION_TEST, and §9.3 removes the contradictory live-reference alternative.

### Implementation gate — CLOSED PENDING TARGETED VERIFICATION

Slices A–C must not start until the §14.4 transport fix receives targeted review and this
document is explicitly returned to `IMPLEMENTATION_READY`.

This status does **not** authorize:

- merging this PR;
- deployment;
- real enterprise document uploads;
- production Tika/S3 enablement;
- paid/real model calls;
- release enablement or user acceptance.

Those remain separate explicit gates.

## 20. Validation plan

Architecture acceptance requires implementation evidence plans for these risks.

### 20.1 Authorization / Organization / disable fence

- viewer denied Knowledge read/manage;
- operator read allowed, manage denied;
- admin read/manage allowed;
- Organization A cannot list/read/preview/materialize B Knowledge;
- browser-selected Organization is not authority;
- grant revoked between selection and materialization fails closed;
- grant revoked after bundle creation blocks the next model dispatch;
- KnowledgeBase disabled after bundle creation blocks bundle reload and next model dispatch;
- Source disabled after bundle creation blocks bundle reload and next model dispatch;
- disable between model Quote and provider dispatch is rechecked and blocks dispatch;
- citation read after disable returns only safe unavailable state, not source name/excerpt;
- historical Review/Agent provenance remains structurally present after disable;
- template reference does not restore revoked/disabled Knowledge access.

### 20.2 Persistence / upload / parser

- Product Agent start rejects unknown/duplicate KnowledgeSelection fields, null/empty
  selection and non-canonical IDs;
- omitted KnowledgeSelection preserves the existing no-Knowledge start path;
- explicit selection accepts one active KnowledgeBase only and server-resolves its Sources;
- PROCESSING/FAILED active Source causes KNOWLEDGE_NOT_READY rather than silent omission;
- same Agent key + same normalized selection adopts the original bundle after a newer
  Knowledge revision appears;
- same Agent key + changed KnowledgeSelection conflicts;
- upload fingerprint is complete before durable idempotency admission/S3 write;
- duplicate Idempotency-Key + same upload returns the same revision/operation;
- concurrent same key + same upload creates one revision identity;
- same key + changed file/metadata conflicts before object write;
- S3 timeout + confirmed matching exact object adopts original write;
- S3 timeout + confirmed missing exact object resends same identity;
- mismatched deterministic key/digest fails closed;
- stale ADMITTED + matching object is adopted into OBJECT_STORED;
- stale ADMITTED + confirmed missing object becomes UPLOAD_INCOMPLETE instead of remaining
  pending forever;
- same upload key/fingerprint can resume the exact UPLOAD_INCOMPLETE revision with resent
  bytes without creating a duplicate Source;
- restart after OBJECT_STORED resumes parsing;
- expired processing lease can be reclaimed without duplicate revision;
- unsupported/encrypted/corrupt input -> FAILED;
- Tika process runs with the approved private/no-general-egress/resource-bounded sandbox;
- useful incomplete extraction -> PARTIAL;
- output bound is enforced without pretending truncated output is complete;
- runtime role cannot DDL, connect to unrelated databases or read raw object credentials.

### 20.3 Context binding / Agent

- fresh Agent Start can materialize a bundle before any AgentRun row exists;
- crash after bundle commit but before Agent Store Claim: retry adopts the same bundle and can
  still acquire the one AgentRun;
- repeated same pre-Start tuple + same full fingerprint adopts the same ContextBundle/ref;
- same pre-Start tuple + changed Product/platform binding, selected Knowledge or policy
  conflicts rather than creating a second bundle;
- materialized-but-unclaimed bundle cannot authorize or imply an AgentRun/model dispatch;
- first model Quote cannot occur before a valid ContextSnapshotRef is committed/bound;
- same Knowledge source update creates a new Revision while an existing run remains on its
  original bundle;
- bundle limits fail visibly rather than silently changing selected evidence;
- ContextSnapshotRef/digest mismatch fails closed;
- citation outside the frozen bundle is rejected;
- knowledge text cannot alter actor, Organization, Tool allowlist, model, budget or approval;
- Product fact conflict wins over Knowledge suggestion;
- canonical `FieldChange.EvidenceIDs` remains Product/source-only;
- Knowledge prompt bytes are included in existing model quote/usage accounting;
- disabled/revoked bundle content is never sent on a later model call;
- model outcome UNKNOWN retains current Product Agent recovery semantics.

### 20.4 Human Review / provenance

- validated citations survive Agent -> Product Review -> process restart;
- Review does not copy raw enterprise documents/source display content;
- revoked/disabled caller path cannot resolve protected citation labels/excerpts;
- Review can still show safe “original citation unavailable/disabled” provenance;
- human-edited title does not falsely inherit AI citation support;
- Accept/Reject/Edit/Apply retain existing Product Review CAS/authorization behavior.

### 20.5 UI / Figma

- all eight accepted candidate flows have loading/empty/denied/failed states where relevant;
- Organization switch cannot render stale Knowledge from the previous Organization;
- upload processing state is truthful after refresh/restart;
- “enabled Agent” is never rendered from AgentRun running state;
- template UI states that defaults are re-authorized at execution;
- title execution shows actual selected Knowledge revision/version indicator;
- Human Review shows citation provenance separately from Product fact validation;
- disabled Knowledge renders an unavailable provenance state without protected excerpt leak.

Real enterprise documents, production S3/Tika deployment and paid model/provider operations
remain separate explicitly authorized acceptance gates.
