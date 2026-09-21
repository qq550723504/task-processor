[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Queue,
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Url,
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Actor,
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Organization,
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Browser,
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Extension,
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Profile,
    [switch]$Headless
)

$ErrorActionPreference = 'Stop'
# Operational owner for cmd/1688-batch-import. The approved actor and organization
# are required inputs and are never read from the browser session; the command
# writes a local queue file and drives exactly one item. Exit code 3 means the
# outcome is unknown, which requires a human check instead of a retry.
Push-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
try {
    $arguments = @(
        'run', './cmd/1688-batch-import',
        '-queue', $Queue,
        '-url', $Url,
        '-actor', $Actor,
        '-organization', $Organization,
        '-browser', $Browser,
        '-extension', $Extension,
        '-profile', $Profile
    )
    if ($Headless) { $arguments += '-headless' }
    & go @arguments
    $result = $LASTEXITCODE
} finally {
    Pop-Location
}
exit $result
