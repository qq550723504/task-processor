param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$testModules = @(
    [pscustomobject]@{
        Name = "root"
        Directory = $repoRoot
        Packages = @("./...")
    },
    [pscustomobject]@{
        Name = "tools"
        Directory = Join-Path $repoRoot "tools"
        Packages = @("./...")
    },
    [pscustomobject]@{
        Name = "debug tools"
        Directory = Join-Path $repoRoot "hack/debug"
        Packages = @("./...")
    }
)

foreach ($module in $testModules) {
    Write-Host "Running Go test suite for $($module.Name) module..." -ForegroundColor Cyan
    Push-Location $module.Directory
    $exitCode = 0
    try {
        $packages = $module.Packages
        & go test -v @GoTestArgs @packages
        $exitCode = $LASTEXITCODE
    }
    catch [System.Management.Automation.NativeCommandExitException] {
        $exitCode = $_.Exception.ExitCode
    }
    finally {
        Pop-Location
    }
    if ($exitCode -ne 0) {
        exit $exitCode
    }
}

exit 0
