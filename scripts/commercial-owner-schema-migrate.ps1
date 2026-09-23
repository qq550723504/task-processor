param(
    [Parameter(Mandatory = $true)]
    [string]$ConfigPath
)

if (-not [System.IO.Path]::IsPathRooted($ConfigPath)) {
    throw "ConfigPath must be an absolute path to the private current-application manifest."
}
if (-not (Test-Path -LiteralPath $ConfigPath -PathType Leaf)) {
    throw "ConfigPath does not exist."
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Push-Location $repoRoot
try {
    & go run ./cmd/commercial-owner-schema-migrate -config $ConfigPath
    if ($LASTEXITCODE -ne 0) {
        throw "Commercial owner schema migration failed with exit code $LASTEXITCODE."
    }
}
finally {
    Pop-Location
}
