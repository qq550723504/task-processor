# Store Service History Hard-cut Runbook

This runbook covers only the approved Phase 1 decision that no authoritative
legacy Store paid-service history source exists. It does not authorize a future
external history resolver.

## Safety boundary

- `verify` is the default, opens only an existing database, and uses a read-only
  session. It scans rows in ID order without materializing the Store table.
- `backfill` opens only an existing database and updates one bounded batch only;
  the maximum batch size is 1000.
- The command never creates schema, adds final constraints, switches read/write
  authority, or enables lifecycle HTTP/BFF routes.
- Missing, malformed, unknown-field, or changed manifests fail closed.
- `record_status`, `service_status`, timestamps, and immutable history evidence
  are updated in one serializable row transaction. Transient PostgreSQL/SQLite
  concurrency failures and PostgreSQL server-side statement timeouts use bounded
  retries; caller cancellation remains terminal.
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

./scripts/store-service-history-migrate.ps1 `
  -ManifestPath C:\release\store-service-history-manifest.json `
  -ConfigPath config/config-prod.yaml

./scripts/store-service-history-migrate.ps1 `
  -Action backfill `
  -BatchSize 100 `
  -ManifestPath C:\release\store-service-history-manifest.json `
  -ConfigPath config/config-prod.yaml
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

Legacy `provisioning` rows are part of the migration cohort even though they do
not yet have service state. Verification remains blocked until they carry
`confirmed_absent` evidence, so a later create recovery cannot activate a row
that bypassed the history gate.

Expanded service fields on a legacy row without immutable history evidence are
not authoritative. Backfill re-derives the compatibility state from the legacy
lifecycle and clears any unexplained service period before persisting
`confirmed_absent` evidence.

Stop on any non-zero blocker. Do not infer missing history, edit evidence rows,
add final constraints, or enable lifecycle routes as a workaround.
