# Canonical Product Source Lineage Design

**Date:** 2026-08-07
**Status:** Approved for implementation

## Goal

Make the standard-product detail page preserve the user's ability to trace a canonical product back to its persisted source reference.

## Context

ListingKit already persists a task-level `SourceReference` and exposes it on the task-detail response. The canonical-product detail read model currently returns only the canonical product, summary, and field traces, so navigating from the task/status flow to the standard-product detail loses the source context.

The source reference is lineage metadata, not a canonical product fact. It must remain outside `canonical.Product` and outside marketplace-specific draft payloads.

## Design

### Frontend read model

Extend the frontend `CanonicalProductDetail` read model with an optional `ListingKitSourceReference` field. `buildCanonicalProductDetail` copies the already parsed task-result `source_reference` into this read model. The backend canonical product persistence model and task result endpoint remain unchanged.

The source is optional. Legacy tasks and task results without a source reference continue to produce the same canonical product detail shape apart from the omitted optional field.

### Frontend detail page

Reuse `ListingKitSourceReference` from the existing ListingKit task types. The canonical-product data mapper carries `result.source_reference` to the detail page. A small source-lineage card renders:

- `任务来源` as the section label;
- platform and source ID when available;
- the persisted source URL as an external link with `target="_blank"` and `rel="noreferrer"`;
- no card when the reference has no non-blank identity fields.

The page will not fetch the task again, reconstruct the source from browser storage, or make the source editable.

### Error and compatibility behavior

- Missing or blank source fields are treated as no source and render no card.
- A missing source does not make canonical-product detail unavailable.
- Existing canonical product, image, attribute, variant, and field-trace rendering is unchanged.
- The external link is rendered only when a non-blank persisted URL exists.

## Alternatives considered

1. **Recommended: extend the canonical detail read model.** One existing task-result request supplies both product data and lineage, with no duplicate persistence or extra network request.
2. Fetch the task-detail endpoint separately from the canonical detail page. This adds a second request and couples the page to two loading/error states for data already available in the canonical detail call.
3. Add source fields to `canonical.Product`. This mixes lineage metadata into platform-neutral product facts and risks copying source data into downstream marketplace payloads.

## Testing

- Frontend mapper tests verify source-reference propagation and omission for legacy task results.
- Component tests verify identity rendering, safe external-link attributes, and the empty-reference behavior.
- Existing canonical-product and full repository/UI suites remain required before handoff.

## Scope exclusions

- No database migration or new table.
- No new endpoint.
- No source editing, retry behavior, or marketplace payload changes.
