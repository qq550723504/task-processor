[CmdletBinding()]
param(
    [string]$ConfigPath = "config/config-dev.yaml",
    [string]$OutputPath = ".local/tmp/listingkit-owner-scope-dry-run.json",
    [string]$SqlOutputPath = ".local/tmp/listingkit-owner-scope-dry-run.sql",
    [string]$SchemaOutputPath = ".local/tmp/listingkit-owner-scope-schema.sql",
    [string]$BackfillOutputPath = ".local/tmp/listingkit-owner-scope-backfill.sql",
    [string]$SafeBackfillOutputPath = ".local/tmp/listingkit-owner-scope-safe-backfill.sql",
    [string]$ManualReviewOutputPath = ".local/tmp/listingkit-owner-scope-manual-review.sql",
    [string]$UnresolvedTasksJsonPath = ".local/tmp/listingkit-owner-scope-unresolved-tasks.json",
    [string]$UnresolvedTasksCsvPath = ".local/tmp/listingkit-owner-scope-unresolved-tasks.csv",
    [string]$UnresolvedStudioJsonPath = ".local/tmp/listingkit-owner-scope-unresolved-studio-sessions.json",
    [string]$UnresolvedStudioCsvPath = ".local/tmp/listingkit-owner-scope-unresolved-studio-sessions.csv",
    [string]$UnresolvedSummaryJsonPath = ".local/tmp/listingkit-owner-scope-unresolved-summary.json"
)

$ErrorActionPreference = "Stop"

go run ./cmd/listingkit-owner-scope-dry-run --config $ConfigPath --output $OutputPath --sql-output $SqlOutputPath --schema-output $SchemaOutputPath --backfill-output $BackfillOutputPath --safe-backfill-output $SafeBackfillOutputPath --manual-review-output $ManualReviewOutputPath --unresolved-tasks-json $UnresolvedTasksJsonPath --unresolved-tasks-csv $UnresolvedTasksCsvPath --unresolved-studio-json $UnresolvedStudioJsonPath --unresolved-studio-csv $UnresolvedStudioCsvPath --unresolved-summary-json $UnresolvedSummaryJsonPath
