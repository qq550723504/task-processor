# ListingKit Current Orphaned Owner Exceptions Design

## Goal

Allow only the 312 owner-reconciliation groups from report `648cdfab03c4` (874,891 rows) to be classified as system-owned, while preserving `creator`, `created_by`, and all future fail-closed behavior.

## Context and constraints

- The current release gate reports `unmapped_candidate` only for the report `648cdfab03c4`.
- The affected rows are existing legacy records whose candidate owner is inactive/deleted or absent from `system_users`.
- No business rows, creator fields, created_by fields, or owner_user_id values may be rewritten.
- New unmapped candidates created after this exception set must continue to block release.
- Exception matching must never log raw tenant IDs, user IDs, candidate values, or ZITADEL tokens.
- The exception set must be auditable and removable without changing business data.

## Options considered

1. **Database exception registry (selected).** Store an explicit row per table/tenant/candidate fingerprint with the source report fingerprint and a reason. Reconciliation loads the registry read-only and classifies only exact matches as system-owned. This is narrow, auditable, and revocable.
2. Checked-in configuration containing 312 fingerprints. This avoids a database table but couples deployment artifacts to one data snapshot and is harder to revoke independently.
3. Treat every unmapped candidate as system-owned. This would remove the release safety boundary for future data and is rejected.

## Architecture

Add a small `listingkit_owner_scope_system_owned_exceptions` table owned by the ListingKit schema migration. It contains:

- `table_name` and `tenant_fingerprint` identifying the persisted group;
- `candidate_fingerprint` identifying the ordered candidate source/value tuple;
- `report_fingerprint`, `reason`, `created_at`, and an active flag for audit/revocation.

The owner-reconciliation repository receives an `ExceptionStore` dependency. During dry-run it computes the existing fingerprints and checks the store before emitting an `unmapped_candidate` finding. A matching active exception becomes a `system_owned` finding and is never eligible for backfill. The preflight runtime opens the same database read-only; the exception table is required by the migrated schema, while missing-table errors fail closed rather than silently allowing data.

The current 312 rows are inserted by an explicit one-shot operator command that accepts the exact report fingerprint and validates the report before inserting. The command is idempotent and refuses a different report fingerprint or changed row set. It emits only counts and fingerprints.

## Data flow

1. Read-only owner reconciliation scans fixed inventory and computes candidate groups.
2. It loads active exception rows and matches `(table, tenant fingerprint, candidate fingerprint)`.
3. Exact matches become system-owned; all other unmapped candidates remain blocking.
4. The operator command inserts the approved 312 rows in one transaction after count/fingerprint validation.
5. The release preflight reruns the scan and blocks if any non-exempt unmapped candidate remains.

## Error handling and safety

- Invalid identifiers, duplicate exception keys, empty fingerprints, mismatched report fingerprints, and changed current report contents fail closed.
- Exception-table query errors fail closed; only a known undefined-table error is accepted during a pre-migration dry-run, never during release preflight.
- The insertion command performs no business-table updates and no ZITADEL calls.
- The exception registry is auditable by report fingerprint and reason and can be deactivated without touching source rows.

## Testing

- Unit tests prove exact-match classification, non-match blocking, duplicate rejection, and report fingerprint mismatch rejection.
- SQL-mock tests prove the repository reads the registry, never writes business tables, and fails closed on registry errors.
- Runtime tests prove the preflight dependency wiring uses the exception store and preserves the non-validating loader.
- Command tests prove the 312-row seed is idempotent and refuses changed reports.
- Focused Go tests, `go vet`, and the existing release-driver tests must pass before deployment.
