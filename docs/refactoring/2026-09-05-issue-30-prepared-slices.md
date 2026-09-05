# Issue 30 prepared slices — 2026-09-05

Baseline: main `2fd42cc061fd689184892380771cdf292abeacb9`.
Branch: `codex/issue-30-product-sourcing`.

This is local contract evidence, **not Issue #30 completion or production
acceptance**. The user accepted the independent-review source-account ownership
BLOCKER and authorized only independent preparation. Production composition and
the old route remain unchanged. No numeric fallback, tenantbridge consumer or
compatibility wrapper was added.

## READY

| Slice | Local evidence | Production status |
| --- | --- | --- |
| Product-owned PublicationIdentity and bounded ProductKey | Red-before-green tests; exact 128-byte boundary, long/Unicode keys, stable product across source versions, explicit-run identity, canonical payload and lineage hashing | Not wired |
| Catalog replay/conflict | File-backed SQLite, close/reopen, same-payload replay, changed payload/lineage conflict, concurrent replay, cancellation, Organization isolation, immutable historical version | Existing Catalog persistence exercised through sourcing Publisher |
| Durable lineage | 1688 adapter → envelope → persisted ProductSnapshot; source ID/platform/reference/checksum/capture metadata/run/request/notes and warnings reread after reopening | Controlled fixture only |
| Store Center access | Existing Organization-scoped Store read contract; identity/platform/lifecycle, unavailable dependency, typed nil, cancellation, revocation on replay | New validation function not wired |
| Product/ApprovedAsset/readiness | Existing ImageAgent V3 approval publisher → file-backed Product Asset repository; wrong actor rejected, explicit approval replay, restart reread, tenant/product/platform/version isolation, canonical Listing preview attachment | Projection and durable object identity are fixtures; no browser, image generation, upload or human approval executed |
| HTTP bounded input/deadline | Unregistered handler; strict JSON, effective Organization/actor, rejects legacy fields, 2 MiB exact/over boundary, earlier deadline preserved, 30-second maximum, real HTTP slow-body read timeout | No module, route or production Importer implementation |
| Architecture guard preparation | Ban legacy ownership imports in prepared slices; negative import fixture; explicit prohibition on app composition importing prepared HTTP package while prerequisite is outstanding | Old-route absence guard deferred until authorized cutover |

`readiness.ProductInputs` is a pure **Product input gate**. Ready means that a
pinned Product snapshot has no outstanding source review and has matching
approved inventory. It does not establish marketplace category, payload,
commercial rules or submission readiness. Source warnings remain visible.

The HTTP maximum bounds body reads on net/http connections using
ResponseController and the Importer context. Importer implementations must
cooperate with cancellation; no goroutine is abandoned to simulate cancellation.
A timeout/response loss after commit requires replay with the same command and
source-run identity. Catalog owns conflict detection. The HTTP handler is not a
substitute for application authorization, and its fake Importer is test-only.

## BLOCKED

- Publication Identity Cutover is a production rollout BLOCKER: no-run retries
  spanning the historical checksum ID and prepared canonical-snapshot ID append
  duplicate publication/version facts. This is reproduced, NOT repaired here.
  See [the bounded prerequisite Issue draft](2026-09-05-publication-identity-cutover-issue-draft.md).
- Source-account ownership cannot currently be validated directly against the
  effective Organization. Existing account repository/crawler ownership is
  numeric. The accepted finding remains BLOCKER.
- No production application import service or source-account adapter has been
  created. Store-only or public-only fixtures are not evidence of account access.
- Production cutover, old-route removal and compatibility handoff retirement are
  paused. This work must not close Issue #30.

## WAITING_FOR_PREREQUISITE

