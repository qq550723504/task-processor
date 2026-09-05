[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$ProfileRoot,
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$SourceId,
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$MetadataId,
    [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$ReceiptPath,
    [ValidateRange(1, 600)][int]$TimeoutSeconds = 120
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
# Resolve receipt relative to the operator's working directory before changing it.
$receiptAbsolute = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($ReceiptPath)
$arguments = @(
    'run', './cmd/source-account-ownership-preflight',
    '-profile-root', $ProfileRoot,
    '-source-id', $SourceId,
    '-metadata-id', $MetadataId,
    '-receipt', $receiptAbsolute,
    '-timeout', "${TimeoutSeconds}s"
)

Push-Location $repoRoot
try {
    & go @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Source account ownership preflight failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
