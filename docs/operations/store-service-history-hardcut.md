# Store Service History Hard-cut Runbook

This runbook covers only the approved Phase 1 decision that no authoritative
legacy Store paid-service history source exists. It does not authorize a future
external history resolver.

## Safety boundary

- `verify` is the default and is read-only.
- `backfill` updates one bounded batch only; the maximum batch size is 1000.
- The command never creates schema, adds final constraints, switches read/write
  authority, or enables lifecycle HTTP/BFF routes.
- Missing, malformed, unknown-field, or changed manifests fail closed.
- `record_status`, `service_status`, timestamps, and immutable history evidence
  are updated in one serializable row transaction. Transient PostgreSQL/SQLite
  concurrency failures use bounded retries.
- A `ready_for_constraints=true` report is evidence for Phase D only. It is not
  approval for Phase E constraints or Phase F enablement.

## Approved manifest

Create a release-controlled JSON file. `decision_reference`, `approved_by`, and
`approved_at` must point to the durable approval record used by the release.

```json
{
  "schema_version": "store-service-history/no-authoritative-source/v1",
  "decision_reference": "replace-with-durable-product-decision-reference",
  "approved_by": "replace-with-approver-subject",
  "approved_at": "REPLACE_WITH_RFC3339_APPROVAL_TIME"
}
```

The canonical manifest hash becomes the immutable source snapshot token. A
different approver, timestamp, or decision reference produces a different token
and causes verification to report `history_snapshot_conflict_count` instead of
silently adopting the new decision.

## Execution order

1. Run the existing Workbench schema migration first so all transitional state
   and history-evidence columns exist.
2. Deploy the compatibility-writer binary and drain older pods.
3. Run read-only verification. It is expected to fail with unresolved rows
   before the first backfill.
4. Run one explicit backfill batch at a time until `scanned_count` and
   `updated_count` are both zero.
5. Run read-only verification again and archive its JSON output.

```powershell
go run ./cmd/listingkit-schema-migrate --scope workbench --config config/config-prod.yaml

go run ./cmd/store-service-history-migrate `
  --action verify `
  --manifest C:\release\store-service-history-manifest.json `
  --config config/config-prod.yaml

go run ./cmd/store-service-history-migrate `
  --action backfill `
  --batch-size 100 `
  --manifest C:\release\store-service-history-manifest.json `
  --config config/config-prod.yaml
```

## Required Phase D result

Before a separate Phase E/F change is considered, the final report must have:

```text
ready_for_constraints=true
unresolved_count=0
invalid_state_count=0
history_unavailable_count=0
history_error_count=0
history_snapshot_conflict_count=0
history_handoff_backlog_count=0
```

Rows in the migration cohort carry `confirmed_absent` evidence from the
approved manifest. Stores created by the compatibility writer carry
`not_applicable_new` evidence bound to their create fingerprint. Neither status
grants service time or resource balance.

Stop on any non-zero blocker. Do not infer missing history, edit evidence rows,
add final constraints, or enable lifecycle routes as a workaround.
