param(
    [Parameter(Mandatory = $true)][string]$Config,
    [Parameter(Mandatory = $true)][string]$InputPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Push-Location (Join-Path $PSScriptRoot '..')
try {
    & go run ./cmd/product-agent-credential-provision -config $Config -input $InputPath
    if ($LASTEXITCODE -ne 0) {
        throw "product-agent-credential-provision failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}
