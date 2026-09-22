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
    [string]$Profile
)

$ErrorActionPreference = 'Stop'
# Operational owner for cmd/1688-batch-import. The actor and organization given here
# are the scope this run EXPECTS, not consent: the batch is attributed only to the
# identity the application reports as verified, after a person confirms it at the
# terminal. The command prompts on stdin twice when needed (confirm the scope; redo
# the item after clearing a captcha or login wall), so run it from an interactive
# console and leave the browser window open until it exits. There is deliberately no
# headless switch: the person who clears the captcha and confirms the scope needs a
# visible window, and the command refuses --headless for that reason.
#
# Exit code 3 means stop and verify: either the outcome is unknown, or an earlier item
# in the queue may already have been published. Both require a human check instead of
# a retry. The wrapper preserves that code by building the binary and running it
# directly: `go`'s run subcommand reports any non-zero exit as 1, which would erase
# the very signal this command exists to carry.
Push-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
try {
    $arguments = @(
        '-queue', $Queue,
        '-url', $Url,
        '-actor', $Actor,
        '-organization', $Organization,
        '-browser', $Browser,
        '-extension', $Extension,
        '-profile', $Profile
    )
    $binary = Join-Path ([System.IO.Path]::GetTempPath()) ("1688-batch-import-{0}.exe" -f [guid]::NewGuid().ToString('N'))
    try {
        & go build -o $binary ./cmd/1688-batch-import
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
        & $binary @arguments
        $result = $LASTEXITCODE
    } finally {
        Remove-Item -LiteralPath $binary -Force -ErrorAction SilentlyContinue
    }
} finally {
    Pop-Location
}
exit $result
