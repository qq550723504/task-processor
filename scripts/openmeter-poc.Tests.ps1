$libraryPath = Join-Path $PSScriptRoot "lib/openmeter-poc.ps1"
if (Test-Path -LiteralPath $libraryPath) {
    . $libraryPath
}

function New-TestOpenMeterPoCFakes {
    param(
        [System.Collections.ArrayList]$Calls,
        [string]$FailureMode = "",
        [string]$Secret = "",
        [hashtable]$SensitiveValues = @{}
    )

    $renderedCompose = @'
{
  "services": {
    "openmeter": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "sink-worker": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "balance-worker": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "notification-service": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "billing-worker": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "openmeter-jobs": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "postgres": { "image": "postgres:17" }
  }
}
'@

    if ($SensitiveValues.Count -gt 0) {
        $renderedModel = $renderedCompose | ConvertFrom-Json
        $renderedModel.services.postgres | Add-Member -NotePropertyName environment -NotePropertyValue ([ordered]@{
            POSTGRES_PASSWORD = $SensitiveValues.DatabasePassword
            DATABASE_URL = $SensitiveValues.DatabaseURL
        })
        $renderedModel.services.openmeter | Add-Member -NotePropertyName environment -NotePropertyValue ([ordered]@{
            JWT_SECRET = $SensitiveValues.JWTSecret
            OPENMETER_API_KEY = $Secret
        })
        $renderedModel.services.openmeter | Add-Member -NotePropertyName command -NotePropertyValue @(
            "--callback=$($SensitiveValues.UserInfoURL)"
        )
        $renderedCompose = $renderedModel | ConvertTo-Json -Depth 8
    }
    if ($FailureMode -eq "raw-compose-latest") {
        $renderedCompose = $renderedCompose.Replace(
            '"openmeter": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" }',
            '"openmeter": { "image": "ghcr.io/openmeterio/openmeter:latest" }'
        )
    }

    $state = [pscustomobject]@{ ComposeDown = $false }

    $commandInvoker = {
        param(
            [string]$FilePath,
            [string[]]$ArgumentList,
            [string]$WorkingDirectory
        )

        $call = [pscustomobject]@{
            FilePath = $FilePath
            Arguments = @($ArgumentList)
            WorkingDirectory = $WorkingDirectory
            Enabled = [Environment]::GetEnvironmentVariable("OPENMETER_POC", "Process")
            Phase = [Environment]::GetEnvironmentVariable("OPENMETER_POC_PHASE", "Process")
        }
        [void]$Calls.Add($call)

        if ($FilePath -eq "docker" -and (($ArgumentList -contains "volume") -or ($ArgumentList -contains "-v"))) {
            return [pscustomobject]@{ ExitCode = 91; Output = "forbidden destructive Docker operation" }
        }

        if ($FilePath -eq "git" -and $ArgumentList[0] -eq "clone") {
            $checkoutPath = $ArgumentList[$ArgumentList.Count - 1]
            $quickstartPath = Join-Path $checkoutPath "quickstart"
            New-Item -ItemType Directory -Path $quickstartPath -Force | Out-Null
            New-Item -ItemType Directory -Path (Join-Path $checkoutPath ".git") -Force | Out-Null
            Set-Content -LiteralPath (Join-Path $quickstartPath "docker-compose.yaml") -Value "services: {}" -Encoding UTF8
            Set-Content -LiteralPath (Join-Path $checkoutPath "sentinel.txt") -Value "preserve me" -Encoding UTF8
            if ($FailureMode -eq "clone") {
                return [pscustomobject]@{ ExitCode = 17; Output = "clone failed after writing a valid controlled checkout" }
            }
            return [pscustomobject]@{ ExitCode = 0; Output = "cloned" }
        }

        if ($FilePath -eq "git" -and $ArgumentList -contains "remote") {
            return [pscustomobject]@{ ExitCode = 0; Output = "https://github.com/openmeterio/openmeter.git" }
        }
        if ($FilePath -eq "git" -and $ArgumentList -contains "describe") {
            return [pscustomobject]@{ ExitCode = 0; Output = "v1.0.0-beta.232" }
        }
        if ($FilePath -eq "git" -and $ArgumentList -contains "rev-parse") {
            return [pscustomobject]@{ ExitCode = 0; Output = "0123456789abcdef0123456789abcdef01234567" }
        }
        if ($FilePath -eq "git" -and $ArgumentList -contains "diff") {
            if ($FailureMode -eq "dirty-quickstart") {
                return [pscustomobject]@{ ExitCode = 1; Output = "quickstart/docker-compose.yaml differs from HEAD" }
            }
            return [pscustomobject]@{ ExitCode = 0; Output = "" }
        }

        if ($FilePath -eq "docker" -and $ArgumentList -contains "config") {
            return [pscustomobject]@{ ExitCode = 0; Output = $renderedCompose }
        }
        if ($FilePath -eq "docker" -and $ArgumentList -contains "down") {
            $state.ComposeDown = $true
            return [pscustomobject]@{ ExitCode = 0; Output = "removed" }
        }
        if ($FilePath -eq "docker" -and $ArgumentList -contains "up" -and $SensitiveValues.Count -gt 0) {
            return [pscustomobject]@{
                ExitCode = 0
                Output = "JWT_SECRET=$($SensitiveValues.JWTSecret)`nDATABASE_URL=$($SensitiveValues.DatabaseURL)`ncallback=$($SensitiveValues.UserInfoURL)"
            }
        }
        if ($FilePath -eq "docker" -and $ArgumentList -contains "inspect") {
            if ($FailureMode -eq "digest") {
                return [pscustomobject]@{ ExitCode = 0; Output = "[]" }
            }
            $imageIndex = [array]::IndexOf($ArgumentList, "inspect") + 1
            $imageRepository = ([string]$ArgumentList[$imageIndex] -replace ':[^/:]+$', '')
            return [pscustomobject]@{ ExitCode = 0; Output = "[`"$imageRepository@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`"]" }
        }
        if ($FilePath -eq "docker" -and $ArgumentList -contains "stats") {
            if ($FailureMode -eq "resource") {
                return [pscustomobject]@{ ExitCode = 29; Output = "stats failed" }
            }
            return [pscustomobject]@{ ExitCode = 0; Output = '{"Name":"openmeter","CPUPerc":"0.10%","MemUsage":"10MiB / 1GiB"}' }
        }
        if ($FilePath -eq "docker" -and $ArgumentList -contains "ps" -and $ArgumentList -contains "-q") {
            if ($state.ComposeDown) {
                if ($FailureMode -eq "cleanup-containers") {
                    return [pscustomobject]@{ ExitCode = 0; Output = "container-still-running" }
                }
                return [pscustomobject]@{ ExitCode = 0; Output = "" }
            }
            return [pscustomobject]@{ ExitCode = 0; Output = "container-a`ncontainer-b" }
        }
        if ($FilePath -eq "docker" -and $ArgumentList -contains "ps") {
            return [pscustomobject]@{ ExitCode = 0; Output = '{"Service":"openmeter","State":"running"}' }
        }

        if ($FilePath -eq "go") {
            if ($FailureMode -eq "go" -and $call.Phase -eq "contract") {
                return [pscustomobject]@{ ExitCode = 23; Output = "contract phase failed" }
            }

            $proof = switch ($call.Phase) {
                "contract" { @{ Package = "task-processor/internal/integration/openmeter"; Test = "TestPoCCountMetersAggregateCommittedSuccesses" } }
                "seed" { @{ Package = "task-processor/internal/integration/openmeter"; Test = "TestPoCReplaySeed" } }
                "unavailable" { @{ Package = "task-processor/internal/integration/openmeter"; Test = "TestPoCUnavailableClassifiesFailureAsRetryable" } }
                "replay" { @{ Package = "task-processor/internal/integration/openmeter"; Test = "TestPoCReplayAfterRecoveryConvergesExactly" } }
                default { @{ Package = "task-processor/tests"; Test = "TestOpenMeterImportsStayInsideIsolatedAdapter" } }
            }
            $proofTest = $proof.Test
            $proofAction = "pass"
            if ($FailureMode -eq "go-skip-seed" -and $call.Phase -eq "seed") {
                $proofAction = "skip"
            }
            if ($FailureMode -eq "go-missing-seed" -and $call.Phase -eq "seed") {
                $proofTest = "TestPoCUnexpectedTarget"
            }
            $outputEvents = @(
                ([ordered]@{
                    Time = "2026-08-13T00:00:00Z"
                    Action = "output"
                    Package = $proof.Package
                    Test = $proofTest
                    Output = "api-key=$Secret`n"
                } | ConvertTo-Json -Compress),
                ([ordered]@{
                    Time = "2026-08-13T00:00:00Z"
                    Action = $proofAction
                    Package = $proof.Package
                    Test = $proofTest
                    Elapsed = 0.01
                } | ConvertTo-Json -Compress),
                ([ordered]@{
                    Time = "2026-08-13T00:00:00Z"
                    Action = "pass"
                    Package = $proof.Package
                    Elapsed = 0.02
                } | ConvertTo-Json -Compress)
            )
            return [pscustomobject]@{ ExitCode = 0; Output = $outputEvents -join "`n" }
        }

        return [pscustomobject]@{ ExitCode = 0; Output = "ok" }
    }.GetNewClosure()

    $healthProbe = {
        param([string]$Uri)
        [void]$Calls.Add([pscustomobject]@{
            FilePath = "health"
            Arguments = @($Uri)
            WorkingDirectory = ""
            Phase = [Environment]::GetEnvironmentVariable("OPENMETER_POC_PHASE", "Process")
        })
        if ($FailureMode -eq "health") {
            return $false
        }
        return $Uri -eq "http://127.0.0.1:48888/api/v1/debug/metrics"
    }.GetNewClosure()

    [pscustomobject]@{
        CommandInvoker = $commandInvoker
        HealthProbe = $healthProbe
    }
}

Describe "OpenMeter PoC path and Compose boundaries" {
    It "rejects every nonexact SDK endpoint before invoking dependencies" {
        $originalURL = $script:OpenMeterPoCURL
        $caseNumber = 0
        try {
            foreach ($uri in @(
            "http://openmeter.example:48888/api/v3",
            "http://192.0.2.10:48888/api/v3",
            "http://localhost:48888/api/v3",
            "http://[::1]:48888/api/v3",
            "https://127.0.0.1:48888/api/v3",
            "http://127.0.0.1:48889/api/v3",
            "http://127.0.0.1:48888/api/v2",
            "http://127.0.0.1:48888/api/v3/",
            "http://user:password@127.0.0.1:48888/api/v3",
            "http://127.0.0.1:48888/api/v3?target=remote",
            "http://127.0.0.1:48888/api/v3#remote"
            )) {
                $caseNumber++
                $calls = New-Object System.Collections.ArrayList
                $fakes = New-TestOpenMeterPoCFakes -Calls $calls
                $repositoryRoot = Join-Path $TestDrive "runner-unsafe-url-$caseNumber"
                New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
                $script:OpenMeterPoCURL = $uri

                $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-url-$caseNumber" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe

                $result | Should Not Be 0
                $calls.Count | Should Be 0
            }
        }
        finally {
            $script:OpenMeterPoCURL = $originalURL
        }
    }

    It "resolves checkout and evidence below the repository-local PoC root" {
        $paths = Get-OpenMeterPoCPaths -RepositoryRoot $TestDrive -RunId "run-42"
        $expectedRoot = [System.IO.Path]::GetFullPath((Join-Path $TestDrive ".local/openmeter-poc"))
        $prefix = $expectedRoot.TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar

        $paths.LocalRoot | Should Be $expectedRoot
        $paths.CheckoutPath.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase) | Should Be $true
        $paths.EvidencePath.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase) | Should Be $true
        $paths.OverridePath.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase) | Should Be $true
        $thrown = $false
        try { Get-OpenMeterPoCPaths -RepositoryRoot $TestDrive -RunId "../escape" } catch { $thrown = $true }
        $thrown | Should Be $true
    }

    It "rejects noncanonical RunIds" {
        foreach ($runId in @("run--42", "run-", "Run-42", "RUN-42")) {
            $thrown = $false
            try { Get-OpenMeterPoCPaths -RepositoryRoot $TestDrive -RunId $runId } catch { $thrown = $true }
            $thrown | Should Be $true
        }
    }

    It "writes an override that pins every OpenMeter-owned service" {
        $paths = Get-OpenMeterPoCPaths -RepositoryRoot $TestDrive -RunId "run-42"
        New-Item -ItemType Directory -Path $paths.LocalRoot -Force | Out-Null

        New-OpenMeterPoCComposeOverride -Path $paths.OverridePath -AllowedRoot $paths.LocalRoot
        $override = Get-Content -LiteralPath $paths.OverridePath -Raw
        $image = [regex]::Escape("ghcr.io/openmeterio/openmeter:v1.0.0-beta.232")
        foreach ($service in @("openmeter", "sink-worker", "balance-worker", "notification-service", "billing-worker", "openmeter-jobs")) {
            $servicePattern = "(?ms)^  $([regex]::Escape($service)):\r?\n    image: $image\s*(?=^  |\z)"
            $override | Should Match $servicePattern
        }
    }

    It "rejects a rendered Compose model with an OpenMeter-owned latest image" {
        $renderedJson = @'
{
  "services": {
    "openmeter": { "image": "ghcr.io/openmeterio/openmeter:latest" },
    "sink-worker": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "balance-worker": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "notification-service": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "billing-worker": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" },
    "openmeter-jobs": { "image": "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232" }
  }
}
'@

        $thrown = $false
        try { Assert-OpenMeterPoCRenderedCompose -Json $renderedJson } catch { $thrown = $true }
        $thrown | Should Be $true
    }
}

