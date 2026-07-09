# Product Sourcing Handoff

> Status: active product-source expansion guide.
>
> Last reviewed: 2026-07-09.
>
> Scope: source identity, crawler handoff, source-result normalization, catalog/asset handoff, and ListingKit stop lines for new product-source work.

## 1. Purpose

The next growth direction is product-source expansion before full new sales-platform workbench expansion.

This document defines how new product sources should enter the system without pushing source-specific logic into root `internal/listingkit` or marketplace publishing packages.

The main rule is:

```text
crawler/integration packages collect raw source data;
product sourcing normalizes source identity and source facts;
catalog and asset packages own platform-neutral product and asset facts;
ListingKit consumes normalized product/source facts through a narrow orchestration boundary;
marketplace packages adapt those facts to platform-specific publishing and workspace rules.
```

## 2. Current source classes

Current and planned source classes:

| Source class | Current examples | Primary owner |
| --- | --- | --- |
| Marketplace/crawler source | `1688`, Amazon crawler facts | crawler/integration adapter plus product sourcing normalization |
| POD/design source | `SDS` | SDS adapter plus product sourcing / asset normalization |
| Warehouse/source catalog | planned overseas warehouse sources, such as 大建云仓 | product sourcing normalization plus catalog/asset handoff |

A source is not the same thing as a target sales platform.

- Source platform answers: where did the raw product/design facts come from?
- Target platform answers: where will the Listing package be adapted or published?

## 3. Source identity

Every source handoff should preserve a stable source identity before ListingKit or marketplace code sees the product.

Minimum identity fields:

```text
source_type        // crawler, pod_design, warehouse_catalog, manual_import, etc.
source_platform    // 1688, sds, amazon, dajian, etc.
source_id          // source-native product/design/listing identifier
source_url         // optional canonical source URL when available
source_version     // optional source-side version/hash/snapshot id
source_fingerprint // normalized fingerprint for dedupe/idempotency when source_id is weak
```

Source identity must be stable enough to support:

- deduplication,
- re-import checks,
- source refresh,
- asset reuse,
- cost/source-SDS mapping,
- task lineage,
- batch retry and recovery explanations.

## 4. Package responsibilities

### 4.1 Crawler and integration packages

Preferred homes:

```text
internal/integration/crawler/*
internal/crawler/* during legacy migration
```

Own:

- raw collection from source systems,
- browser/API/runtime client execution,
- source-side pagination or fetch orchestration,
- raw payload capture and technical retry behavior,
- source-specific extraction that is required to make the raw payload usable.

Must not own:

- ListingKit task orchestration,
- marketplace publish payloads,
- SHEIN/TEMU/Amazon workspace rules,
- generic product identity semantics beyond raw extraction,
- cross-source canonical product decisions.

### 4.2 Product sourcing

Preferred home:

```text
internal/product/sourcing
```

Own:

- source identity normalization,
- source result envelopes,
- source provenance and fingerprinting,
- source-to-catalog handoff contracts,
- source-to-asset handoff contracts,
- source-level validation that is independent from target marketplace rules,
- source facts readiness before ListingKit orchestration.

Must not own:

- marketplace category/attribute rules,
- SHEIN/TEMU/Amazon publish payload construction,
- ListingKit API DTO shells,
- browser or external API client construction.

### 4.3 Catalog

Preferred home:

```text
internal/catalog
```

Own:

- platform-neutral product facts,
- canonical product title/description/attribute structures,
- variant/option/spec facts,
- product identity that is no longer source-runtime-specific,
- facts that can be reused across multiple target platforms.

Must not depend on:

- root `internal/listingkit`,
- marketplace workspace or publishing packages,
- HTTP runtime assembly.

### 4.4 Asset

Preferred home:

```text
internal/asset
internal/productimage when the current image pipeline owns the behavior
```

Own:

- reusable image facts,
- design/mockup/variant asset facts,
- asset bundle construction,
- platform-neutral image processing and normalization,
- POD source image/template handoff.

Must not own:

- target-marketplace image policy unless it is explicitly platform-neutral,
- SHEIN/TEMU/Amazon submit payload assembly,
- ListingKit task persistence ordering.

### 4.5 ListingKit

Preferred home for compatibility/orchestration only:

```text
internal/listingkit
```

Own:

- API-facing orchestration,
- task and batch creation coordination,
- compatibility DTO adaptation,
- tenant/user context handoff,
- references from source/catalog/asset facts into Listing tasks.

Must not own:

- new source-specific extraction rules,
- canonical product facts,
- reusable asset facts,
- new marketplace publishing policy,
- crawler runtime clients.

### 4.6 Marketplace packages

Preferred homes:

```text
internal/marketplace/shein/*
internal/marketplace/amazon/*
internal/marketplace/temu/*
```

Own:

- target-platform category/attribute/image/SKU/price rules,
- marketplace payload preparation,
- workspace review/repair rules,
- remote result and error interpretation,
- target-specific readiness or blocker mapping.

Must not own:

- crawler/source execution,
- cross-source product identity,
- platform-neutral catalog facts.

## 5. Recommended handoff flow

Use this flow for new source work:

```text
raw source fetch
  -> source adapter result
  -> product sourcing normalized envelope
  -> catalog facts + asset facts
  -> ListingKit task/batch orchestration
  -> marketplace-specific adaptation
```

A minimal normalized envelope should include:

```text
SourceIdentity
RawSourceReference
CanonicalProductCandidate
AssetCandidates
CostOrSupplierFacts, when available
Warnings / MissingFacts
Trace / Debug metadata for operator support
```

## 6. Stop lines for new source work

Do not:

- add `if source == ...` policy branches to root `internal/listingkit` unless it is temporary API-shell adaptation with a follow-up;
- make crawler packages import root `internal/listingkit`;
- make crawler packages import marketplace publishing/workspace packages;
- put target-marketplace category or publish rules in `internal/product/sourcing`;
- let source adapters construct SHEIN/TEMU/Amazon publish payloads;
- use generated package maps or stale dependency baselines as source ownership evidence;
- start full new sales-platform workbench expansion while product-source normalization is still unclear.

## 7. Review checklist

Before merging a product-source change, check:

```text
[ ] The source adapter can run without importing root internal/listingkit.
[ ] Product/source identity is explicit and stable enough for dedupe or retry.
[ ] Platform-neutral facts are handed to catalog and asset packages, not marketplace packages.
[ ] Target-platform rules stay in marketplace packages.
[ ] ListingKit only orchestrates or adapts DTOs around normalized facts.
[ ] Tenant/source ownership is preserved through the handoff.
[ ] Tests cover the normalized source envelope or the chosen source identity rule.
```

## 8. First safe slices

Good first slices:

1. Inventory current 1688 and SDS source identity fields and normalize naming.
2. Add a small `internal/product/sourcing` envelope type that does not import ListingKit.
3. Move one source-result normalization rule out of crawler or ListingKit into `internal/product/sourcing`.
4. Add tests proving crawler/integration packages do not import ListingKit, marketplace publishing, or marketplace workspace packages.
5. Document source-to-catalog and source-to-asset field gaps before implementing new source ingestion.

Avoid first slices that require:

- new marketplace publish runtime,
- database schema changes,
- broad DTO rewrites,
- changes to public ListingKit API contracts,
- product-source work and sales-platform expansion in the same PR.
