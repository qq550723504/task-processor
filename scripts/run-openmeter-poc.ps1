[CmdletBinding()]
param(
    [string]$RunId = (Get-Date -Format 'yyyyMMdd-HHmmss'),
    [string]$ApiKey,
    [switch]$KeepEnvironment
)

$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot "lib/openmeter-poc.ps1")

$exitCode = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId $RunId -ApiKey $ApiKey -KeepEnvironment:$KeepEnvironment
if ($exitCode -ne 0) {
    exit $exitCode
}
