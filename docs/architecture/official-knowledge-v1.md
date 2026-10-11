# Official Knowledge V1 — Product Gate and design proposal

> Status: **APPROVED / IMPLEMENTATION_READY** — approved reading scope only.
>
> Independent read-only Architecture Review: `official_architecture_review`,
> 2026-10-11, contract HEAD `6fcb27fbf740729b4f9172e60e7417f8005a0545`.
> Result: IMPLEMENTATION_READY, no BLOCKER. One IMPLEMENTATION_TEST requires an
> unread GET body rejection test in the BFF; it does not reopen architecture.
>
> Execution: [Issue #632](https://github.com/qq550723504/task-processor/issues/632).
>
> Sole Writer: Codex chat `01a128eb-5e42-7900-99aa-ae4eb841f8a4`.
>
> Inspected baseline: `origin/main @ 60d432aa8b9391491278912e4268950ba65b58f0`,
> fetched on 2026-10-11. Separate branch: `codex/official-knowledge`.
>
> Design Basis: **Independent Architecture**. Product decisions below and applicable
> independent architecture review must precede `IMPLEMENTATION_READY` and production edits.

## 1. User outcome and current gap

A signed-in Workbench user opens AI工作台 → 知识库 → 官方知识库 to find real
Sumi-maintained material, read it, and identify its source and exact version. Any
selection/use action must lead to an approved real consumer, rather than a success
message or an enterprise copy of platform material.

[Current UI Authority](../product/final-ui-ia-authority.md) and
[Figma official frame 682:359](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=682-359)
were inspected live. The frame states: Sumi maintains the content, users choose it
as needed, and it is not cited by default. Its four category cards describe possible
content; they are not evidence of published documents or granted content rights.

The approved [enterprise Knowledge design](agent-knowledge-context-v1.md) governs
current-organization documents and execution context. Its trust rules also mention
official knowledge, but it has no official catalogue, publisher, shared-content
visibility or official-version contract. Issue #555 and runtime Issue #619 do not
fill that gap.

Main has a concrete enterprise Knowledge owner. PR #631, still open at inspected
HEAD `69122b6351d15bc579a9435e2f3439d19720fa55`, adds the navigation and an explicit
unavailable official page. There is no current official API or content owner.

## 2. Product Gate — approved 2026-10-11

The user explicitly selected all three proposed first-release choices in this chat:

| Decision | Approved choice |
| --- | --- |
| First real content | AI电商应用指南, curated from verified repository facts and approved product documents, with named provenance |
| Maintenance/publication | Repository versioning, PR review and publication with the application release |
| First usable path | Read real documents, exact versions and sources; formal AI consumption waits for suitable material |

Application instructions help a human operate the product. They are not automatically
appropriate supplemental input for generating a product title. Treating every guide
as title-generation context merely to make “选择使用” clickable would manufacture
a product rule and provide little current value.

Show “查看内容” for the approved reading action and state that AI reference is not open.
This explicit phase decision narrows Figma's “选择使用” action for this release.

The decisions are recorded in Issue #632 before architecture admission. No further
content-source or publisher product decision is required for this approved reading path.

## 3. Scope and exclusions

For a confirmed repository-published reading release: catalogue, real article text,
exact version, update date, maintained-by information, provenance, unavailable/error
states, normal authenticated Workbench access, and a normal startup handoff.

Do not fabricate the other three Figma categories, article counts, freshness dates,
customer data, provider readiness, or a selected-for-AI state. Empty/unpublished
categories may be omitted or visibly unavailable according to the confirmed scope.

Excluded: personal Knowledge, automatic web scraping or refresh, generic RAG/search,
new IAM/admin roles, a new workflow/recovery platform, paid model calls, legacy
migration/fallback/dual writes, existing trial data operations, production deployment,
merge, and Issue closure. AI consumption and online publishing are explicitly Later
under the user's current phase decision, not current acceptance requirements.

## 4. Owner and implementation path

Use a separate platform-content owner under `internal/knowledge/official`. It must
not depend on enterprise `knowledge.Service`, create a fake Organization, create
enterprise Base/Source rows, or publish a second copy to every enterprise database.

For repository publication, canonical content is a bounded release catalogue and
versioned UTF-8 content files. It is neither a customer upload nor a Product asset.
The Go embed facility already used in the repository can package this catalogue;
no new database, S3 bucket or Tika parser is justified for authored text.

Proposed call chain:

```text
official catalogue/read contract
  -> bounded embedded catalogue + exact version content
  -> owning official/httpapi module
  -> current-application assembly injection
  -> authenticated, bounded Workbench BFF
  -> existing ConsolePage/Card/Button and official catalogue/article page
```

Application assembly owns construction only. Content validation and version lookup
belong in the official owner; route registration belongs in its HTTP module. Domain
code must not import app HTTP assembly or a retired ListingKit facade.

Reuse current auth identity, Workbench context, route policies, strict JSON response
reader, Zod, scoped query cancellation and shared Console components. Do not introduce
a CMS or knowledge platform to serve a bounded first collection. These existing
capabilities and an embedded catalogue cover the proposed current user operation.

Reuse the existing Knowledge plain-text preview (`pre` with wrapped text) for the
initial authored UTF-8 guide. React renders text inertly; no Markdown parser, HTML
renderer or new dependency is needed. Provenance links are structured metadata,
validated as HTTPS URLs under the repository's GitHub location; body text cannot
add interactive links, remote images or executable markup.

## 5. Catalogue identity, version and publication

Each published article has a stable library/article identity, exact revision,
human title/description, bounded source metadata and content digest. Release
metadata identifies the first approved publish/update date; it must not use the
runtime clock or file modification time as a claimed editorial update.

Only approved catalogue entries are discoverable. Exact revision lookup must not
redirect to latest or resolve a missing revision to a different document. Changed
content receives a new revision and digest. An existing version remains immutable;
normal version history of this new system is not legacy migration.

Repository publication has no customer-facing write API, online publisher state
machine, external mutation, upload protocol or database transaction. Maintainers
review changes through the existing PR process. Serving construction reads packaged
facts only and does not publish content or install schema.

Withdrawing content or retaining old revisions needs an explicit product policy
before adding a runtime withdrawal API or guaranteeing historical model access.
In a reading-only release, absent/unpublished exact versions return unavailable;
there is no alternate owner or fallback. Wrong identity/digest fails closed.

## 6. Access, content safety and bounded reads

Reuse existing signed-in Workbench identity, fresh Effective Organization admission
and `workbench.knowledge.read`. Operator/admin grants remain as currently defined;
viewer/anonymous access is not expanded. `knowledge.manage` does not authorize
official publication. No new publisher identity or online mutation endpoint exists.

The public projection contains no enterprise documents, actor credentials, private
config, object keys or customer references. Official and enterprise IDs/routes remain
distinct even when the navigation places them together.

Use current live-access route descriptors and no-store identity-scoped BFF responses.
Every request rechecks identity/access; browser role hints cannot authorize content.
Context switches cancel old requests and clear selection/error state using current
Workbench conventions. Cache keys include actor/organization and exact content version.

Initial bounds: catalogue at most 100 entries/128 KiB, article source text
at most 64 KiB, projected article at most 256 KiB, provenance at most 10 bounded
entries, GET timeout at most 10 seconds, strict IDs/query shape, and rejected unread
GET bodies before reading. No arbitrary client-selected file paths/URLs or server
fetch is allowed. These are local safety limits, not a capacity-platform project.

Untrusted text remains supplemental information, even when curated by Sumi.
Content cannot become identity, permission, Product fact, validator exemption,
approval, paid budget or a provider instruction. Text renders inertly; no raw HTML,
script, iframe, remote image beacon, javascript/data URL or dynamic include.

Read cancellation, timeout or response loss performs no write. The user may retry
the same exact version GET; no UNKNOWN write protocol or reconciler is needed for
that reading path. Catalogue corruption or missing content fails construction/read,
never synthesizes a document or substitutes an enterprise store.

## 7. Frozen read contract

Module `official-knowledge` registers exactly these GET routes, under the current
identity/live-organization policies, existing knowledge.read and a 10-second deadline:

- `/api/v1/workbench/official-knowledge` → `{items: ArticleSummary[]}`.
- `/api/v1/workbench/official-knowledge/:article_id/revisions/:revision` → `Article`.

Summary fields: `id`, `revision`, `title`, `summary`, `category`, `updatedAt`,
`maintainedBy`, `digest`. Article adds `body` and `sources: [{title,url}]`.
The stable ID is a lowercase ASCII slug (1–64 characters); revision is a canonical
positive integer string (at most 9 digits); digest is lower-case SHA-256 of exact
UTF-8 body bytes. Strict catalogue construction rejects duplicate `(id,revision)`,
invalid metadata/digests, empty/oversized body, or invalid provenance. List shows
the greatest published revision for each ID; detail always reads the requested
exact revision. Missing ID/revision returns 404, invalid/query/body shape 400.

Catalogue content is embedded at build time, validated during construction and
cannot mutate at runtime. There is no draft catalogue API or dynamic withdrawal
contract. A later release may omit a revision; direct access then returns 404
without resolving latest. This release creates only one real guide/revision.

The current-application assembles the built-in module whenever Workbench is enabled,
independently of the enterprise Knowledge/Tika/database feature. Route admission
validates the exact descriptor. The existing Knowledge BFF is extended only for
these read shapes, preserving identity/cookie/token, strict response validation and
no-store; official reads use 10-second/128–256 KiB limits and reject request bodies
before reading. Unsupported mutation methods return 405 without forwarding.

Console uses `/workbench/ai/knowledge/official` and exact version detail
`/workbench/ai/knowledge/official/:articleId/revisions/:revision`. Identity/access
failure removes content and requires explicit context confirmation; context switches
and permission changes discard old query state. Detail validates returned identity
and revision against its URL. No AI selection, local storage, creation or upload.

## 7.1 AI consumption — Later, no implementation in this batch

Future formal AI work requires its own approved scope/contract. Existing code is
not directly extensible by placing official UUIDs in the enterprise selection field:

- `productAgentRequestBody.knowledgeSelection` accepts only `knowledgeBaseId`.
- `knowledge.ContextRequest` and its fingerprint admit `knowledge-base:<uuid>`.
- Context materialization locks enterprise Base/Source lifecycle rows and stores
  an organization-scoped immutable bundle and citation identities.
- Product Agent configuration, Chat/BusinessTask execution, model dispatch and
  Review provenance assume that existing contract.

A legitimate additive official-reference contract must distinguish content kind,
pin exact official revision/digest in the request fingerprint before Start, recheck
current read/access before dispatch/resume/citation projection, and reject missing
or changed revisions. It must reuse existing governed model, quote/budget/UNKNOWN,
Agent and Product Review/Apply owners. A template default is not execution permission.
No selection must keep the current no-Knowledge path.

Do not write an implementation against a speculative union type, add a second
Agent runtime, bypass live permission, or copy catalogue content into an enterprise
Base merely to satisfy the existing transport. Exact contract → materializer/reader
→ injection → consumers and related tests must be added here if this option is
chosen. Full historical model/Review behavior cannot be declared ready by the
reading-only proposal.

## 8. Menu/runtime dependency and isolation

The official owner and API proposal are independent of PR #631's menu changes.
The reading UI consumes PR #631's approved KnowledgeOverview/menu. On 2026-10-11
it remains OPEN at `7aebfeb16582d6417073fab2e38c8bfdb9b546df`. The official branch
merges that exact dependency and PR #633 targets `codex/data-services-runtime` as a
stacked PR. Its review diff contains only official knowledge work. Recheck dependency
state before the final candidate; do not merge #631 without authorization.

No changes are allowed in #631's worktree. The existing
`task-processor-unified-20261010` runtime, private configuration and all named volumes
remain outside this task's writes. A new isolated local instance or normal startup
instructions must use an approved candidate and current documented startup path;
do not turn a stopped verification fixture into a claimed user handoff.

## 9. Legacy and validation

Legacy decision: **N/A**. Reuse only current owner/component capabilities.
No legacy wrapper, historical data import, dual read/write or compatibility layer.

For a confirmed reading path, test meaningful behavior using TDD: catalogue validation,
exact-version/digest lookup, absent/unpublished versions, content/link safety, route
identity/live access, body/response bounds, BFF validation and actual list/detail/error
states. Run only relevant owner/BFF/UI checks plus applicable CI, then verify the
requested screen against the live Figma response using the shared shell.

For a confirmed AI path, add focused selection/idempotency/version/citation/access
tests around the existing real consumer. Controlled providers do not establish
paid-provider or product acceptance. Do not build a fault-injection platform.

One bounded independent architecture check follows frozen product decisions;
one final independent check follows a complete user path. Incremental findings are
classified under AGENTS.md before fixes. Implementation, CI, independent review,
normal login/use and user acceptance remain separate evidence.

## 10. Admission checklist

- [x] Current main, related Issues/PR, owners, Figma and applicable instructions inspected.
- [x] One Writer, isolated branch/worktree, dedicated execution Issue.
- [x] User confirms first real content and legal/usage provenance: repository-supported guide.
- [x] User confirms maintenance/publication and initial use scope: PR/release, reading only.
- [x] Scope-specific read contracts above frozen; AI/publisher UI explicitly Later.
- [x] Applicable independent architecture review completed; no BLOCKER.
- [x] Explicit `IMPLEMENTATION_READY`; Issue #632 records Ready before production edits.

Current result: **IMPLEMENTATION_READY** for this read-only guide. Implementation,
provider readiness and user/product acceptance remain separate evidence.
