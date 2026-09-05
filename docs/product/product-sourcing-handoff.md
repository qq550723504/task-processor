# Product Sourcing Handoff

> ACTIVE: owner and handoff guide under the current Product Domain contract.
> CURRENT STATE observations: `main @ cae67730c5c0e645d708cb2f6814f14781962bb1`.
> Execution scope/order: [#30](https://github.com/qq550723504/task-processor/issues/30),
> [#307](https://github.com/qq550723504/task-processor/issues/307) and
> [#137](https://github.com/qq550723504/task-processor/issues/137).

## 1. Authority and purpose

New source work reuses the [Product Domain contract](../superpowers/specs/2026-09-01-internal-target-architecture-phase3-product-design.md),
[current package mapping](../refactoring/module-target-mapping.md) and
[Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md).
Source adapters collect evidence; Product Sourcing normalizes it; Catalog owns
canonical facts; Product Asset owns approved assets. Marketplace rules remain
downstream. A source platform is not a target sales platform.

The [clean-slate decision](issue30-clean-slate-cutover.md) replaces old source-to-task
acceptance and historical publication migration requirements. It does not relax
new-system idempotency, authorization, immutable history or explicit approval.
Final navigation follows [final UI / IA](final-ui-ia-authority.md): shared domain
facts do not require a top-level Listing Center or a platform Workbench.

## 2. Current contracts and owners

| Responsibility | Current owner | Boundary |
| --- | --- | --- |
| Fetching, parsing, provider DTOs | `internal/integration/crawler/a1688`, `internal/integration/crawler/amazon` | Adapter-local snapshots become neutral `SourceEnvelope`; no Product service/workflow ownership |
| POD/design evidence | `internal/sds/adapter/product_source` | Preserve SDS specialization; not generic source onboarding |
| Identity, normalization, provenance, warnings | `internal/product/sourcing` | `SourceIdentity`, `SourceEnvelope`, `Normalize`, `ToSnapshot`; no crawler/integration/runtime imports |
| Canonical publication | `internal/product/sourcing.Publisher` → `internal/product/catalog.Publisher` | Explicit tenant/product/publication identity; Catalog owns `ProductSnapshot`, immutable versions and replay/conflict semantics |
| Approved assets | `internal/product/asset` | `ApprovedAsset`, approval provenance and repository ports; source images remain candidates |
| Image capabilities and execution | `internal/product/image` + `internal/imageagent` | Capability ports return candidates; ImageAgent owns image workflow/budget/retry/recovery/approval |
| Access and orchestration | Current application + Organization/Store/source-account owners | Verified effective Organization and actor; recheck access before publication, including replay |
| Platform adaptation/readiness/submission | `internal/marketplace/*` and existing Listing owners | Deterministic validation and one submission owner |

`internal/catalog`, `internal/asset`, `internal/productimage`,
`internal/productenrich` and `internal/imageasset` are retired roots; do not
recreate them. Existing `internal/crawler`, root ListingKit and
`internal/compatibility/*` are extraction/retirement areas, not preferred homes.

## 3. Identity and evidence

Preserve source type/platform, source-native ID, canonical URL when available,
source version/fingerprint, raw reference, warnings and trace/lineage according
to [SourceEnvelope](../../internal/product/sourcing/source_envelope.go) and
[SourceIdentity](../../internal/product/sourcing/source_identity.go).
Do not manufacture missing product facts or confuse evidence metadata with
authority. Adapter DTOs, browser clients and marketplace payloads stay outside
the neutral model.

[Publication identity](../../internal/product/sourcing/publication_identity.go)
is distinct from source identity. Use the approved deterministic helper and
explicit publication contract; do not infer identity from legacy task results
or introduce checksum fallback. New-system same explicit key plus same payload
and effective lineage replays; changed payload conflicts. Content changes can
produce a new content-addressed publication. Catalog owns the version result.

## 4. Target flow and observed implementation

The approved target is:

```text
explicit new import with verified effective Organization / actor / access
  -> source adapter -> SourceEnvelope
  -> sourcing.Publisher -> catalog.Publisher -> ProductSnapshot
  -> explicit asset approval -> ApprovedAsset
  -> deterministic marketplace/listing readiness
```

This is target acceptance, not completed wiring. On the baseline above:

- [App composition](../../internal/app/httpapi/composition_builder.go) still wires
  `internal/compatibility/listingkit/sourcehandoff/a1688` to
  `/api/v1/product-sourcing/1688/listingkit/tasks`. The old command publishes
  through the current Publisher before legacy task creation. This is CURRENT
  STATE / drain-only debt under #30, not the new source contract.
- [Catalog composition](../../internal/app/httpapi/product_catalog_composition.go)
  already constructs the sourcing/Catalog Publisher and durable reader.
- [Prepared HTTP contract](../../internal/app/productsourcing/httpapi/handler.go)
  is merged but unregistered. Its Importer port must revalidate access; the
  handler alone is not a complete application import service or rollout.
- Source-account preflight from #303 is merged. This does not prove
  Organization/profile migration or #30 production cutover is accepted.

Do not extend the old handoff, add consumers, wrap it for new sources, or
automatically forward old requests into new imports. #30/#307 own separately
approved cutover and route retirement; this guide does not authorize it.

## 5. Acceptance and stop lines

Use the [MVP closeout guide](product-sourcing-mvp-plan.md) and current Issue for
the chosen slice. Required invariants remain:

- Product packages do not import ListingKit, compatibility, crawler/integration,
  HTTP runtime or concrete providers. Integration may consume the neutral contract.
- New imports preserve warnings, durable lineage and exact Catalog identity.
  Source images are not automatically approved; absent approval means not ready.
- Organization/account/store ownership, disabled/deleted/revoked access and
  cross-Organization isolation are checked by existing authorities.
- Old product/asset/task state and late execution results cannot enter the new
  approved business scope. Protected account/profile, IAM, Store, financial and
  external-effect evidence is not disposable business data.
- No legacy fallback, dual-read/write, second Product fact source, new submission
  owner or new IAM is introduced.

If an approved contract cannot satisfy the slice, name the concrete conflict
and owner in its Issue. Do not revive old packages. No new source or target
expansion is scheduled here; use #137 and a bounded execution Issue. Fixtures,
local integration and production acceptance are separate evidence classes.

## 6. Historical evidence

The [previous guide at the audit baseline](https://github.com/qq550723504/task-processor/blob/73aa79cb98def16536123dc1ea3f55c578ed253b/docs/product/product-sourcing-handoff.md)
records the former ListingKit-task model. It is historical evidence, not a
current expansion guide. Dated validation does not validate today's HEAD.
