param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('ai_projects_runtime')]
    [string]$RuntimeRole
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($env:PROJECT_CENTER_SCHEMA_DSN)) {
    throw 'PROJECT_CENTER_SCHEMA_DSN is required'
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Push-Location -LiteralPath $repoRoot
try {
    go run ./cmd/project-center-schema-init --runtime-role $RuntimeRole
    if ($LASTEXITCODE -ne 0) {
        throw "Project Center schema initialization failed (exit $LASTEXITCODE)"
    }
}
finally {
    Pop-Location
}
