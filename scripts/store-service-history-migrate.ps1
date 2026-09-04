[CmdletBinding()]
param(
    [string]$ConfigPath = "config/config-dev.yaml",
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$ManifestPath,
    [ValidateSet("verify", "backfill", "constraints")][string]$Action = "verify",
    [ValidateRange(1, 1000)][int]$BatchSize = 100,
    [ValidateNotNullOrEmpty()][string]$ConstraintLockTimeout = "500ms",
    [ValidateNotNullOrEmpty()][string]$ConstraintStatementTimeout = "30s"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$arguments = @(
    "run", "./cmd/store-service-history-migrate",
    "--config", $ConfigPath,
    "--manifest", $ManifestPath,
    "--action", $Action,
    "--batch-size", $BatchSize,
    "--constraint-lock-timeout", $ConstraintLockTimeout,
    "--constraint-statement-timeout", $ConstraintStatementTimeout
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
