param(
    [Parameter(Mandatory = $true)]
    [string]$DsnFile,
    [Parameter(Mandatory = $true)]
    [ValidateSet('reports')]
    [string]$ConfirmDatabase,
    [switch]$InstallEmptySchema
)

$ErrorActionPreference = 'Stop'
if (-not $InstallEmptySchema -or -not [IO.Path]::IsPathRooted($DsnFile)) {
    throw 'Require an absolute private owner DSN file and -InstallEmptySchema'
}
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
Push-Location -LiteralPath $repoRoot
try {
    go run ./cmd/report-center-schema-init -dsn-file $DsnFile -confirm-database $ConfirmDatabase -install-empty-schema
    if ($LASTEXITCODE -ne 0) {
        throw "Report schema initialization refused (exit $LASTEXITCODE)"
    }
}
finally {
    Pop-Location
}
