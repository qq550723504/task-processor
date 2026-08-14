# PAY-041 usage ledger reconciliation and PAY-042 handoff

## Scope and safety boundary

PAY-041 provides a durable, idempotent local usage ledger. This handoff adds a
read-only reconciliation helper; it does not connect paid feature entrypoints,
change a payment provider, cut over counters, or deliver events to OpenMeter.

`listingsubscription.ReconcileUsageLedger(ctx, repo)` is a dry-run helper. It
performs ordered `SELECT` queries over `saas_usage_events`,
`saas_usage_buckets`, and `saas_usage_event_outbox`, and returns
`DryRun: true`. It does not create, update, delete, retry, replay, reverse, or
otherwise repair any production row. Operators must treat findings as evidence
for investigation, not as authorization for a data mutation.

The report compares:

- bucket `committed` against the sum of `committed` and `reversed` event
  quantities for the same tenant/module/period/metric;
- bucket `reserved` against the sum of `reserved` event quantities;
- every ledger event's outbox identity and every outbox item's ledger-event
  identity; and
- failed outbox delivery state without exposing `last_error` or event metadata.

Each attributable finding has tenant, module, metric, period, event ID, and an
operator-safe reason. Outbox rows without an event retain only their event ID:
tenant and metric are intentionally not invented. A valid released reservation
is not a false positive: the current PAY-041 transaction creates one outbox
identity while reserving, and reconciliation only reports absent/orphaned or
failed identities.

## Operator invocation

The production caller must construct the existing `*GormRepository` from the
normal application database configuration and invoke the helper in a read-only
administrative command. The helper itself is the command boundary:

```go
report, err := listingsubscription.ReconcileUsageLedger(ctx, repository)
if err != nil {
    return err
}
// Persist or emit only report.DryRun and report.Findings through approved,
// redacted operator logging; do not use the report to mutate ledger rows.
```

The current automated dry-run evidence is:

```powershell
go test ./internal/listingsubscription -run 'TestUsageLedgerReconciliation' -count=1
```

Repairs, scheduled reconciliation, exports, manual adjustments, and support
reports remain PAY-044. Before any repair is proposed, preserve the report,
identify the owning business event, and obtain a separately approved runbook.

## PAY-042 required integration contract

PAY-042 must replace the legacy `AuthorizeUsage` / ignored `RecordUsage`
pattern with `ReserveUsage`, followed by exactly one terminal transition for a
stable business idempotency key. It must wire both public routes and internal
workers/recovery paths; adding only a route guard is insufficient.

| Business outcome | Exact current entrypoints to change in PAY-042 | Ledger transition |
| --- | --- | --- |
| Studio design jobs | `internal/listingkit/api/studio_designs_handler.go`, `internal/listingkit/api/studio_async_jobs_handler_entrypoints.go`, `internal/listingkit/api/studio_async_jobs_handler_runner.go` (`/studio/designs`) | Reserve before accepted work; commit only after persisted success; release on failure/cancellation. |
| Studio product-image jobs | `internal/listingkit/api/studio_product_images_handler.go`, `internal/listingkit/api/studio_async_jobs_handler_entrypoints.go`, `internal/listingkit/api/studio_async_jobs_handler_runner.go` (`/studio/product-images`) | Same stable job identity and terminal policy as design jobs. |
| SHEIN save draft and publish | `internal/listingkit/api/submit_handler.go` (`SubmitTask`) plus the task submission execution, persistence, and recovery services under `internal/listingkit/` | Reserve per requested action; commit only after the corresponding remote success is durably persisted. |
| Storage upload/delete | `internal/listingkit/api/upload_handler.go` (`UploadListingKitImages`, `DeleteUploadedListingKitImage`) and `internal/listingkit/api/studio_sessions_handler.go` | Storage current-usage deltas use a stable asset operation ID; upload commits after storage succeeds, delete commits the negative delta only after completed deletion. |
| Batch and recovery/retry | `internal/listingkit/api/studio_batch_runs_handler.go`, `internal/listingkit/task_studio_batch_run_service.go`, and SHEIN submission recovery paths under `internal/listingkit/` | Each item keeps its original idempotency key; do not reserve or commit once per batch/run envelope. |

The following are explicitly non-commit outcomes: local execution failure,
user cancellation, platform rejection, and unknown remote state. They must
release a pre-remote reservation when safe, or remain reserved for controlled
reconciliation when remote outcome is unknown; neither path commits customer
usage. A retry/recovery must first locate the original event by tenant and
idempotency key and reuse it rather than creating another charge.

PAY-042 must also remove every `_ = RecordUsage(...)` / ignored usage-write
error in its in-scope path. It must not cut over legacy counters globally until
the entrypoint tests, reconciliation evidence, and rollout approval are
complete.

## Deferred work

- PAY-042 implements the entrypoint integration above; it is not implemented
  by PAY-041.
- PAY-043 owns subscription registry and provider/payment state.
- PAY-044 owns scheduling, export, adjustment, and approved repair workflow.
- OpenMeter production deployment, capacity, retention, and SLO design remain
  separate from this local-ledger evidence.
