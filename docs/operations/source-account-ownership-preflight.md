# Source-account ownership preflight (slice A)

This is a read-only preview, not a backfill or a production cutover gate. The full
inventory, target contract and A–D rollout dependencies are in
[the design](../superpowers/specs/2026-09-05-source-account-organization-cutover.md).

Run on the host with the actual 1688 account profile volume mounted at its runtime
path. Use the same absolute profile root that currently feeds
`NewAccountProfileResolver`; do not substitute the public/anonymous browser directory.
The stored ProfileRef is currently only a marker; the actual directory is
`<root>/<legacy tenant>/<account ID>`. Existing directories are verified and reused
as opaque references in the receipt. Nothing is created, moved, deleted or launched.
Missing profiles (including disabled/deleted accounts) block; resolve their retention
and runtime history explicitly rather than creating empty replacement profiles. The
preflight also compares filesystem identity across accounts, so two account paths that
resolve to the same underlying browser profile (for example through a bind mount) fail
closed even when the visible path names differ.

Provide these environment variables through the existing secret mechanism:

- `SOURCE_ACCOUNT_PREFLIGHT_DSN`: business PostgreSQL database with `source_account`.
- `SOURCE_ACCOUNT_METADATA_DSN`: the explicitly selected authoritative ZITADEL
  database with `projections.org_metadata2`; no automatic candidate database discovery.

Use credentials restricted to SELECT. The command additionally uses read-only,
repeatable-read transactions. Two separate database snapshots are **not atomic**.
Verify the correct ZITADEL instance/environment before running. Projection freshness,
current authority and multi-host profile agreement must be revalidated under the B
freeze; a successful A receipt does not certify them.

```powershell
go run ./cmd/source-account-ownership-preflight `
  -source-id production/business-cluster `
  -metadata-id production/zitadel-cluster `
  -profile-root C:/runtime/1688-account-profiles `
  -receipt C:/migration-evidence/1688-ownership-preflight-001.json
```

Source identities are non-secret audit labels, not DSNs. The maintained operational script is
`scripts/source-account-ownership-preflight.ps1`, with `-ProfileRoot`, `-SourceId`,
`-MetadataId` and `-ReceiptPath` parameters. It forwards the same environment-based
connections and read-only command, preserves the caller's receipt path and propagates
failure. Direct `go run` invocation above is equivalent. The receipt path must be
absolute, its directory must already exist, and it must be outside the browser profile
root. Symlink/junction aliases in the receipt-parent path are rejected, and the receipt
parent ancestry is compared with the verified profile root/tenant/account filesystem
identities so direct profile aliases cannot become evidence destinations. Use a new
output name for each attempt. Default total deadline is two minutes (maximum ten); each
collection is capped at 100,000 rows. Exceeding the bound fails closed rather than
truncating. A larger installation needs a separately reviewed paged snapshot
implementation, not an unbounded flag. Command errors suppress database connection
details; troubleshoot connection/schema/permissions with existing operator tools.

On success, the command prints account count and SHA-256. The JSON includes each
account's old ownership/status/deleted/profile reference, mapped Organization and exact
verified profile directory, plus metadata sequences/removal flags and source observation
times. It contains no profile contents or credentials. Treat it as internal audit data.
The digest binds the captured fields and source identities/database names; observation
times are excluded so identical captured inputs give identical digests across reruns.
It does not fingerprint uncaptured account fields or browser cookie contents, and is
not a signature or authorization to mutate data. B must take its own complete before-image.

The final receipt is published exclusively from a synced temporary file using a hard
link. An existing final file is never overwritten; unsupported filesystems fail closed.
Publication observes the command context before the final hard link and before reporting
success. Cancellation before the final link leaves no final receipt; if cancellation is
observed immediately after linking, the final link is removed before the command returns.
A process interruption earlier can leave a `.ownership-receipt-*.pending` file. Such
files are not receipts. Rerun with a new output path; no database rollback or profile
recovery is needed. Power-loss durability of the directory entry depends on the
filesystem; a missing final file is handled by rerunning the read-only command.

Validation blocks on missing, removed, duplicate or ambiguous ownership; invalid
noncanonical metadata; invalid/duplicate account identity; absent/non-directory or
symlink/junction profile path (including ancestors); or two accounts sharing one
filesystem profile identity. Duplicate Organization metadata is rejected rather than
choosing a sequence and accidentally resurrecting a removed owner. The command reads all
1688 account rows, including disabled/deleted, and does not activate them.

Before B/C: freeze all 1688 admissions and source-account/mapping writers, drain and
attest every old process queue/worker, reconcile Redis and local terminal outcomes,
export/checksum retained results and verify authoritative metadata again. Redis emptiness
or a six-hour wait is not evidence that old jobs never existed. Unknown ownership or
unknown in-flight outcome blocks migration. A does not connect to Redis or stop workers.
C also needs coordination with the old #30 handoff consumer; never add a numeric adapter
to make that reader survive. Neither #30 nor #301 is closed by this prerequisite PR.

Local validation:

```powershell
go test ./internal/integration/persistence/sourceaccount/ownershipmigration -count=1
go vet ./internal/integration/persistence/sourceaccount/ownershipmigration ./cmd/source-account-ownership-preflight
go test ./internal/sourceaccount/... ./internal/crawler/alibaba1688/... ./internal/crawler/shared/... -count=1
go test ./tests -count=1
```
