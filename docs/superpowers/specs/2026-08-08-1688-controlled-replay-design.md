# 1688 Controlled Replay Validation Design

## Goal

Close the deterministic validation gap in the Product Sourcing MVP by replaying one fixed 1688 product through the existing HTTP handoff and ListingKit task-creation boundary, while keeping real crawling, store submission, and marketplace ownership out of scope.

## Context

The repository already has neutral source contracts, 1688 source-envelope mapping, catalog/asset facts handoff, a ListingKit `GenerateRequest` bridge, and a 1688 HTTP task-creation adapter. Existing tests verify these pieces separately, but there is no single replay that proves the HTTP request, authenticated tenant/user context, source lineage, generated request, and task response remain connected.

The replay is validation evidence, not a production crawler. It must not claim that a real 1688 operator flow, asynchronous task execution, preview generation, or SHEIN submission succeeded.

## Chosen approach

Add a cross-boundary Go test that uses:

1. A deterministic `Product1688` fixture with title, category, brand, images, cost, supplier, description, and one variant.
2. The real `sourcea1688.Handler` and real `a1688.TaskCommandService`.
3. An in-memory `GenerateTaskCreator` that captures the generated ListingKit request and returns a deterministic task.
4. An authenticated request context carrying the tenant and user identity.

This approach is preferred over an HTTP-only contract test because it exercises the real command service and handoff. It is preferred over a database-backed staging harness because the first objective is to verify data propagation and error semantics without requiring credentials, a running database, a worker, or a live marketplace.

## Data flow

```text
fixed Product1688 fixture
        |
        v
POST /api/v1/product-sourcing/1688/listingkit/tasks
        |
        v
sourcea1688.Handler
        |
        v
a1688.TaskCommandService
        |
        +--> Alibaba1688SourceEnvelope
        |       +--> SourceIdentity / SourceKey
        |       +--> source warnings
        |       +--> catalog and asset facts
        |
        v
listingkit.GenerateRequest
        |
        v
in-memory GenerateTaskCreator
        |
        v
deterministic task response and captured request
```

The existing ListingKit task result, preview, readiness, and SHEIN submission code remains the downstream owner. This change does not add a source-specific preview or submission path.

## Test scenarios

### Successful replay

The test posts the fixed product with:

- verified tenant and user identity;
- source store and SHEIN target store IDs;
- `shein` as the target platform;
- source run ID, request ID, and raw snapshot metadata.

It asserts:

- HTTP success and deterministic task ID;
- normalized `crawler:1688:<id>` source key and normalized source URL;
- source warnings are empty for complete input;
- tenant and user context are copied into the generated request;
- title, brand, category, description, price, images, and variant facts reach the request;
- the request contains one normalized `shein` platform and the target store ID;
- the generated request carries the source reference used by the existing task persistence/result projection;
- the fake creator is called exactly once.

### Missing-facts replay

The test posts a product without a title and without usable images. It asserts:

- the handler returns a client error with `task_creation_failed`;
- the response includes the normalized source identity and source warnings;
- the error identifies that the 1688 source cannot create a ListingKit task;
- the task creator is not called.

### Source-error replay

The test posts a valid source identity with `source_error`. It asserts:

- the handler returns a client error;
- source identity and the source-error warning remain observable;
- the task creator is not called.

## Error and evidence boundaries

- Store access failures remain the existing 403 behavior and are not reclassified by the replay.
- Authentication and tenant mismatch behavior remain covered by existing handler/service tests; the successful replay uses the same authenticated context contract.
- The replay must not invoke a crawler, real store client, database, worker queue, preview generator, or SHEIN submission client.
- The validation report must distinguish deterministic replay evidence from real runtime acceptance. Preview/readiness and real task IDs remain pending until a controlled environment run records them.

## Files

- Create `tests/a1688_source_to_task_flow_test.go` for the cross-boundary replay scenarios.
- Reuse the existing inline fixture shape from `tests/a1688_source_facts_flow_test.go`; extract only a helper if doing so removes duplication without changing production behavior.
- Create `docs/product/validation/2026-08-08-1688-controlled-replay.md` with the exact commit, commands, scenarios, and explicit unverified runtime gates.
- Do not modify production code unless the new replay demonstrates a real propagation or error-reporting defect.

## Acceptance criteria

1. The three replay scenarios pass with the real HTTP handler and real 1688 command service.
2. `go test ./internal/product/sourcing/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... ./tests/... -count=1` passes.
3. The full backend test result is recorded separately; a timeout or unavailable environment is not reported as success.
4. The validation note records the exact commit and clearly marks real preview/readiness and operator acceptance as pending.
5. No new source-specific marketplace or submission owner is introduced.
