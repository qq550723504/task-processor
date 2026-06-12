# Legacy Store Routing Retirement Checklist

> Status: completed. This note records the retirement of legacy `/store-routing` behavior and the remaining historical context.

## Goal

Retire legacy SHEIN store routing settings safely, without breaking:

- historical task / preview snapshots,
- existing handler / route tests,
- constructor and repository wiring,
- any external caller still depending on `/api/v1/listing-kits/store-routing`.

## Already Done

- Runtime store selection no longer depends on store routing settings.
- Frontend no longer exposes store routing UI or task-create auto-routing behavior.
- Legacy routing code paths are isolated into dedicated `*_legacy*` files.
- `httpapi` and `service` wiring now label this dependency as `legacy`.
- Legacy store routing service entrypoints now behave as a compatibility shell and return synthesized `manual` defaults instead of persisted state.
- Main ListingKit service construction no longer injects legacy routing repositories into the active service path.
- Legacy routing repository builders, persistence implementations, and persistence-only tests have been removed.
- Legacy `/api/v1/listing-kits/store-routing` route, handler, service entrypoints, and interface requirements have been removed.
- Route/handler/service tests and stubs were pruned to match the simplified store-profile-only admin surface.

## Remaining Historical Surfaces

### 1. Historical task data

Legacy routing fields may still appear in historical task / preview snapshots created before the refactor:

- historical snapshot payloads in stored tasks

Follow-up decision if desired:

- leave these historical fields as-is for backward readability, or
- rename/document them more explicitly as legacy snapshot data.

### 2. Documentation debt

Some refactoring notes may still mention intermediate compatibility files that have now been deleted:

- `docs/refactoring/service-slimming-checkpoint.md`
- older builder review / planning notes under `docs/`

## Final Outcome

- Store selection now depends on the active workspace/store choice and store profile data only.
- No dedicated store-routing repository, builder, service facade, or HTTP route remains.
- Store admin APIs are reduced to the capabilities that are still exercised by the product.

## Verification Notes

- Persistence seam removal was covered by targeted `internal/listingkit/...` tests.
- Route/interface removal was covered by targeted `internal/listingkit/...` and `internal/app/httpapi/...` tests.
