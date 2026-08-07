# 1688 Source Lineage Through ListingKit Tasks

## Status

Design approved for implementation planning.

## Goal

Preserve the normalized 1688 source identity when a source envelope is handed
through catalog and asset facts into an existing ListingKit task, so operators
and later workflow stages can trace a task back to its source without
reconstructing identity from a URL.

The supported flow is:

```text
1688 source result
  -> SourceEnvelope
  -> catalog.ProductFacts / asset.Facts
  -> ListingKit GenerateRequest
  -> persisted Task
  -> existing SHEIN preview and readiness path
```

## Context

The repository already contains the neutral source contracts, 1688 envelope
mapping, catalog/asset handoff, ListingKit request bridge, 1688 command service,
and product-sourcing HTTP route. The current bridge copies the source URL into
`GenerateRequest.ProductURL`, but it does not retain the normalized source key,
source platform, or source ID as an explicit request field. That makes source
lineage dependent on URL parsing and leaves the new source handoff path without
a durable, typed reference on the created task.

The existing ListingKit task lifecycle, SHEIN preview/readiness flow, tenant
authorization, and submission state machine remain the owners of their current
responsibilities.

## Design

### 1. Add a neutral request-level source reference

Add a small `listingkit.SourceReference` value to `GenerateRequest`:

```go
type SourceReference struct {
	Key      string `json:"key,omitempty"`
	Type     string `json:"type,omitempty"`
	Platform string `json:"platform,omitempty"`
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
}
```

The type is intentionally owned by ListingKit's request compatibility model,
not by `internal/product/sourcing`, so product sourcing does not depend on the
orchestration package. It contains identity metadata only; it does not carry
raw crawler payloads, source snapshots, marketplace fields, credentials, or
publishing state.

### 2. Populate the reference at the neutral facts bridge

Extend `SourceFactsGenerateRequestInput` with a source reference derived from
`catalog.ProductFacts`, or derive it in one place inside
`GenerateRequestFromSourceFacts`. The implementation must preserve:

- `ProductFacts.SourceKey` as `SourceReference.Key`;
- `ProductFacts.SourceType` as `SourceReference.Type`;
- `ProductFacts.SourcePlatform` as `SourceReference.Platform`;
- `ProductFacts.SourceID` as `SourceReference.ID`;
- `ProductFacts.SourceURL` as `SourceReference.URL`.

The existing URL, text, image, platform, and category mappings remain
unchanged. Empty source identity fields remain empty for legacy requests that
start from manually supplied text or images.

### 3. Preserve legacy JSON and persistence behavior

The new field is optional and uses `omitempty`. Existing request JSON without a
source reference must continue to deserialize and execute as before. The
existing JSON/GORM task persistence mechanism must persist the expanded request
payload without a new table or migration.

No existing task list filter, access-scope rule, retry path, preview builder, or
SHEIN submission payload should treat the source reference as a new state
machine or authorization source.

### 4. Keep warning ownership unchanged

This slice does not move source warnings into the ListingKit request. Source
warnings continue to be represented by `catalog.ProductFacts.Warnings`, asset
facts warnings, source handoff errors, and the existing generated result/review
metadata. The implementation must add tests proving that adding the source
reference does not suppress or rewrite warning behavior.

If a controlled request fails before task creation, the existing 1688 handoff
response continues to return source identity and warnings from its prepared
handoff. This slice does not add a second warning schema.

## Data flow

```text
SourceEnvelope.Identity
        |
        v
CatalogProductFactsFromEnvelope
        |
        v
catalog.ProductFacts.SourceKey/Type/Platform/ID/URL
        |
        v
GenerateRequestFromSourceFacts
        |
        v
listingkit.GenerateRequest.Source
        |
        v
Task.Request persisted as existing JSON payload
```

The existing 1688 HTTP handler and command service require verified tenant and
user identity and validate source/target store access before task creation. The
new field must not change those checks.

## Error handling

- Missing source identity is not manufactured or inferred beyond the existing
  `ProductFacts` values.
- A source error that currently blocks task creation continues to return the
  existing error and prepared handoff details.
- A valid legacy request with no source identity remains valid.
- Serialization errors, if encountered while persisting the expanded request,
  continue through the existing task creation error path.

## Testing strategy

Tests are written before production changes.

1. Add a ListingKit bridge test proving a complete `ProductFacts` identity is
   copied into `GenerateRequest.Source`.
2. Add a bridge test proving a legacy facts request with no identity keeps an
   empty optional source reference and preserves existing mappings.
3. Add a 1688 handoff/command test proving the created task request retains
   `crawler:1688:<id>` and the normalized source URL.
4. Add or extend a controlled HTTP integration test for
   `POST /api/v1/product-sourcing/1688/listingkit/tasks`, asserting the response
   and created request retain the authenticated tenant/user and source identity.
5. Run focused source, catalog, asset, handoff, HTTP adapter, and ListingKit
   tests. Run the full backend test command with a bounded timeout and report a
   timeout separately from a test failure if the repository baseline does not
   finish.

## Non-goals

- No `listing-ai-service` process or deployment.
- No new database table or migration.
- No Consumer Portal, workspace, subscription, or course-system work.
- No new source-specific branch in root ListingKit orchestration.
- No raw 1688 crawler payload in `GenerateRequest`.
- No change to SHEIN preview, readiness, publish, retry, or submission ownership.
- No change to the existing legacy `/api/v1/listing-kits/generate` contract.

## Acceptance criteria

- A normalized 1688 source identity survives the source-facts bridge into the
  persisted ListingKit task request.
- The source reference is absent, not fabricated, for legacy requests without
  source facts.
- Existing 1688 warning/error behavior remains unchanged.
- Existing tenant/store authorization remains unchanged.
- Existing SHEIN preview/readiness tests remain green.
- No new service, table, state machine, or marketplace owner is introduced.
