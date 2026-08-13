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

if (-not [string]::IsNullOrEmpty($env:TEST_ALL_FAIL_DIRECTORY) -and
    (Get-Location).Path -eq $env:TEST_ALL_FAIL_DIRECTORY) {
    exit 23
}
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Directory "go.ps1") -Encoding UTF8
}

function Invoke-TestAllHarness {
    param(
        [Parameter(Mandatory = $true)]
        [string]$CallsPath,
        [string]$FailDirectory = ""
    )

    $priorCallsPath = $env:TEST_ALL_CALLS_PATH
    $priorFailDirectory = $env:TEST_ALL_FAIL_DIRECTORY
    try {
        $env:TEST_ALL_CALLS_PATH = $CallsPath
        $env:TEST_ALL_FAIL_DIRECTORY = $FailDirectory
        $output = & pwsh -NoProfile -File $testAllScript -count=1 -run HarnessMarker
        $exitCode = $LASTEXITCODE
        if ($null -eq $output) {
            throw "test-all.ps1 produced no visible output"
        }
        return $exitCode
    }
    finally {
        $env:TEST_ALL_CALLS_PATH = $priorCallsPath
        $env:TEST_ALL_FAIL_DIRECTORY = $priorFailDirectory
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
            @{ Directory = $repositoryRoot; Packages = @("./cmd/...", "./internal/...", "./tests/...") },
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
}
