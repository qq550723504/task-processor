# ListingKit owner reconciliation verification

Date: 2026-08-10
Branch: `codex/listingkit-owner-reconciliation`

## Scope

This change restores a safe, one-time ListingKit owner reconciliation command.
The default mode is read-only. The optional write mode requires the exact
fingerprint from a freshly repeated report and updates only blank owner fields
for uniquely verified legacy-to-ZITADEL mappings.

## Evidence

- `go test ./internal/listingkit/ownerreconcile ./internal/app/runtime/listingkitownerreconcile ./cmd/listingkit-owner-scope-dry-run -count=1`
- `go vet ./internal/listingkit/ownerreconcile ./internal/app/runtime/listingkitownerreconcile`
- PowerShell parser validation for `scripts/listingkit-owner-scope-dry-run.ps1`
- `git diff --check`

The repository tests use SQL mock connections to prove SELECT-only dry-runs,
fixed inventory identifiers, redacted findings, exact confirmation checks,
transaction rollback, and parameterized updates. No production database or
provider token was used by this implementation pass.

## Operator boundary

Run the PowerShell wrapper without `-Execute` to produce the report. Review
unresolved and conflicting groups before considering a backfill. If and only if
the report is approved, pass its exact 12-hex fingerprint with `-Execute
-ConfirmReport`; a changed report fails closed before the first UPDATE.

The report and CSV artifacts contain only table names, aggregate counts, and
short SHA-256 fingerprints. Do not paste raw database identifiers, subjects,
tokens, email addresses, or SQL bodies into tickets or release logs.

## Known limitation

The full repository test suite is not a required gate for this isolated tool;
unrelated crawler metadata fixtures may fail when the shared metadata table is
absent. The focused packages above are the verification scope for this change.
