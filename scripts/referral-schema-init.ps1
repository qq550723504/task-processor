[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$DsnFile,
    [ValidateRange(1, 60)][int]$TimeoutSeconds = 30
)

$ErrorActionPreference = 'Stop'
# Resolve against the caller's directory before changing to the repository.
$resolvedDsnFile = (Resolve-Path -LiteralPath $DsnFile -ErrorAction Stop).ProviderPath
if (-not (Test-Path -LiteralPath $resolvedDsnFile -PathType Leaf)) {
    throw 'An existing DSN file is required'
}
$repoRoot = Split-Path -Parent $PSScriptRoot
$arguments = @('run', './cmd/referral-schema-init', '-dsn-file', $resolvedDsnFile, '-timeout', "${TimeoutSeconds}s")
Push-Location $repoRoot
try {
    & go @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Referral schema initialization failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
