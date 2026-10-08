param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[a-z][a-z0-9_]{0,62}$')]
    [string]$RuntimeRole
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($env:NOTIFICATION_CENTER_SCHEMA_DSN)) {
    throw 'NOTIFICATION_CENTER_SCHEMA_DSN is required'
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Push-Location -LiteralPath $repoRoot
try {
    go run ./cmd/notification-center-schema-init --runtime-role $RuntimeRole
    if ($LASTEXITCODE -ne 0) {
        throw "Notification Center schema initialization failed (exit $LASTEXITCODE)"
    }
}
finally {
    Pop-Location
}
