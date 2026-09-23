param(
    [Parameter(Mandatory = $true)]
    [string]$ConfigPath,
    [Parameter(Mandatory = $true)]
    [string]$MoneyConfigPath
)

if (-not [System.IO.Path]::IsPathRooted($ConfigPath) -or -not [System.IO.Path]::IsPathRooted($MoneyConfigPath)) {
    throw "Both config paths must be absolute paths to private schema-owner manifests."
}
if (-not (Test-Path -LiteralPath $ConfigPath -PathType Leaf) -or -not (Test-Path -LiteralPath $MoneyConfigPath -PathType Leaf)) {
    throw "A schema-owner config path does not exist."
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Push-Location $repoRoot
try {
    & go run ./cmd/commercial-owner-schema-migrate -config $ConfigPath -money-config $MoneyConfigPath
    if ($LASTEXITCODE -ne 0) {
        throw "Commercial owner schema migration failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}
