[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Config,
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$ConfirmEmptyDatabase
)

$ErrorActionPreference = 'Stop'
# Explicit operational owner only. The CLI validates the private manifest,
# existing empty database and pre-provisioned role; no default credentials.
Push-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
try {
    & go run ./cmd/product-acquisition-init -config $Config -confirm-empty-database $ConfirmEmptyDatabase
    $result = $LASTEXITCODE
} finally {
    Pop-Location
}
exit $result
