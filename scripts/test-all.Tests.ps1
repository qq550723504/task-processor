$testAllScript = Join-Path $PSScriptRoot "test-all.ps1"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

function New-TestAllGoFake {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Directory
    )

    New-Item -ItemType Directory -Path $Directory -Force | Out-Null
    @'
param(
    [switch]$v,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$FakeArguments
)

[pscustomobject]@{
    WorkingDirectory = (Get-Location).Path
    Arguments = @("test") + @($(if ($v) { "-v" })) + @($FakeArguments | Select-Object -Skip 1)
} | ConvertTo-Json -Compress | Add-Content -LiteralPath $env:TEST_ALL_CALLS_PATH

if (-not [string]::IsNullOrEmpty($env:TEST_ALL_REQUIRED_ROOT_PACKAGE) -and
    (Get-Location).Path -eq $env:TEST_ALL_REPOSITORY_ROOT) {
    $required = $env:TEST_ALL_REQUIRED_ROOT_PACKAGE
    $covered = $false
    foreach ($argument in @($FakeArguments | Select-Object -Skip 1)) {
        if ($argument -eq "./...") {
            $covered = $true
            break
        }
        if ($argument.EndsWith("/...")) {
            $prefix = $argument.Substring(0, $argument.Length - 3)
            if ($required.StartsWith($prefix, [System.StringComparison]::Ordinal)) {
                $covered = $true
                break
            }
        }
    }
    if (-not $covered) {
        exit 29
    }
}

if (-not [string]::IsNullOrEmpty($env:TEST_ALL_FAIL_DIRECTORY) -and
    (Get-Location).Path -eq $env:TEST_ALL_FAIL_DIRECTORY) {
    exit 23
}
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Directory "fake-go.ps1") -Encoding UTF8
    @'
@echo off
pwsh -NoProfile -File "%~dp0fake-go.ps1" %*
exit /b %ERRORLEVEL%
'@ | Set-Content -LiteralPath (Join-Path $Directory "go.cmd") -Encoding ASCII
}

function Invoke-TestAllHarness {
    param(
        [Parameter(Mandatory = $true)]
        [string]$CallsPath,
        [string]$FailDirectory = "",
        [string]$RequiredRootPackage = "",
        [switch]$NativeErrorPreference
    )

    $priorCallsPath = $env:TEST_ALL_CALLS_PATH
    $priorFailDirectory = $env:TEST_ALL_FAIL_DIRECTORY
    $priorRequiredRootPackage = $env:TEST_ALL_REQUIRED_ROOT_PACKAGE
    $priorRepositoryRoot = $env:TEST_ALL_REPOSITORY_ROOT
    try {
        $env:TEST_ALL_CALLS_PATH = $CallsPath
        $env:TEST_ALL_FAIL_DIRECTORY = $FailDirectory
        $env:TEST_ALL_REQUIRED_ROOT_PACKAGE = $RequiredRootPackage
        $env:TEST_ALL_REPOSITORY_ROOT = $repositoryRoot
        if ($NativeErrorPreference) {
            $escapedScript = $testAllScript.Replace("'", "''")
            $command = "`$global:PSNativeCommandUseErrorActionPreference = `$true; & '$escapedScript' -count=1 -run HarnessMarker; exit `$LASTEXITCODE"
            $output = & pwsh -NoProfile -Command $command
        }
        else {
            $output = & pwsh -NoProfile -File $testAllScript -count=1 -run HarnessMarker
        }
        $exitCode = $LASTEXITCODE
        if ($null -eq $output) {
            throw "test-all.ps1 produced no visible output"
        }
        return $exitCode
    }
    finally {
        $env:TEST_ALL_CALLS_PATH = $priorCallsPath
        $env:TEST_ALL_FAIL_DIRECTORY = $priorFailDirectory
        $env:TEST_ALL_REQUIRED_ROOT_PACKAGE = $priorRequiredRootPackage
        $env:TEST_ALL_REPOSITORY_ROOT = $priorRepositoryRoot
    }
}

function Read-TestAllCalls {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    return @(Get-Content -LiteralPath $Path | ForEach-Object { $_ | ConvertFrom-Json })
}

Describe "repository-wide Go test harness" {
    BeforeEach {
        $fakeDirectory = Join-Path $TestDrive "fake-bin"
        $callsPath = Join-Path $TestDrive "go-calls.jsonl"
        Remove-Item -LiteralPath $callsPath -ErrorAction SilentlyContinue
        New-TestAllGoFake -Directory $fakeDirectory
        $script:priorPath = $env:Path
        $env:Path = $fakeDirectory + [IO.Path]::PathSeparator + $env:Path
    }

    AfterEach {
        $env:Path = $script:priorPath
    }

    It "runs each Go module from its own directory and propagates Go test arguments" {
        $result = Invoke-TestAllHarness -CallsPath $callsPath

        $result | Should Be 0
        $calls = Read-TestAllCalls -Path $callsPath
        $calls.Count | Should Be 3

        $expected = @(
            @{ Directory = $repositoryRoot; Packages = @("./...") },
            @{ Directory = (Join-Path $repositoryRoot "tools"); Packages = @("./...") },
            @{ Directory = (Join-Path $repositoryRoot "hack/debug"); Packages = @("./...") }
        )
        for ($index = 0; $index -lt $expected.Count; $index++) {
            $calls[$index].WorkingDirectory | Should Be $expected[$index].Directory
            (@($calls[$index].Arguments) -contains "test") | Should Be $true
            (@($calls[$index].Arguments) -contains "-v") | Should Be $true
            (@($calls[$index].Arguments) -contains "-count=1") | Should Be $true
            (@($calls[$index].Arguments) -contains "-run") | Should Be $true
            (@($calls[$index].Arguments) -contains "HarnessMarker") | Should Be $true
            foreach ($package in $expected[$index].Packages) {
                (@($calls[$index].Arguments) -contains $package) | Should Be $true
            }
        }

        (@($calls[0].Arguments) -contains "./tools/...") | Should Be $false
        (@($calls[0].Arguments) -contains "./hack/debug/...") | Should Be $false
    }

    It "returns the nested Go module failure code and stops later module execution" {
        $toolsDirectory = Join-Path $repositoryRoot "tools"
        $result = Invoke-TestAllHarness -CallsPath $callsPath -FailDirectory $toolsDirectory

        $result | Should Be 23
        $calls = Read-TestAllCalls -Path $callsPath
        $calls.Count | Should Be 2
        $calls[0].WorkingDirectory | Should Be $repositoryRoot
        $calls[1].WorkingDirectory | Should Be $toolsDirectory
    }

    It "covers root-module packages outside cmd internal and tests" {
        $requiredPackage = "./scripts/listingkit-shein-pod-image-index-backfill"
        $result = Invoke-TestAllHarness -CallsPath $callsPath -RequiredRootPackage $requiredPackage

        $result | Should Be 0
        $calls = Read-TestAllCalls -Path $callsPath
        $calls.Count | Should Be 3
    }

    It "preserves a nested native failure code when native errors use ErrorActionPreference" {
        $toolsDirectory = Join-Path $repositoryRoot "tools"
        $result = Invoke-TestAllHarness -CallsPath $callsPath -FailDirectory $toolsDirectory -NativeErrorPreference

        $result | Should Be 23
        $calls = Read-TestAllCalls -Path $callsPath
        $calls.Count | Should Be 2
    }
}