Observed coordination: [PR #303](https://github.com/qq550723504/task-processor/pull/303)
is open and supplies source-account ownership **preflight slice A only**, under
#301. Its own scope explicitly leaves backfill (B), reader cutover (C) and
retirement (D) outstanding. A preflight receipt does not clear this blocker.

1. Organization-owned source-account contract and one-time ownership migration.
2. Verified handling of missing/ambiguous owner mapping, old crawler readers,
   persisted in-flight work and browser-profile references.
3. Reviewed cutover baseline and its failure/restart/authorization evidence.
4. Only then: implement and wire the application Importer, validate actual
   account/store access before publication and on replay, run controlled HTTP
   import → durable snapshot → explicit approval → downstream acceptance, and
   replace the temporary no-wiring guard with legacy route/import absence guards.

## Required source-account interface

The prerequisite owner must expose an Organization-native contract equivalent to
the following (proposed contract, deliberately **not** a compiled adapter here):

```go
type SourceAccessRequest struct {
    OrganizationID string // verified effective Organization, never home/legacy tenant
    ActorID        string // verified subject
    AccountID      int64  // opaque source-account identity, never an owner identity
    Platform       string // must match "1688"
}

type SourceAccess struct {
    OrganizationID string
    AccountID      int64
    Platform       string
    Mode           AccessMode // public or account_assisted, selected explicitly
}

type SourceAccessValidator interface {
    ValidateSourceAccess(context.Context, SourceAccessRequest) (SourceAccess, error)
}
```

Required semantics:

- OrganizationID and ActorID come from verified request context. A future Importer
  rechecks its command identity against that context and uses the same effective
  Organization for Store, source account and Catalog `TenantID`. No fallback to
  TenantID/HomeOrganizationID or parsing an Organization ID as an integer.
- HTTP source_account_id is required: omission and null are invalid. Explicit
  AccountID zero selects public access. A negative ID is invalid.
  A positive unavailable/disabled/deleted/wrong-platform/wrong-owner account fails
  closed; it must never fall back to public access. Missing validator fails closed.
- Access success returns exactly the requested Organization/account/platform and
  the matching access mode. The application rejects inconsistent receipts.
- Validate on every import, including replay. Honor context cancellation and the
  caller deadline; do not keep an application-level authorization cache. Revoked
  access blocks future commands; already committed Product history remains durable.
- Errors distinguish invalid request, unavailable/forbidden, disabled and temporary
  dependency failure for application handling. External messages must not disclose
  whether another Organization owns the account or expose cookies/profile paths.
- Return no credentials, cookies, proxy secrets or browser filesystem paths to
  Product/HTTP. The crawler retains responsibility for opaque runtime references.
- The prerequisite supplies the live-write Organization authorization middleware
  evidence. The unregistered HTTP contract uses EffectiveOrganizationID but does
  not create a second role/grant system.
- Migrate existing account ownership once, prove provenance, reject ambiguous or
  missing mappings, drain old numeric readers and specify in-flight/profile
  cutover. No request-time metadata bridge, dual ownership or compatibility wrapper.

## Verification commands

```powershell
go test ./internal/product/sourcing ./internal/product/catalog ./internal/storecenter ./internal/listing/readiness ./internal/listing/preview ./internal/app/productsourcing/httpapi ./internal/integration/crawler/a1688 ./internal/imageagent/assetpublication ./internal/integration/persistence/product/catalog ./internal/integration/persistence/product/asset -count=1
go test ./tests -run 'Test(Issue30|ProductDomainDoesNotDependOnOuterAdapters|Phase3ProductDomainBoundaryDepguardRuleCoversOnlyTargetSubpackages|Alibaba1688CrawlerDoesNotImportListingKitRoot|Alibaba1688CrawlerDoesNotConfigureLegacyTenantThroughListingKitHTTPAPI|Alibaba1688CrawlerDoesNotImportListingKitHTTPAPIForSourceAccountBuilder)' -count=1
```

Both passed locally. No staging/production acceptance, PostgreSQL runtime
acceptance, production route switch, merge or deployment is claimed.

`go vet` and `golangci-lint run --new-from-rev=origin/main` passed for the four
changed production packages (sourcing, Store Center, readiness and prepared HTTP).

The previous three documentation guard failures were recorded against old main
2fd42cc06. R1-B merged current main 1b82592e1, including #302 and #305.
That historical baseline-failure classification is superseded: current checks
must be assessed on the final PR HEAD and reported in the PR evidence.

Legacy decision: EXTRACT (prepared behavior); RETIRE is waiting for prerequisite.
Reusable behavior: deterministic source/publication identity, bounded keys,
validated Store access, explicit asset/readiness and HTTP contracts.
Current owner: Product sourcing, Store Center, Listing readiness, Application HTTP.
Cutover/deletion condition: prerequisite plus current-account application acceptance.

## R1-B prepared HTTP contract and rollout boundary

Baseline: remote PR HEAD 5704828ded; merged main 1b82592e1 without rewriting history.
This is a bounded transport correction plus characterization tests/documents;
no authorization implementation, persistent state transition or migration changes.

- HTTP owns the snake_case snapshot DTO and explicitly maps all current fields,
  including main_image, min_price, product_details and nested supplier/video/
  variant/pack/shipping fields. The local-agent protocol is the reference; its
  production handler is unchanged. Handwritten JSON fixture is protocol-shaped,
  not serialized from the integration Go model.
- Exactly one access selector: required numeric source_account_id. Zero explicitly
  means public; positive int64 means account. Omission, null, negative, fractional,
  string, overflow and alternate access_mode/public fields return 400
  invalid_request without invoking Importer. Failed account access returns 403
  source_access_denied and is never retried as public. Store ID is still required.
- source_identity exposes exactly source_type, source_platform, source_id,
  source_url, source_version and source_fingerprint, including empty values.
  It never serializes or fills fields from legacy Platform/Region/ProductID/StoreID.
- Strict unknown-field/trailing JSON rejection, exact 2 MiB limit, connection read
  deadline, maximum 30 seconds and earlier cancellation/deadline remain enforced.
- ImportCommand carries the converted integration snapshot; ImportResult is
  projected into HTTP response DTOs. No production Importer is implemented.

Finding: snapshot wire format cannot be consumed.
Product requirement affected: controlled snapshot import.
Classification: IMPLEMENTATION_TEST within the unregistered prepared slice.
Reason: DTO translation resolves the transport defect without changing domain or
authorization design; the core production path remains blocked separately.
Action: red snake_case fixture, explicit mapper, green focused HTTP tests.

Finding: omission/null selects public and SourceIdentity leaks legacy fields.
Product requirement affected: explicit source access and source-neutral contract.
Classification: IMPLEMENTATION_TEST.
Reason: correct the unregistered wire contract before any cutover; no new access
policy or compatibility obligation is introduced.
Action: required selector and exact allowlist response tests, red then green.

Finding: cross-cutover publication ID changes.
Product requirement affected: idempotent retry and immutable Catalog publication.
Classification: BLOCKER for production rollout (unsafe rollout/migration).
Action: durable restart reproduction and prerequisite draft; keep review open.
No fallback or migration is implemented. Prepared-slice merge eligibility requires
independent review accepting this isolation boundary; unwired code alone does not
establish that approval or resolve the defect.

Production inspection: app/httpapi/composition_builder.go and types.go still use
the old handoff; the old route and composition have no changes against main.
Production sourcing.PublicationIdentity callers are absent. The temporary guard
now rejects imports of the prepared package throughout internal and cmd, rather
than only internal/app/httpapi. Catalog/Store/readiness helpers remain unwired;
their fixture checks are not live provider, PostgreSQL or #30 acceptance.

Final HEAD test/CI receipts belong in the PR body and review replies so they can
name the actual immutable commit, rather than embed a self-referential SHA here.
