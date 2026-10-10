# AI Workbench Project Center V1

> Status: APPROVED / IMPLEMENTATION_READY.
> Design Basis: Independent Architecture. Execution: #624; parents #137 / #298.
> Writer: 01a11f6e-6d5e-7ca1-a82a-b0946c2cd14f.
> Baseline: main `12e855b53fa40c09132e2336f6acec5d8a76435b` (2026-10-09).

## 1. Product outcome and current decisions

Current internal-trial members can create, revisit and manage a durable private project
for a long-term business goal, organize existing conversations/tasks/materials/results,
archive and restore it, and save its structure as a personal reusable template.

The user explicitly confirmed in the execution conversation on 2026-10-09:

- A project is private to its creator **within the current organization**. No enterprise
  administrator or project collaborator gains access to another person's project.
- Creation saves the goal; the user manually associates existing resources. AI planning
  and execution continue through the existing Chat proposal/explicit confirmation path.
- Templates come from the user's own project structures. No invented platform templates.

Figma authority: `docs/product/final-ui-ia-authority.md`, file
`tg48P46SSXl6TBy9lZwg63`, visible non-archived page `31:463`.
Read frames: active `667:359`/light `667:630`, store/non-store `669:397`/`669:675`,
recent `579:3445`, archived `579:4069`, new `568:367`, detail `568:365`,
type selector `570:365`, templates `427:4081`, Chat associations `721:764`/`722:1278`.
The user's manual-organization decision supersedes the new-project automatic-decomposition
copy. Figma example percentages, reports, task counts and suggested actions are not facts.
Template frame still contains generic sample tasks, so implement the confirmed personal
template semantics using the established Console components and layout.

In scope: active/recent/archived project lists, title search, type/work-scope filters,
new/detail/edit, optional current Store reference, manual resource links, archive/restore,
personal structure templates, fresh authorized task/result projections.
Out of scope: shared projects, member grants, automatic AI decomposition, extra model calls,
new task/run/report/file/Knowledge owners, task cancellation/retry, remote publish,
payment, paid providers, legacy migration or compatibility, general project management.

## 2. Verified owners and architecture problem

`internal/aiworkbench` owns the delivered private Conversation and BusinessTask. Its
approved `ai-workbench-chat-business-task-v1.md` excludes new persistent Projects/Reports.
AgentRun, Review, Knowledge, acquisition/Product, Store and authorization retain their
existing contracts. Project links do not grant any resource permission.

The existing AIWorkbench composition requires Product Agent and model policies. Requiring
them merely to save a project would prevent this core path in an unconfigured trial.
Project Center therefore has a separately optional feature module, initialized from an
explicit bounded owner pool; project CRUD has no provider/Agent/Commercial dependency.
There is no second AI runtime. Missing resource readers disable the affected association
kind and produce an explicit unavailable projection, while project metadata stays usable.

## 3. Contract -> implementation -> injection -> consumer

| Responsibility | Owner / path |
| --- | --- |
| Project, Template, relationship intent, visits, receipts | `internal/aiworkbench/projectcenter` |
| PostgreSQL implementation | `internal/integration/persistence/aiworkbench/projectcenter` |
| Feature-local HTTP descriptors and bounded JSON | `internal/aiworkbench/projectcenter/httpapi` |
| Fresh authorization and narrow reference adapters | `internal/app/httpapi/current_project_center.go` |
| Configuration/pool lifecycle/initializer | existing `currentapplication` / current-schema-init |
| BFF, contracts, page and components | existing `web/listingkit-ui` workbench conventions |

New feature routes consume the existing CurrentIdentity + LiveWrite organization resolver
and Casbin authorizer. The app composition injects scoped readers to the domain via a
`ReferenceReader` port; no project SQL joins or writes another domain's tables.
Adapters consume existing scoped Conversation/Task Store methods, existing task projection,
Knowledge GetBase/GetSource, Product acquisition authorized reader and member-scoped Store.
They never return their raw records or secret fields through Project Center.

## 4. Facts and relational model

Project fields: UUID, organization, creator, title, goal, type, optional Store ID,
optional target calendar date, ACTIVE/ARCHIVED, positive revision, created/updated times.
Type values map to the visible design: STORE_OPERATIONS, PRODUCT_DEVELOPMENT,
PRODUCT_RESEARCH, BRAND_BUILDING, OPC, OTHER. These are organization labels, not capabilities.

