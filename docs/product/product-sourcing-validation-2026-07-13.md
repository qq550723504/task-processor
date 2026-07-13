# Product Sourcing MVP Validation Checkpoint — 2026-07-13

## Baseline and scope

- Commit: `49a202c0a964e54f9864b3a57be5bed4bfbf2cf1`
- Scope: deterministic repository validation for the current Product Sourcing MVP foundation.
- Excluded: uncontrolled browser crawling, live store submission, and a real 1688 source-to-task operator run.

## Passed validation

The following command completed successfully with `GOWORK=off`:

```powershell
go test ./internal/product/sourcing/... ./internal/catalog/... ./internal/asset/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... -count=1
go test ./internal/listingkit/... ./tests/... -count=1
```

This verifies the neutral source contracts, Amazon and 1688 envelope mappings, catalog/asset handoff, 1688 task-creation adapter, ListingKit bridge, and the repository import-boundary guards.

The maintained production entrypoints also built successfully with the Makefile-equivalent command:

```powershell
$env:CGO_ENABLED='0'; $env:GOOS='linux'
go build ./cmd/listing-control-plane ./cmd/product-listing-api ./cmd/shein-listing ./cmd/temu-listing
```

`make build-all` itself could not run because GNU Make is not installed in the Windows validation environment; the equivalent target commands above were used instead.

## Remaining MVP closeout work

The repository foundation is verified, but the MVP is not yet operationally closed. A controlled 1688 flow must still record:

1. source URL or source ID, normalized identity/key, and warnings;
2. tenant/user context, generated ListingKit request, and created task ID;
3. durable source lineage plus the preview/readiness outcome;
4. a missing-fact or adapter-error outcome with an actionable failure phase; and
5. confirmation that the existing SHEIN preview/submission path remains the downstream owner.

Do not treat a prior `go test ./...` timeout without output as a passing full-backend result. It remains a separate validation-environment issue to classify before declaring a full-suite gate green.

## Checkpoint decision

Product Sourcing code boundaries and deterministic integration seams are ready for the controlled 1688 validation. No additional package extraction is justified before that runtime evidence exists.
