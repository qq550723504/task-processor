[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$binaryDirectory = Join-Path $repoRoot '.local/bin'
$binaryName = if ($IsWindows) { 'source-account-ownership-rehearsal.exe' } else { 'source-account-ownership-rehearsal' }
$binaryPath = Join-Path $binaryDirectory $binaryName

Push-Location $repoRoot
try {
    New-Item -ItemType Directory -Force -Path $binaryDirectory | Out-Null
    & go build -o $binaryPath ./cmd/source-account-ownership-rehearsal
    if ($LASTEXITCODE -ne 0) {
        throw "Source Account ownership rehearsal build failed with exit code $LASTEXITCODE"
    }

    & $binaryPath rehearsal --yes
    $rehearsalExitCode = $LASTEXITCODE
    if ($rehearsalExitCode -ne 0) {
        Write-Error "Source Account ownership rehearsal failed with exit code $rehearsalExitCode"
        exit $rehearsalExitCode
    }
}
finally {
    Pop-Location
}
