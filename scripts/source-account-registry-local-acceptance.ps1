[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$taskRepoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $taskRepoRoot
try {
	& go test -tags=integration ./cmd/source-account-registry-schema-init ./internal/integration/persistence/sourceaccountregistry ./internal/app/httpapi -run 'TestBinaryInitializesEmptyPostgresExactly|TestSourceAccountRegistryPostgres|TestSourceAccountApplicationPostgresAcceptance' -count=1
    if ($LASTEXITCODE -ne 0) {
        throw "Source Account registry local acceptance failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
