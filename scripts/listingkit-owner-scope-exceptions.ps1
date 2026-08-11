[CmdletBinding()]
param(
    [string]$ConfigPath = "config/config-prod.yaml",
    [Parameter(Mandatory = $true)][string]$ReportPath,
    [Parameter(Mandatory = $true)][string]$ConfirmReport
)

$ErrorActionPreference = "Stop"
& go run ./cmd/listingkit-owner-scope-exceptions --config $ConfigPath --report $ReportPath --confirm-report $ConfirmReport
if ($LASTEXITCODE -ne 0) {
    throw "owner exception seeder failed with exit code $LASTEXITCODE"
}
