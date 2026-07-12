# SDS POD Canonical Revision Alignment Design

## Goal

When a SHEIN revision recomputes derived state for a task with completed SDS POD output, restore the same canonical product facts that the normal SDS workflow applies: title, identity attributes, product images, per-variant images, and Studio AI style.

## Problem

The normal SDS workflow calls `applySDSSyncMetadataToCanonical`, which projects `SDSSyncSummary` and `SDSSyncOptions` into neutral `sdspod.CanonicalMetadata` before calling `sdspod.ApplyCanonical`.

The revision path currently calls `sdspod.ApplyCanonical` directly with only `StyleName`. A later revision can therefore retain stale title, identity, or rendered-image facts even though the task already has authoritative completed SDS output.

## Design

### Ownership

- `internal/product/sourcing/sdspod` continues to own deterministic application of neutral canonical metadata. It remains independent of ListingKit DTOs, persistence, runtime clients, and SHEIN packages.
- Root `internal/listingkit` continues to own task/result DTO adaptation and revision orchestration.
- The existing root adapter `applySDSSyncMetadataToCanonical` remains the sole conversion path from `SDSSyncSummary` and `SDSSyncOptions` to `sdspod.CanonicalMetadata`.

### Revision alignment

In `refreshSheinDerivedState`, replace the style-only direct `sdspod.ApplyCanonical` invocation with the existing root adapter using:

- `task.Result.CanonicalProduct`;
- `task.Result.SDSDesignResult`; and
- `task.Request.Options.SDS`.

The call happens at the same current position before building the SHEIN publish request. Category resolution, attribute regeneration, review-state refresh, persistence, and all remote interactions remain unchanged.

### Empty and partial data

- A missing canonical product, SDS summary, or SDS options remains safe: the adapter does not invent canonical facts and only applies available non-empty values.
- Existing `sdspod.ApplyCanonical` idempotence remains the source of truth. Repeating the revision refresh with unchanged data does not modify the canonical product.
- Completed SDS summary fields take precedence exactly as in the normal workflow; options provide the existing fallback product name, style, and attribute values.

## Tests

Add a service revision regression test with a task whose canonical product contains stale title, attributes, and images while its completed SDS summary contains current product, variant, and rendered-image metadata. Assert that a qualifying SHEIN revision refresh restores the same canonical values as the normal SDS workflow.

Also cover the no-SDS case to verify revision refresh does not mutate canonical facts when no authoritative SDS result exists.

Keep the existing `sdspod` package tests and its import boundary guard unchanged; add a root boundary assertion only if the implementation needs a new adapter or bypass risk.

## Non-goals

- Do not change the SDS result schema or `sdspod.CanonicalMetadata` API.
- Do not move ListingKit DTOs into `sdspod`.
- Do not change SHEIN category, attribute, price, review, persistence, or remote submission behavior.
- Do not backfill or rewrite already-persisted tasks outside an explicit revision refresh.