Describe "OpenMeter PoC runner behavior" {
    It "uses the built-in health check when no custom probe is supplied" {
        $repositoryRoot = Join-Path $TestDrive "runner-default-health"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        $paths = Get-OpenMeterPoCPaths -RepositoryRoot $repositoryRoot -RunId "run-default-health"
        New-Item -ItemType Directory -Path $paths.EvidencePath -Force | Out-Null
        Mock Invoke-WebRequest { [pscustomobject]@{ StatusCode = 204 } }

        Assert-OpenMeterPoCHealth -Paths $paths

        Get-Content -LiteralPath $paths.RunnerLogPath -Raw | Should Match "health verified: http://127.0.0.1:48888/api/v1/debug/metrics"
    }

    It "uses the exact Compose project and cleans up without deleting volumes or the checkout" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls
        $repositoryRoot = Join-Path $TestDrive "runner-cleanup"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        $localRoot = Join-Path $repositoryRoot ".local/openmeter-poc"
        $evidenceRoot = Join-Path $localRoot "evidence"
        $nestedSentinelRoot = Join-Path $localRoot "preexisting/nested"
        New-Item -ItemType Directory -Path $evidenceRoot -Force | Out-Null
        New-Item -ItemType Directory -Path $nestedSentinelRoot -Force | Out-Null
        Set-Content -LiteralPath (Join-Path $localRoot "local-root-sentinel.txt") -Value "preserve local root" -Encoding UTF8
        Set-Content -LiteralPath (Join-Path $evidenceRoot "evidence-root-sentinel.txt") -Value "preserve evidence root" -Encoding UTF8
        Set-Content -LiteralPath (Join-Path $nestedSentinelRoot "nested-sentinel.txt") -Value "preserve nested directory" -Encoding UTF8

        $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-cleanup" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe

        $result | Should Be 0
        $composeCalls = @($calls | Where-Object { $_.FilePath -eq "docker" -and $_.Arguments[0] -eq "compose" -and $_.Arguments -contains "-p" })
        $composeCalls.Count | Should BeGreaterThan 0
        foreach ($call in $composeCalls) {
            $projectIndex = [array]::IndexOf($call.Arguments, "-p")
            $call.Arguments[$projectIndex + 1] | Should Be "task-processor-openmeter-poc"
        }
        $downCalls = @($composeCalls | Where-Object { $_.Arguments -contains "down" })
        $downCalls.Count | Should Be 1
        $downCalls[0].Arguments -contains "-v" | Should Be $false
        $downIndex = [array]::IndexOf($composeCalls, $downCalls[0])
        $postDownChecks = @($composeCalls | Select-Object -Skip ($downIndex + 1) | Where-Object { $_.Arguments -contains "ps" -and $_.Arguments -contains "-q" })
        $postDownChecks.Count | Should Be 1
        $runnerLog = Get-Content -LiteralPath (Join-Path $repositoryRoot ".local/openmeter-poc/evidence/run-cleanup/runner.log") -Raw
        $runnerLog | Should Match "cleanup verified: no Compose containers remain"
        Test-Path -LiteralPath (Join-Path $repositoryRoot ".local/openmeter-poc/upstream/sentinel.txt") | Should Be $true
        Test-Path -LiteralPath (Join-Path $localRoot "local-root-sentinel.txt") | Should Be $true
        Test-Path -LiteralPath (Join-Path $evidenceRoot "evidence-root-sentinel.txt") | Should Be $true
        Test-Path -LiteralPath (Join-Path $nestedSentinelRoot "nested-sentinel.txt") | Should Be $true
    }

    It "returns nonzero when project containers remain after Compose down" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "cleanup-containers"
        $repositoryRoot = Join-Path $TestDrive "runner-cleanup-containers"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null

        $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-cleanup-containers" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe

        $result | Should Not Be 0
        $runnerLog = Get-Content -LiteralPath (Join-Path $repositoryRoot ".local/openmeter-poc/evidence/run-cleanup-containers/runner.log") -Raw
        $runnerLog | Should Match "CLEANUP FAILED"
    }

    It "keeps all persisted evidence free of credentials while validating raw Compose output" {
        $secret = @("api", "credential", "42") -join "-"
        $sensitiveValues = @{
            DatabasePassword = @("db", "credential", "42") -join "-"
            JWTSecret = @("jwt", "credential", "42") -join "-"
            DatabaseURL = "postgresql://writer:$(@('dsn', 'credential', '42') -join '-')@postgres/openmeter"
            UserInfoURL = "https://callback:$(@('userinfo', 'credential', '42') -join '-')@service.example/hook"
        }
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -Secret $secret -SensitiveValues $sensitiveValues
        $repositoryRoot = Join-Path $TestDrive "runner-redaction"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        $oldPhase = [Environment]::GetEnvironmentVariable("OPENMETER_POC_PHASE", "Process")
        [Environment]::SetEnvironmentVariable("OPENMETER_POC_PHASE", "caller-phase", "Process")
        try {
            $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-redaction" -ApiKey $secret -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe
            $evidenceRoot = Join-Path $repositoryRoot ".local/openmeter-poc/evidence/run-redaction"
            $evidenceFiles = @(Get-ChildItem -Path $evidenceRoot -File -Recurse)

            $result | Should Be 0
            $evidenceFiles.Count | Should BeGreaterThan 0
            foreach ($file in $evidenceFiles) {
                $content = Get-Content -LiteralPath $file.FullName -Raw
                foreach ($sensitiveValue in @($secret) + @($sensitiveValues.Values)) {
                    $content.Contains([string]$sensitiveValue) | Should Be $false
                }
            }
            $renderedModel = Get-Content -LiteralPath (Join-Path $evidenceRoot "compose.rendered.json") -Raw | ConvertFrom-Json
            $renderedModel.services.openmeter.image | Should Be "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232"
            @($renderedModel.services.PSObject.Properties).Count | Should Be 7
            $null -eq $renderedModel.services.openmeter.PSObject.Properties["environment"] | Should Be $true
            $null -eq $renderedModel.services.postgres.PSObject.Properties["environment"] | Should Be $true
            [Environment]::GetEnvironmentVariable("OPENMETER_POC_PHASE", "Process") | Should Be "caller-phase"
        }
        finally {
            [Environment]::SetEnvironmentVariable("OPENMETER_POC_PHASE", $oldPhase, "Process")
        }
    }

    It "rejects an unsafe image from raw Compose output before starting services" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "raw-compose-latest"
        $repositoryRoot = Join-Path $TestDrive "runner-raw-compose-latest"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null

        $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-raw-compose-latest" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe

        $result | Should Not Be 0
        @($calls | Where-Object { $_.FilePath -eq "docker" -and $_.Arguments -contains "up" }).Count | Should Be 0
    }

    It "runs exact JSON Go targets in deterministic outage and recovery order" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls
        $repositoryRoot = Join-Path $TestDrive "runner-lifecycle"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null

        $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-lifecycle" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe

        $result | Should Be 0
        $goCalls = @($calls | Where-Object { $_.FilePath -eq "go" -and $_.Arguments[0] -eq "test" })
        $goCalls.Count | Should Be 5
        [string]::IsNullOrEmpty([string]$goCalls[0].Enabled) | Should Be $true
        foreach ($call in @($goCalls | Select-Object -Skip 1)) {
            $call.Enabled | Should Be "1"
        }
        $expectations = @(
            @{ Phase = ""; Regex = "OpenMeter|UsageEvent|Client|PoC"; Test = "TestOpenMeterImportsStayInsideIsolatedAdapter"; Log = "go-test-default.log" },
            @{ Phase = "contract"; Regex = "^TestPoC"; Test = "TestPoCCountMetersAggregateCommittedSuccesses"; Log = "go-test-contract.log" },
            @{ Phase = "seed"; Regex = "^TestPoCReplaySeed$"; Test = "TestPoCReplaySeed"; Log = "go-test-replay-seed.log" },
            @{ Phase = "unavailable"; Regex = "^TestPoCUnavailableClassifiesFailureAsRetryable$"; Test = "TestPoCUnavailableClassifiesFailureAsRetryable"; Log = "go-test-replay-unavailable.log" },
            @{ Phase = "replay"; Regex = "^TestPoCReplayAfterRecoveryConvergesExactly$"; Test = "TestPoCReplayAfterRecoveryConvergesExactly"; Log = "go-test-replay-recovery.log" }
        )
        for ($index = 0; $index -lt $expectations.Count; $index++) {
            $call = $goCalls[$index]
            $expectation = $expectations[$index]
            [string]$call.Phase | Should Be $expectation.Phase
            $call.Arguments -contains "-json" | Should Be $true
            $runIndex = [array]::IndexOf($call.Arguments, "-run")
            $runIndex | Should BeGreaterThan -1
            $call.Arguments[$runIndex + 1] | Should Be $expectation.Regex

            $logPath = Join-Path $repositoryRoot ".local/openmeter-poc/evidence/run-lifecycle/$($expectation.Log)"
            $events = @(Get-Content -LiteralPath $logPath | ForEach-Object { $_ | ConvertFrom-Json })
            @($events | Where-Object { $_.Action -eq "pass" -and $_.Test -eq $expectation.Test }).Count | Should Be 1
        }

        $runnerLog = Get-Content -LiteralPath (Join-Path $repositoryRoot ".local/openmeter-poc/evidence/run-lifecycle/runner.log") -Raw
        $seedPosition = $runnerLog.IndexOf('^TestPoCReplaySeed$')
        $stopPosition = $runnerLog.IndexOf('stop openmeter', $seedPosition)
        $unavailablePosition = $runnerLog.IndexOf('^TestPoCUnavailableClassifiesFailureAsRetryable$', $stopPosition)
        $restartPosition = $runnerLog.IndexOf('up -d --wait openmeter', $unavailablePosition)
        $healthPosition = $runnerLog.IndexOf('health verified: http://127.0.0.1:48888/api/v1/debug/metrics', $restartPosition)
        $replayPosition = $runnerLog.IndexOf('^TestPoCReplayAfterRecoveryConvergesExactly$', $healthPosition)
        $seedPosition | Should BeGreaterThan -1
        $stopPosition | Should BeGreaterThan $seedPosition
        $unavailablePosition | Should BeGreaterThan $stopPosition
        $restartPosition | Should BeGreaterThan $unavailablePosition
        $healthPosition | Should BeGreaterThan $restartPosition
        $replayPosition | Should BeGreaterThan $healthPosition
    }

    It "returns nonzero when the selected Go target skips despite exit zero" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "go-skip-seed"
        $repositoryRoot = Join-Path $TestDrive "runner-go-skip"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null

        $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-go-skip" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe
        $seedLog = Get-Content -LiteralPath (Join-Path $repositoryRoot ".local/openmeter-poc/evidence/run-go-skip/go-test-replay-seed.log") -Raw

        $seedLog | Should Match '"Action":"skip"'
        $seedLog | Should Match '"Test":"TestPoCReplaySeed"'
        $result | Should Not Be 0
    }

    It "returns nonzero when the selected Go target is missing despite exit zero" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "go-missing-seed"
        $repositoryRoot = Join-Path $TestDrive "runner-go-missing"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null

        $result = Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-go-missing" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe
        $seedLog = Get-Content -LiteralPath (Join-Path $repositoryRoot ".local/openmeter-poc/evidence/run-go-missing/go-test-replay-seed.log") -Raw

        $seedLog | Should Match '"Action":"pass"'
        $seedLog | Should Match '"Test":"TestPoCUnexpectedTarget"'
        $result | Should Not Be 0
    }

    It "returns nonzero when the official checkout clone fails" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "clone"
        $repositoryRoot = Join-Path $TestDrive "runner-clone"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-clone" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe | Should Not Be 0
    }

    It "returns nonzero when the official quickstart differs from the pinned tag" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "dirty-quickstart"
        $repositoryRoot = Join-Path $TestDrive "runner-dirty-quickstart"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-dirty" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe | Should Not Be 0
    }

    It "returns nonzero when Compose health verification fails" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "health"
        $repositoryRoot = Join-Path $TestDrive "runner-health"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-health" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe | Should Not Be 0
    }

    It "returns nonzero when image digest resolution is empty" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "digest"
        $repositoryRoot = Join-Path $TestDrive "runner-digest"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-digest" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe | Should Not Be 0
    }

    It "returns nonzero when a Go phase fails" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "go"
        $repositoryRoot = Join-Path $TestDrive "runner-go"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-go" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe | Should Not Be 0
    }

    It "returns nonzero when resource capture fails" {
        $calls = New-Object System.Collections.ArrayList
        $fakes = New-TestOpenMeterPoCFakes -Calls $calls -FailureMode "resource"
        $repositoryRoot = Join-Path $TestDrive "runner-resource"
        New-Item -ItemType Directory -Path $repositoryRoot -Force | Out-Null
        Invoke-OpenMeterPoC -RepositoryRoot $repositoryRoot -RunId "run-resource" -CommandInvoker $fakes.CommandInvoker -HealthProbe $fakes.HealthProbe | Should Not Be 0
    }
}