A project can hold up to 100 typed references: CONVERSATION, BUSINESS_TASK, PRODUCT,
KNOWLEDGE_BASE, KNOWLEDGE_SOURCE. Only stable owner-issued IDs are persisted, never labels,
result text, task status, permission snapshots, Knowledge content or Product copies.
Task results/reports remain projections of linked tasks' existing authorized result/Review
references; no independent Report ID, snapshot or store is invented.
Conversation links are independent: linking one does not silently include all its tasks,
and linking a task does not expose the conversation transcript without Chat authorization.
PRODUCT identifies the actor-scoped saved acquisition **operation UUID** consumed by
PublishedAcquisitionReader; it never accepts a bare ProductKey or arbitrary catalog lookup.
Each link has a project-owned slot UUID. Denied projections preserve only this slot and kind,
so its owner can remove an inaccessible link without receiving protected target identifiers.

A Template is a private immutable structure snapshot of title suggestion, goal and type,
with its own user-chosen name, version and created time. It copies no Store/resource IDs,
dates, lifecycle, visits, receipts, tasks, outputs or permission. The user explicitly chooses
Store/date/resources for each new project. Apply template pre-fills the new-project form;
creation persists the explicitly confirmed fields, never executes tasks or models.
Deleting a template may be expressed as metadata archival; no project is deleted.

Project and Template facts, visits and operation receipts live in the feature's
`ai_workbench_projects` schema in an explicitly configured logical DB, with runtime role
`ai_projects_runtime`. This is a subdivision of the current aiworkbench domain, not a
replacement Conversation/Task owner. Other owner pools are borrowed by reference adapters
with their current restricted identities. Runtime never installs/migrates schema.
The configured project logical database must differ from every configured owner by
host/port/database, independently of login role or connection-pool object identity.

## 5. Transactions, versions and idempotency

All mutations require a UUID Idempotency-Key scoped to organization + actor, an operation
tag and canonical payload fingerprint. Same key/different operation or payload conflicts.
Receipt, project/template/relationship mutation and append-only project audit commit in one
transaction. PostgreSQL scoped transaction advisory locks serialize keys, then project row
locks serialize revisions. Locks never span external-owner I/O.

Create produces revision 1. Edit, reference add/remove, archive and restore require the exact
expected revision and increment once. Each reference add/remove holds the project lock;
removal identifies the project-owned slot and does not delete targets. Archived projects reject edits/new links;
restore alone reopens metadata management. Replayed receipts do not repeat writes and do
not replace a later revision. The mutation response identifies its committed revision;
clients reload the current entity after replay rather than installing an old snapshot.

Visit is explicit POST after opening a detail page, not a mutation hidden in GET. It uses
the same authorization and idempotency, updates only the private visit record, and never
changes the Project revision, business progress or updated time. Recent ordering uses visit
time with stable ID tie-breaks. All lists are bounded (max page size 20) and keyset ordered.

Reference validation happens before the local transaction. Store association likewise uses
the original member-scoped reader. Current organization/actor permission is rechecked at
mutation entry; reference permissions are freshly checked again on each projection.
Revocation during read validation cannot grant access or copy protected data: the only
persisted value is a non-authoritative reference, and future reads fail closed.
Unknown DB commit results retain the same key and reconcile by receipt; never mint a fresh
key automatically. All effects are local, repeat-safe references; no remote recovery/Saga.
Browser recovery storage encodes actor/organization as an unambiguous JSON tuple;
identifier delimiters cannot merge two private scopes.

## 6. Authorization and bounded data

New permissions are `workbench.project.read` and `workbench.project.manage`, registered
through existing enterprise module grants. Static viewer can read their own existing
projects; operator/admin can manage their own projects; current platform admin also remains
limited to their own project in the active organization. Native enterprise custom-role
grants follow the existing Casbin model. Projects module grants confer neither Chat/Task,
Store, Product nor Knowledge access. Each referenced kind requires its original permission.

Every repository predicate includes organization + actor + resource ID. Client scope,
creator, authorization flags, hrefs and projected fields are not accepted as inputs.
Missing/wrong scope returns NOT_FOUND without disclosing names/counts. Fresh resource denial
returns only a typed unavailable reference slot; protected target ID/title/url/output is
omitted from the response. A bad link candidate cannot be used as an existence oracle.
Project-owned title/goal remain readable by its owner; they are user-authored text, never
trusted model/system instructions. UI renders plain text.

