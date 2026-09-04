[CmdletBinding()]
param(
    [string]$ConfigPath = "config/config-dev.yaml",
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$ManifestPath,
    [ValidateSet("verify", "backfill")][string]$Action = "verify",
    [ValidateRange(1, 1000)][int]$BatchSize = 100
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$arguments = @(
    "run", "./cmd/store-service-history-migrate",
    "--config", $ConfigPath,
    "--manifest", $ManifestPath,
    "--action", $Action,
    "--batch-size", $BatchSize
)

Push-Location $repoRoot
try {
    & go @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Store service history migration failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
