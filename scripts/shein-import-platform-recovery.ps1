[CmdletBinding()]
param(
    [string]$ConfigPath = "config/config-dev.yaml",
    [Parameter(Mandatory = $true)][ValidateRange(1, [int]::MaxValue)][int]$ExpectedCount,
    [switch]$Execute,
    [string]$ConfirmFingerprint = ""
)

$ErrorActionPreference = "Stop"

$args = @(
    "run", "./cmd/shein-import-platform-recovery",
    "--config", $ConfigPath,
    "--store-id", "986",
    "--expected-count", $ExpectedCount
)
if ($Execute) {
    if ([string]::IsNullOrWhiteSpace($ConfirmFingerprint)) {
        throw "-ConfirmFingerprint is required with -Execute"
    }
    $args += @("--execute", "--confirm-fingerprint", $ConfirmFingerprint)
}

& go @args
if ($LASTEXITCODE -ne 0) {
    throw "SHEIN import platform recovery failed with exit code $LASTEXITCODE"
}