Requests have bounded deadline, JSON bytes (16 KiB), strict unknown/duplicate field rejection,
UTF-8, identifiers, strings and list bounds. GET rejects unread bodies before parsing.
List projections cap resource resolution and do not fetch full transcripts/reports.
Cards read local reference membership, resolve only task/store summaries with a page-wide
limit of 20 source reads and a two-second projection deadline, and keep an unavailable
summary when the budget or source availability prevents a complete aggregate. Detailed
non-task references are resolved only by the single-project view.
Runtime role has no schema ownership/CREATE, role inheritance, cross-owner privileges,
TRUNCATE or target-object writes. Only required table/column mutations are granted and
verified at startup. New-empty-instance initialization is explicit; no legacy migration.

## 7. UI and honest projections

Reuse the Console shell/tokens and native Card/Button/Input/Select/Dialog, current
WorkbenchContextProvider and actor+organization+session-epoch request isolation. A->B->A
late responses and in-flight mutation completions cannot restore the earlier context.
No localStorage/fake data is a Project fact store. Retain same mutation key for ambiguous
responses and show a reconciliation/reload action; disable context-sensitive edits in flight.

Detail tabs organize overview, linked tasks/conversations, materials/Knowledge and linked
task results. Use the original resource entrypoints for actions and Review; Project Center
does not duplicate execution buttons or apply to Product. Archived read-only detail retains
links and offers restore. Optional Store metadata is freshly authorized; no access hides
its label and link without changing an existing user's authored project goal.

Show `completed linked tasks / linked task total` only with complete authorized projections;
label this precisely as linked-task completion, never a guessed overall project percentage.
When no tasks are linked show the real empty state. Unknown/denied projections do not become
zero or completed; show unavailable summary. Waiting counts come from existing TaskState.
Project lifecycle is user-owned metadata; runtime task lifecycle continues when archived.
Search/type/Store filters and today's metadata updates use real owner facts, not Figma samples.

## 8. Implementation and evidence

Before production changes: independent review of this contract must explicitly establish
IMPLEMENTATION_READY; findings follow AGENTS classifications, max two normal rounds.
Shared navigation/policy/config/runtime files are claimed in #624/#137. The user explicitly
assigned this thread the **Project Center local injection only** on 2026-10-09; other module
owners keep their scopes. Shared files preserve all unrelated registration and fields.

First independent Architecture Review: `/root/project_architecture_review`, 2026-10-09,
read-only actual-code review: **no BLOCKER / IMPLEMENTATION_READY** after the reference-slot,
acquisition identity and receipt response clarifications above. No second global round required.
Three bounded IMPLEMENTATION_TEST obligations carry into #624:

- Project permission alone must not authorize Conversation/Task/Knowledge/Store/Product
  readers whose low-level methods rely on callers enforcing their original permissions.
- Response-loss replay after source revocation/project archival reads the original receipt
  and does not repeat mutations; subsequent GET uses current authorized projections.
- With Agent/model disabled, independently configured Project CRUD works; missing source
  readers disable only the associated kind and do not turn missing data into empty success.

TDD for meaningful rules: scoped private access; key replay/conflict and atomic rollback;
revision/archived conflicts; link source permissions and denial redaction; template copy
excludes target refs and dates; visit does not change project revision; Task result projection
does not grant original-owner permission. Use existing Go tests, PostgreSQL integration and
frontend Vitest. Match checks to the actual risks, no new runner or verification platform.
Final review checks the completed user path and actual diff. CI and developer checks remain
separate from independent/user acceptance. Startup/handoff follows the existing current
application path, private configuration and explicit fresh-schema initialization.

Legacy decision: RETIRE.
Reusable behavior: current scoped owner readers, Gorm/PG transactions and receipt patterns,
existing Casbin/organization middleware and Console/BFF components.
Current owner: aiworkbench Project Center plus unchanged resource owners.
Cutover/deletion condition: no retired Workspace/Task/ListingKit facade consumer is created;
the ai/projects placeholder is replaced only when composition is registered and operational.

Merge, Issue closure, shared/production deployment, existing database DDL, real data and
paid provider calls remain unauthorized. User acceptance and real-provider checks: NOT_RUN.
