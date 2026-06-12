param()

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$hooksPath = Join-Path $repoRoot ".githooks"

if (-not (Test-Path -LiteralPath $hooksPath)) {
    throw "Hooks directory not found: $hooksPath"
}

Set-Location $repoRoot
git config core.hooksPath .githooks

Write-Host "Configured local git hooks path: .githooks" -ForegroundColor Green
Write-Host "pre-push will now run scripts/test-fast.ps1 before pushing." -ForegroundColor Green
