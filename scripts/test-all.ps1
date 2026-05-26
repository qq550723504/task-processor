param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

$allTestPackages = @(
    "./cmd/..."
    "./internal/..."
    "./tests/..."
    "./tools/..."
    "./hack/debug/..."
)

Write-Host "Running full Go test suite..." -ForegroundColor Cyan
& go test -v @GoTestArgs @allTestPackages
exit $LASTEXITCODE
