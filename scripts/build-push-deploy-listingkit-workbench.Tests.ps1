$scriptPath = Join-Path $PSScriptRoot "build-push-deploy-listingkit-workbench.ps1"

Describe "build-push-deploy-listingkit-workbench release gate" {
    $previousBash = $null
    $previousProgramFiles = $null
    $commandLog = $null
    $preflightExitCode = 0
    $apiApplyExitCode = 0

    BeforeEach {
        $previousBash = [Environment]::GetEnvironmentVariable("LISTINGKIT_BASH", "Process")
        $previousProgramFiles = [Environment]::GetEnvironmentVariable("ProgramFiles", "Process")
        [Environment]::SetEnvironmentVariable("LISTINGKIT_BASH", "bash", "Process")
        $commandLog = New-Object System.Collections.Generic.List[string]
        $preflightExitCode = 0
        $apiApplyExitCode = 0

        function global:docker {
            $commandLog.Add("docker " + ($args -join " "))
            if ($args[0] -eq "image" -and $args[1] -eq "inspect") {
                $repository = ($args[-1] -replace ':[^/:]+$', '')
                if ($args[-1] -match "identity-preflight") {
                    Write-Output "$repository@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
                } else {
                    Write-Output "$repository@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                }
            }
            $global:LASTEXITCODE = 0
        }

        function global:bash {
            $command = "bash " + ($args -join " ")
            $commandLog.Add($command)
            if ($command -match "listingkit-identity-preflight-job\.sh") {
                $global:LASTEXITCODE = $preflightExitCode
                return
            }
            if ($command -match "listingkit-apply-api-deployment\.sh") {
                $global:LASTEXITCODE = $apiApplyExitCode
                return
            }
            $global:LASTEXITCODE = 0
        }

        function global:kubectl {
            $commandLog.Add("kubectl " + ($args -join " "))
            $global:LASTEXITCODE = 0
        }
    }

    AfterEach {
        [Environment]::SetEnvironmentVariable("LISTINGKIT_BASH", $previousBash, "Process")
        [Environment]::SetEnvironmentVariable("ProgramFiles", $previousProgramFiles, "Process")
        Remove-Item Function:\global:docker -ErrorAction SilentlyContinue
        Remove-Item Function:\global:bash -ErrorAction SilentlyContinue
        Remove-Item Function:\global:kubectl -ErrorAction SilentlyContinue
    }

    It "skips every Kubernetes mutation when SkipApply is requested" {
        & $scriptPath -Tag "release-20260810" -SkipTests -SkipApply

        @($commandLog | Where-Object { $_ -match "^(bash|kubectl) " }).Count | Should Be 0
    }

    It "runs preflight then immutable API apply before the matching UI update" {
        & $scriptPath -Tag "release-20260810" -SkipTests

        $preflightIndex = $commandLog.FindIndex([Predicate[string]]{ param($command) $command -match "listingkit-identity-preflight-job\.sh" })
        $apiApplyIndex = $commandLog.FindIndex([Predicate[string]]{ param($command) $command -match "listingkit-apply-api-deployment\.sh" })
        $uiUpdateIndex = $commandLog.FindIndex([Predicate[string]]{ param($command) $command -match "set image deployment/listingkit-ui" })

        $preflightIndex | Should BeGreaterThan -1
        $apiApplyIndex | Should BeGreaterThan $preflightIndex
        $uiUpdateIndex | Should BeGreaterThan $apiApplyIndex
        $commandLog[$preflightIndex] | Should Match "--image xuwei190/task-processor-product-listing-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        $commandLog[$preflightIndex] | Should Match "--runner-image xuwei190/task-processor-listingkit-identity-preflight@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
        $commandLog[$apiApplyIndex] | Should Match "--image xuwei190/task-processor-product-listing-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        ($commandLog -join "`n") | Should Not Match "apply -k"
        ($commandLog -join "`n") | Should Not Match "set image deployment/product-listing-api"
    }

    It "preflights and applies the same custom-registry API image" {
        & $scriptPath -Tag "release-20260810" -DockerHubUser "alternate-registry" -SkipTests

        $preflightCommand = @($commandLog | Where-Object { $_ -match "listingkit-identity-preflight-job\.sh" })
        $apiApplyCommand = @($commandLog | Where-Object { $_ -match "listingkit-apply-api-deployment\.sh" })
        $preflightCommand.Count | Should Be 1
        $apiApplyCommand.Count | Should Be 1
        $preflightCommand[0] | Should Match "--image alternate-registry/task-processor-product-listing-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        $apiApplyCommand[0] | Should Match "--image alternate-registry/task-processor-product-listing-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }

    It "prevents every deployment mutation when the identity preflight fails" {
        $preflightExitCode = 1

        { & $scriptPath -Tag "release-20260810" -SkipTests } | Should Throw "identity preflight failed"

        ($commandLog -join "`n") | Should Match "listingkit-identity-preflight-job\.sh"
        ($commandLog -join "`n") | Should Not Match "listingkit-apply-api-deployment\.sh"
        ($commandLog -join "`n") | Should Not Match "kubectl"
    }

    It "prevents UI or rollout mutations when immutable API apply fails" {
        $apiApplyExitCode = 1

        { & $scriptPath -Tag "release-20260810" -SkipTests } | Should Throw "immutable API deployment apply failed"

        ($commandLog -join "`n") | Should Match "listingkit-identity-preflight-job\.sh"
        ($commandLog -join "`n") | Should Match "listingkit-apply-api-deployment\.sh"
        ($commandLog -join "`n") | Should Not Match "kubectl"
    }

    It "rejects a latest release candidate before any external command" {
        { & $scriptPath -Tag "latest" -SkipTests -SkipApply } | Should Throw "ListingKit release requires an immutable API image tag; latest is not a release candidate"

        $commandLog.Count | Should Be 0
    }

    It "rejects an explicitly empty release candidate before any external command" {
        { & $scriptPath -Tag "" -SkipTests -SkipApply } | Should Throw "ListingKit release requires a non-empty immutable API image tag"

        $commandLog.Count | Should Be 0
    }

    It "does not require Git Bash for the build-and-push-only SkipApply mode" {
        [Environment]::SetEnvironmentVariable("LISTINGKIT_BASH", $null, "Process")
        [Environment]::SetEnvironmentVariable("ProgramFiles", $TestDrive, "Process")
        function global:Get-Command { throw "SkipApply must not resolve a Bash executable" }

        try {
            & $scriptPath -Tag "release-20260810" -SkipTests -SkipApply
        } finally {
            Remove-Item Function:\global:Get-Command -ErrorAction SilentlyContinue
        }

        @($commandLog | Where-Object { $_ -match "^(bash|kubectl) " }).Count | Should Be 0
    }
}
