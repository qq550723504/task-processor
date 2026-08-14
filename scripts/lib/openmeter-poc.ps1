$script:OpenMeterPoCTag = "v1.0.0-beta.232"
$script:OpenMeterPoCImage = "ghcr.io/openmeterio/openmeter:v1.0.0-beta.232"
$script:OpenMeterPoCRepository = "https://github.com/openmeterio/openmeter.git"
$script:OpenMeterPoCComposeProject = "task-processor-openmeter-poc"
$script:OpenMeterPoCURL = "http://127.0.0.1:48888/api/v3"
$script:OpenMeterPoCHealthURL = "http://127.0.0.1:48888/api/v1/debug/metrics"
$script:OpenMeterPoCOwnedServices = @(
    "openmeter",
    "sink-worker",
    "balance-worker",
    "notification-service",
    "billing-worker",
    "openmeter-jobs"
)

function Assert-OpenMeterPoCChildPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$AllowedRoot
    )

    $root = [System.IO.Path]::GetFullPath($AllowedRoot).TrimEnd('\', '/')
    $candidate = [System.IO.Path]::GetFullPath($Path)
    $prefix = $root + [System.IO.Path]::DirectorySeparatorChar
    if ($candidate -ne $root -and -not $candidate.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "path escapes OpenMeter PoC root: $candidate"
    }
    return $candidate
}

function Get-OpenMeterPoCPaths {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryRoot,
        [Parameter(Mandatory = $true)]
        [string]$RunId
    )

    if ($RunId.Length -gt 40 -or $RunId -cnotmatch '^[a-z0-9]+(?:-[a-z0-9]+)*$') {
        throw "RunId must match ^[a-z0-9]+(?:-[a-z0-9]+)*$ and contain at most 40 characters"
    }

    $repositoryPath = [System.IO.Path]::GetFullPath($RepositoryRoot)
    if (-not (Test-Path -LiteralPath $repositoryPath -PathType Container)) {
        throw "repository root does not exist: $repositoryPath"
    }

    $localRoot = [System.IO.Path]::GetFullPath((Join-Path $repositoryPath ".local/openmeter-poc"))
    $checkoutPath = Assert-OpenMeterPoCChildPath -Path (Join-Path $localRoot "upstream") -AllowedRoot $localRoot
    $evidenceRoot = Assert-OpenMeterPoCChildPath -Path (Join-Path $localRoot "evidence") -AllowedRoot $localRoot
    $evidencePath = Assert-OpenMeterPoCChildPath -Path (Join-Path $evidenceRoot $RunId) -AllowedRoot $localRoot
    $overridePath = Assert-OpenMeterPoCChildPath -Path (Join-Path $localRoot "compose.override.yaml") -AllowedRoot $localRoot

    [pscustomobject]@{
        RepositoryRoot = $repositoryPath
        LocalRoot = $localRoot
        CheckoutPath = $checkoutPath
        EvidenceRoot = $evidenceRoot
        EvidencePath = $evidencePath
        OverridePath = $overridePath
        BaseComposePath = Assert-OpenMeterPoCChildPath -Path (Join-Path $checkoutPath "quickstart/docker-compose.yaml") -AllowedRoot $localRoot
        RunnerLogPath = Assert-OpenMeterPoCChildPath -Path (Join-Path $evidencePath "runner.log") -AllowedRoot $localRoot
        RenderedComposePath = Assert-OpenMeterPoCChildPath -Path (Join-Path $evidencePath "compose.rendered.json") -AllowedRoot $localRoot
    }
}

function Protect-OpenMeterPoCText {
    param(
        [AllowNull()]
        [object]$Text,
        [AllowEmptyString()]
        [string]$ApiKey = ""
    )

    $value = if ($null -eq $Text) { "" } else { [string]$Text }
    if (-not [string]::IsNullOrEmpty($ApiKey)) {
        $value = $value.Replace($ApiKey, "[REDACTED]")
    }

    $secretName = '(?:password|passwd|secret|token|api[_-]?key|private[_-]?key|dsn|connection(?:[_-]?(?:string|url))?|database[_-]?url)'
    $value = [regex]::Replace(
        $value,
        '(?i)("(?:[^"\\]|\\.)*' + $secretName + '(?:[^"\\]|\\.)*"\s*:\s*)"(?:[^"\\]|\\.)*"',
        '$1"[REDACTED]"'
    )
    $value = [regex]::Replace(
        $value,
        '(?i)("(?:authorization|proxy-authorization)"\s*:\s*)"(?:[^"\\]|\\.)*"',
        '$1"[REDACTED]"'
    )
    $value = [regex]::Replace(
        $value,
        '(?im)^(\s*[A-Za-z0-9_.-]*' + $secretName + '[A-Za-z0-9_.-]*\s*:\s*)(?:"(?:[^"\\]|\\.)*"|''[^'']*''|[^\r\n#]*)',
        '$1[REDACTED]'
    )
    $value = [regex]::Replace(
        $value,
        '(?i)(\b[A-Za-z0-9_.-]*' + $secretName + '[A-Za-z0-9_.-]*\s*=\s*)(?:"(?:[^"\\]|\\.)*"|''[^'']*''|[^\s,;"''\\}]+)',
        '$1[REDACTED]'
    )
    $value = [regex]::Replace(
        $value,
        '(?i)\b([a-z][a-z0-9+.-]*://)(?:[^/\s@"]+)@',
        '$1[REDACTED]@'
    )
    $value = [regex]::Replace(
        $value,
        '(?i)([?&]' + $secretName + '=)[^&#\s"]+',
        '$1[REDACTED]'
    )
    $value = [regex]::Replace(
        $value,
        '(?im)^(\s*(?:authorization|proxy-authorization)\s*:\s*(?:bearer|basic)\s+).+$',
        '$1[REDACTED]'
    )
    return $value
}

function Assert-OpenMeterPoCLocalEndpoint {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri
    )

    $expected = "http://127.0.0.1:48888/api/v3"
    if (-not [string]::Equals($Uri, $expected, [System.StringComparison]::Ordinal)) {
        throw "OpenMeter PoC SDK endpoint must be exactly http://127.0.0.1:48888/api/v3"
    }
}

function Write-OpenMeterPoCFile {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$AllowedRoot,
        [AllowNull()]
        [object]$Value,
        [AllowEmptyString()]
        [string]$ApiKey = "",
        [switch]$Append
    )

    $safePath = Assert-OpenMeterPoCChildPath -Path $Path -AllowedRoot $AllowedRoot
    $parent = Split-Path -Parent $safePath
    $null = New-Item -ItemType Directory -Path $parent -Force -ErrorAction Stop
    $protected = Protect-OpenMeterPoCText -Text $Value -ApiKey $ApiKey
    if ($Append) {
        Add-Content -LiteralPath $safePath -Value $protected -Encoding UTF8 -ErrorAction Stop
    }
    else {
        Set-Content -LiteralPath $safePath -Value $protected -Encoding UTF8 -ErrorAction Stop
    }
}

function New-OpenMeterPoCComposeOverride {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$AllowedRoot
    )

    $override = @"
services:
  openmeter:
    image: $script:OpenMeterPoCImage
  sink-worker:
    image: $script:OpenMeterPoCImage
  balance-worker:
    image: $script:OpenMeterPoCImage
  notification-service:
    image: $script:OpenMeterPoCImage
  billing-worker:
    image: $script:OpenMeterPoCImage
  openmeter-jobs:
    image: $script:OpenMeterPoCImage
"@
    Write-OpenMeterPoCFile -Path $Path -AllowedRoot $AllowedRoot -Value $override
}

function Assert-OpenMeterPoCRenderedCompose {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Json
    )

    try {
        $model = $Json | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        throw "rendered Compose config is not valid JSON: $($_.Exception.Message)"
    }
    if ($null -eq $model.services) {
        throw "rendered Compose config has no services"
    }

    $images = [ordered]@{}
    $sanitizedServices = [ordered]@{}
    foreach ($property in $model.services.PSObject.Properties) {
        $serviceImage = [string]$property.Value.image
        if (-not [string]::IsNullOrWhiteSpace($serviceImage)) {
            $images[$property.Name] = $serviceImage
        }

        $sanitizedService = [ordered]@{}
        if (-not [string]::IsNullOrWhiteSpace($serviceImage)) {
            $sanitizedService.image = $serviceImage
        }
        if ($null -ne $property.Value.ports) {
            $sanitizedPorts = @()
            foreach ($port in @($property.Value.ports)) {
                if ($port -is [string]) {
                    $sanitizedPorts += [string]$port
                    continue
                }
                $sanitizedPort = [ordered]@{}
                foreach ($field in @("host_ip", "target", "published", "protocol", "mode")) {
                    $portProperty = $port.PSObject.Properties[$field]
                    if ($null -ne $portProperty) {
                        $sanitizedPort[$field] = $portProperty.Value
                    }
                }
                $sanitizedPorts += [pscustomobject]$sanitizedPort
            }
            $sanitizedService.ports = $sanitizedPorts
        }
        $sanitizedServices[$property.Name] = [pscustomobject]$sanitizedService
    }
    foreach ($service in $script:OpenMeterPoCOwnedServices) {
        $property = $model.services.PSObject.Properties[$service]
        if ($null -eq $property) {
            throw "rendered Compose config is missing OpenMeter-owned service $service"
        }
        $image = [string]$property.Value.image
        if ($image -match '(^|:)latest$') {
            throw "rendered Compose service $service still uses latest"
        }
        if ($image -ne $script:OpenMeterPoCImage) {
            throw "rendered Compose service $service uses $image instead of $script:OpenMeterPoCImage"
        }
    }
    return [pscustomobject]@{
        Images = $images
        SanitizedModel = [ordered]@{
            name = [string]$model.name
            services = $sanitizedServices
        }
    }
}

function Invoke-OpenMeterPoCNativeCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [string[]]$ArgumentList = @(),
        [Parameter(Mandatory = $true)]
        [string]$WorkingDirectory
    )

    $output = @()
    $exitCode = 1
    Push-Location -LiteralPath $WorkingDirectory -ErrorAction Stop
    try {
        try {
            $output = @(& $FilePath @ArgumentList 2>&1)
            $exitCode = [int]$LASTEXITCODE
        }
        catch {
            $output = @($_.Exception.Message)
            $exitCode = 1
        }
    }
    finally {
        Pop-Location
    }

    [pscustomobject]@{
        ExitCode = $exitCode
        Output = ($output | ForEach-Object { [string]$_ }) -join [Environment]::NewLine
    }
}

function Invoke-OpenMeterPoCRequiredCommand {
    param(
        [Parameter(Mandatory = $true)]
        [scriptblock]$CommandInvoker,
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [string[]]$ArgumentList = @(),
        [Parameter(Mandatory = $true)]
        [string]$WorkingDirectory,
        [Parameter(Mandatory = $true)]
        [pscustomobject]$Paths,
        [AllowEmptyString()]
        [string]$ApiKey = "",
        [string]$OutputPath = "",
        [switch]$SuppressOutputLog
    )

    $result = & $CommandInvoker $FilePath $ArgumentList $WorkingDirectory
    if ($null -eq $result -or $null -eq $result.PSObject.Properties["ExitCode"] -or $null -eq $result.PSObject.Properties["Output"]) {
        throw "command invoker returned an invalid result for $FilePath"
    }

    $safeOutput = Protect-OpenMeterPoCText -Text $result.Output -ApiKey $ApiKey
    $displayArguments = Protect-OpenMeterPoCText -Text ($ArgumentList -join " ") -ApiKey $ApiKey
    $logEntry = "> $FilePath $displayArguments`nexit=$($result.ExitCode)"
    if (-not $SuppressOutputLog -and -not [string]::IsNullOrEmpty($safeOutput)) {
        $logEntry += "`n$safeOutput"
    }
    Write-OpenMeterPoCFile -Path $Paths.RunnerLogPath -AllowedRoot $Paths.LocalRoot -Value $logEntry -ApiKey $ApiKey -Append

    if (-not [string]::IsNullOrEmpty($OutputPath)) {
        Write-OpenMeterPoCFile -Path $OutputPath -AllowedRoot $Paths.LocalRoot -Value $safeOutput -ApiKey $ApiKey
    }
    if ([int]$result.ExitCode -ne 0) {
        throw "$FilePath command failed with exit code $($result.ExitCode)"
    }
    return [string]$result.Output
}

function Get-OpenMeterPoCComposeArguments {
    param(
        [Parameter(Mandatory = $true)]
        [pscustomobject]$Paths,
        [string[]]$Tail = @()
    )

    return @(
        "compose",
        "-p", $script:OpenMeterPoCComposeProject,
        "-f", $Paths.BaseComposePath,
        "-f", $Paths.OverridePath
    ) + $Tail
}

function Initialize-OpenMeterPoCCheckout {
    param(
        [Parameter(Mandatory = $true)]
        [pscustomobject]$Paths,
        [Parameter(Mandatory = $true)]
        [scriptblock]$CommandInvoker,
        [AllowEmptyString()]
        [string]$ApiKey = ""
    )

    if (-not (Test-Path -LiteralPath $Paths.CheckoutPath)) {
        $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "git" -ArgumentList @(
            "clone", "--depth", "1", "--single-branch", "--branch", $script:OpenMeterPoCTag,
            $script:OpenMeterPoCRepository, $Paths.CheckoutPath
        ) -WorkingDirectory $Paths.LocalRoot -Paths $Paths -ApiKey $ApiKey
    }
    elseif (-not (Test-Path -LiteralPath (Join-Path $Paths.CheckoutPath ".git") -PathType Container)) {
        throw "existing OpenMeter checkout path is not a Git checkout: $($Paths.CheckoutPath)"
    }

    $origin = (Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "git" -ArgumentList @(
        "-C", $Paths.CheckoutPath, "remote", "get-url", "origin"
    ) -WorkingDirectory $Paths.RepositoryRoot -Paths $Paths -ApiKey $ApiKey).Trim()
    $allowedOrigins = @(
        "https://github.com/openmeterio/openmeter.git",
        "https://github.com/openmeterio/openmeter",
        "git@github.com:openmeterio/openmeter.git",
        "ssh://git@github.com/openmeterio/openmeter.git"
    )
    if ($origin -notin $allowedOrigins) {
        throw "existing OpenMeter checkout has unexpected origin: $origin"
    }

    $tag = (Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "git" -ArgumentList @(
        "-C", $Paths.CheckoutPath, "describe", "--tags", "--exact-match", "HEAD"
    ) -WorkingDirectory $Paths.RepositoryRoot -Paths $Paths -ApiKey $ApiKey).Trim()
    if ($tag -ne $script:OpenMeterPoCTag) {
        throw "existing OpenMeter checkout is at tag $tag instead of $script:OpenMeterPoCTag"
    }
    if (-not (Test-Path -LiteralPath $Paths.BaseComposePath -PathType Leaf)) {
        throw "official OpenMeter quickstart Compose file is missing: $($Paths.BaseComposePath)"
    }
    $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "git" -ArgumentList @(
        "-C", $Paths.CheckoutPath, "diff", "--quiet", "HEAD", "--", "quickstart/docker-compose.yaml"
    ) -WorkingDirectory $Paths.RepositoryRoot -Paths $Paths -ApiKey $ApiKey

    $upstreamSHAPath = Join-Path $Paths.EvidencePath "upstream-git-sha.txt"
    $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "git" -ArgumentList @(
        "-C", $Paths.CheckoutPath, "rev-parse", "HEAD"
    ) -WorkingDirectory $Paths.RepositoryRoot -Paths $Paths -ApiKey $ApiKey -OutputPath $upstreamSHAPath
}

function Test-OpenMeterPoCHealth {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri,
        [AllowEmptyString()]
        [string]$ApiKey = ""
    )

    $headers = @{}
    if (-not [string]::IsNullOrEmpty($ApiKey)) {
        $headers.Authorization = "Bearer $ApiKey"
    }
    try {
        $response = Invoke-WebRequest -Uri $Uri -Method Get -Headers $headers -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop
        return [int]$response.StatusCode -ge 200 -and [int]$response.StatusCode -lt 400
    }
    catch {
        return $false
    }
}

function Assert-OpenMeterPoCHealth {
    param(
        [AllowNull()]
        [scriptblock]$HealthProbe,
        [Parameter(Mandatory = $true)]
        [pscustomobject]$Paths,
        [AllowEmptyString()]
        [string]$ApiKey = ""
    )

    $healthy = if ($null -eq $HealthProbe) {
        Test-OpenMeterPoCHealth -Uri $script:OpenMeterPoCHealthURL -ApiKey $ApiKey
    }
    else {
        & $HealthProbe $script:OpenMeterPoCHealthURL
    }
    if (-not $healthy) {
        throw "OpenMeter API health verification failed at $script:OpenMeterPoCHealthURL"
    }
    Write-OpenMeterPoCFile -Path $Paths.RunnerLogPath -AllowedRoot $Paths.LocalRoot -Value "health verified: $script:OpenMeterPoCHealthURL" -ApiKey $ApiKey -Append
}

function Save-OpenMeterPoCResourceSnapshot {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,
        [Parameter(Mandatory = $true)]
        [pscustomobject]$Paths,
        [Parameter(Mandatory = $true)]
        [scriptblock]$CommandInvoker,
        [AllowEmptyString()]
        [string]$ApiKey = ""
    )

    $containerOutput = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $Paths -Tail @("ps", "-q")) -WorkingDirectory (Split-Path -Parent $Paths.BaseComposePath) -Paths $Paths -ApiKey $ApiKey
    $containerIDs = @($containerOutput -split '\r?\n' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($containerIDs.Count -eq 0) {
        throw "resource capture found no Compose containers"
    }

    $statsPath = Join-Path $Paths.EvidencePath "docker-stats-$Name.jsonl"
    $stats = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (@(
        "stats", "--no-stream", "--format", "{{json .}}"
    ) + $containerIDs) -WorkingDirectory $Paths.RepositoryRoot -Paths $Paths -ApiKey $ApiKey -OutputPath $statsPath
    if ([string]::IsNullOrWhiteSpace($stats)) {
        throw "resource capture returned empty Docker stats"
    }
}

function Save-OpenMeterPoCInventory {
    param(
        [Parameter(Mandatory = $true)]
        [pscustomobject]$Paths,
        [Parameter(Mandatory = $true)]
        [scriptblock]$CommandInvoker,
        [Parameter(Mandatory = $true)]
        [System.Collections.IDictionary]$Images,
        [AllowEmptyString()]
        [string]$ApiKey = ""
    )

    $serviceListPath = Join-Path $Paths.EvidencePath "compose-services.jsonl"
    $services = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $Paths -Tail @("ps", "--format", "json")) -WorkingDirectory (Split-Path -Parent $Paths.BaseComposePath) -Paths $Paths -ApiKey $ApiKey -OutputPath $serviceListPath
    if ([string]::IsNullOrWhiteSpace($services)) {
        throw "service inventory capture returned empty output"
    }

    $tagLines = foreach ($service in $Images.Keys) {
        "$service=$($Images[$service])"
    }
    Write-OpenMeterPoCFile -Path (Join-Path $Paths.EvidencePath "image-tags.txt") -AllowedRoot $Paths.LocalRoot -Value ($tagLines -join "`n") -ApiKey $ApiKey

    $digestPath = Join-Path $Paths.EvidencePath "openmeter-repo-digests.json"
    $digestInventory = [ordered]@{}
    foreach ($image in @($Images.Values | Sort-Object -Unique)) {
        $digestOutput = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList @(
            "image", "inspect", $image, "--format", "{{json .RepoDigests}}"
        ) -WorkingDirectory $Paths.RepositoryRoot -Paths $Paths -ApiKey $ApiKey
        try {
            $digests = @($digestOutput | ConvertFrom-Json -ErrorAction Stop)
        }
        catch {
            throw "image RepoDigests output is invalid JSON for $image"
        }
        if ($digests.Count -eq 0 -or @($digests | Where-Object { $_ -notmatch '@sha256:[0-9a-f]{64}$' }).Count -ne 0) {
            throw "image RepoDigests did not resolve to immutable sha256 digests for $image"
        }
        $digestInventory[$image] = $digests
    }
    Write-OpenMeterPoCFile -Path $digestPath -AllowedRoot $Paths.LocalRoot -Value ($digestInventory | ConvertTo-Json -Depth 4) -ApiKey $ApiKey
}

function Invoke-OpenMeterPoCGoPhase {
    param(
        [AllowNull()]
        [string]$Phase,
        [Parameter(Mandatory = $true)]
        [string[]]$ArgumentList,
        [Parameter(Mandatory = $true)]
        [string]$ExpectedPackage,
        [Parameter(Mandatory = $true)]
        [string]$ExpectedTest,
        [Parameter(Mandatory = $true)]
        [string]$LogName,
        [Parameter(Mandatory = $true)]
        [string]$RunId,
        [Parameter(Mandatory = $true)]
        [pscustomobject]$Paths,
        [Parameter(Mandatory = $true)]
        [scriptblock]$CommandInvoker,
        [AllowEmptyString()]
        [string]$ApiKey = ""
    )

    if ($ArgumentList.Count -eq 0 -or $ArgumentList[0] -ne "test") {
        throw "OpenMeter PoC Go phase must invoke go test"
    }
    $jsonArguments = @($ArgumentList[0], "-json") + @($ArgumentList | Select-Object -Skip 1)
    $names = @("OPENMETER_POC", "OPENMETER_POC_URL", "OPENMETER_POC_RUN_ID", "OPENMETER_POC_PHASE", "OPENMETER_API_KEY")
    $previous = @{}
    foreach ($name in $names) {
        $previous[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
        [Environment]::SetEnvironmentVariable($name, $null, "Process")
    }

    try {
        if (-not [string]::IsNullOrEmpty($Phase)) {
            [Environment]::SetEnvironmentVariable("OPENMETER_POC", "1", "Process")
            [Environment]::SetEnvironmentVariable("OPENMETER_POC_URL", $script:OpenMeterPoCURL, "Process")
            [Environment]::SetEnvironmentVariable("OPENMETER_POC_RUN_ID", $RunId, "Process")
            [Environment]::SetEnvironmentVariable("OPENMETER_POC_PHASE", $Phase, "Process")
            if (-not [string]::IsNullOrEmpty($ApiKey)) {
                [Environment]::SetEnvironmentVariable("OPENMETER_API_KEY", $ApiKey, "Process")
            }
        }

        $outputPath = Join-Path $Paths.EvidencePath $LogName
        $output = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "go" -ArgumentList $jsonArguments -WorkingDirectory $Paths.RepositoryRoot -Paths $Paths -ApiKey $ApiKey -OutputPath $outputPath
        Assert-OpenMeterPoCTestPassed -Output $output -ExpectedPackage $ExpectedPackage -ExpectedTest $ExpectedTest
    }
    finally {
        foreach ($name in $names) {
            [Environment]::SetEnvironmentVariable($name, $previous[$name], "Process")
        }
    }
}

function Assert-OpenMeterPoCTestPassed {
    param(
        [AllowEmptyString()]
        [string]$Output,
        [Parameter(Mandatory = $true)]
        [string]$ExpectedPackage,
        [Parameter(Mandatory = $true)]
        [string]$ExpectedTest
    )

    $targetEvents = @()
    $lineNumber = 0
    foreach ($line in @($Output -split '\r?\n')) {
        $lineNumber++
        if ([string]::IsNullOrWhiteSpace($line)) {
            continue
        }
        try {
            $event = $line | ConvertFrom-Json -ErrorAction Stop
        }
        catch {
            throw "go test -json emitted invalid JSON at line ${lineNumber}: $($_.Exception.Message)"
        }
        if ([string]$event.Package -eq $ExpectedPackage -and [string]$event.Test -eq $ExpectedTest) {
            $targetEvents += $event
        }
    }

    if (@($targetEvents | Where-Object { $_.Action -eq "skip" }).Count -ne 0) {
        throw "required Go test $ExpectedPackage/$ExpectedTest was skipped"
    }
    if (@($targetEvents | Where-Object { $_.Action -eq "fail" }).Count -ne 0) {
        throw "required Go test $ExpectedPackage/$ExpectedTest failed"
    }
    $passCount = @($targetEvents | Where-Object { $_.Action -eq "pass" }).Count
    if ($passCount -ne 1) {
        throw "required Go test $ExpectedPackage/$ExpectedTest emitted $passCount exact PASS events, want 1"
    }
}

function Invoke-OpenMeterPoC {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RepositoryRoot,
        [Parameter(Mandatory = $true)]
        [string]$RunId,
        [AllowEmptyString()]
        [string]$ApiKey = "",
        [switch]$KeepEnvironment,
        [scriptblock]$CommandInvoker,
        [scriptblock]$HealthProbe
    )

    $exitCode = 0
    $paths = $null
    $composeConfigured = $false

    try {
        Assert-OpenMeterPoCLocalEndpoint -Uri $script:OpenMeterPoCURL
        $paths = Get-OpenMeterPoCPaths -RepositoryRoot $RepositoryRoot -RunId $RunId
        $null = New-Item -ItemType Directory -Path $paths.LocalRoot -Force -ErrorAction Stop
        if (Test-Path -LiteralPath $paths.EvidencePath) {
            throw "evidence directory already exists for RunId $RunId"
        }
        $null = New-Item -ItemType Directory -Path $paths.EvidencePath -Force -ErrorAction Stop
        Write-OpenMeterPoCFile -Path $paths.RunnerLogPath -AllowedRoot $paths.LocalRoot -Value "OpenMeter PoC run $RunId started" -ApiKey $ApiKey

        if ($null -eq $CommandInvoker) {
            $CommandInvoker = {
                param([string]$FilePath, [string[]]$ArgumentList, [string]$WorkingDirectory)
                Invoke-OpenMeterPoCNativeCommand -FilePath $FilePath -ArgumentList $ArgumentList -WorkingDirectory $WorkingDirectory
            }
        }
        foreach ($prerequisite in @(
            @{ FilePath = "git"; Arguments = @("--version") },
            @{ FilePath = "go"; Arguments = @("version") },
            @{ FilePath = "docker"; Arguments = @("version", "--format", "{{.Server.Version}}") },
            @{ FilePath = "docker"; Arguments = @("compose", "version") }
        )) {
            $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath $prerequisite.FilePath -ArgumentList $prerequisite.Arguments -WorkingDirectory $paths.RepositoryRoot -Paths $paths -ApiKey $ApiKey
        }

        Initialize-OpenMeterPoCCheckout -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey
        $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "git" -ArgumentList @(
            "-C", $paths.RepositoryRoot, "rev-parse", "HEAD"
        ) -WorkingDirectory $paths.RepositoryRoot -Paths $paths -ApiKey $ApiKey -OutputPath (Join-Path $paths.EvidencePath "task-processor-git-sha.txt")

        New-OpenMeterPoCComposeOverride -Path $paths.OverridePath -AllowedRoot $paths.LocalRoot
        $rawCompose = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $paths -Tail @("config", "--format", "json")) -WorkingDirectory (Split-Path -Parent $paths.BaseComposePath) -Paths $paths -ApiKey $ApiKey -SuppressOutputLog
        $renderedCompose = Assert-OpenMeterPoCRenderedCompose -Json $rawCompose
        Write-OpenMeterPoCFile -Path $paths.RenderedComposePath -AllowedRoot $paths.LocalRoot -Value ($renderedCompose.SanitizedModel | ConvertTo-Json -Depth 8) -ApiKey $ApiKey
        $images = $renderedCompose.Images
        $composeConfigured = $true

        $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $paths -Tail @("up", "-d", "--wait")) -WorkingDirectory (Split-Path -Parent $paths.BaseComposePath) -Paths $paths -ApiKey $ApiKey
        Assert-OpenMeterPoCHealth -HealthProbe $HealthProbe -Paths $paths -ApiKey $ApiKey

        Save-OpenMeterPoCInventory -Paths $paths -CommandInvoker $CommandInvoker -Images $images -ApiKey $ApiKey
        Save-OpenMeterPoCResourceSnapshot -Name "before" -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey

        $defaultArguments = @("test", "./internal/integration/openmeter", "./tests", "-run", "OpenMeter|UsageEvent|Client|PoC", "-count=1")
        Invoke-OpenMeterPoCGoPhase -Phase $null -ArgumentList $defaultArguments -ExpectedPackage "task-processor/tests" -ExpectedTest "TestOpenMeterImportsStayInsideIsolatedAdapter" -LogName "go-test-default.log" -RunId $RunId -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey
        Invoke-OpenMeterPoCGoPhase -Phase "contract" -ArgumentList @("test", "./internal/integration/openmeter", "-run", "^TestPoC", "-count=1", "-v") -ExpectedPackage "task-processor/internal/integration/openmeter" -ExpectedTest "TestPoCCountMetersAggregateCommittedSuccesses" -LogName "go-test-contract.log" -RunId $RunId -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey
        Invoke-OpenMeterPoCGoPhase -Phase "seed" -ArgumentList @("test", "./internal/integration/openmeter", "-run", "^TestPoCReplaySeed$", "-count=1", "-v") -ExpectedPackage "task-processor/internal/integration/openmeter" -ExpectedTest "TestPoCReplaySeed" -LogName "go-test-replay-seed.log" -RunId $RunId -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey

        $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $paths -Tail @("stop", "openmeter")) -WorkingDirectory (Split-Path -Parent $paths.BaseComposePath) -Paths $paths -ApiKey $ApiKey
        Invoke-OpenMeterPoCGoPhase -Phase "unavailable" -ArgumentList @("test", "./internal/integration/openmeter", "-run", "^TestPoCUnavailableClassifiesFailureAsRetryable$", "-count=1", "-v") -ExpectedPackage "task-processor/internal/integration/openmeter" -ExpectedTest "TestPoCUnavailableClassifiesFailureAsRetryable" -LogName "go-test-replay-unavailable.log" -RunId $RunId -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey

        $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $paths -Tail @("up", "-d", "--wait", "openmeter")) -WorkingDirectory (Split-Path -Parent $paths.BaseComposePath) -Paths $paths -ApiKey $ApiKey
        Assert-OpenMeterPoCHealth -HealthProbe $HealthProbe -Paths $paths -ApiKey $ApiKey
        Invoke-OpenMeterPoCGoPhase -Phase "replay" -ArgumentList @("test", "./internal/integration/openmeter", "-run", "^TestPoCReplayAfterRecoveryConvergesExactly$", "-count=1", "-v") -ExpectedPackage "task-processor/internal/integration/openmeter" -ExpectedTest "TestPoCReplayAfterRecoveryConvergesExactly" -LogName "go-test-replay-recovery.log" -RunId $RunId -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey

        Save-OpenMeterPoCResourceSnapshot -Name "after" -Paths $paths -CommandInvoker $CommandInvoker -ApiKey $ApiKey
        Write-OpenMeterPoCFile -Path $paths.RunnerLogPath -AllowedRoot $paths.LocalRoot -Value "OpenMeter PoC run completed" -ApiKey $ApiKey -Append
    }
    catch {
        $exitCode = 1
        $safeError = Protect-OpenMeterPoCText -Text $_.Exception.Message -ApiKey $ApiKey
        if ($null -ne $paths -and (Test-Path -LiteralPath $paths.EvidencePath)) {
            Write-OpenMeterPoCFile -Path $paths.RunnerLogPath -AllowedRoot $paths.LocalRoot -Value "FAILED: $safeError" -ApiKey $ApiKey -Append
        }
        Write-Warning "OpenMeter PoC failed: $safeError"
    }
    finally {
        if ($composeConfigured -and -not $KeepEnvironment -and $null -ne $CommandInvoker) {
            try {
                $null = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $paths -Tail @("down")) -WorkingDirectory (Split-Path -Parent $paths.BaseComposePath) -Paths $paths -ApiKey $ApiKey
                $remainingOutput = Invoke-OpenMeterPoCRequiredCommand -CommandInvoker $CommandInvoker -FilePath "docker" -ArgumentList (Get-OpenMeterPoCComposeArguments -Paths $paths -Tail @("ps", "--all", "-q")) -WorkingDirectory (Split-Path -Parent $paths.BaseComposePath) -Paths $paths -ApiKey $ApiKey
                $remainingContainers = @($remainingOutput -split '\r?\n' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
                if ($remainingContainers.Count -ne 0) {
                    throw "cleanup verification found $($remainingContainers.Count) Compose container(s) still present"
                }
                Write-OpenMeterPoCFile -Path $paths.RunnerLogPath -AllowedRoot $paths.LocalRoot -Value "cleanup verified: no Compose containers remain" -ApiKey $ApiKey -Append
            }
            catch {
                $exitCode = 1
                $safeCleanupError = Protect-OpenMeterPoCText -Text $_.Exception.Message -ApiKey $ApiKey
                Write-OpenMeterPoCFile -Path $paths.RunnerLogPath -AllowedRoot $paths.LocalRoot -Value "CLEANUP FAILED: $safeCleanupError" -ApiKey $ApiKey -Append
                Write-Warning "OpenMeter PoC cleanup failed: $safeCleanupError"
            }
        }
        elseif ($composeConfigured -and $KeepEnvironment) {
            Write-OpenMeterPoCFile -Path $paths.RunnerLogPath -AllowedRoot $paths.LocalRoot -Value "environment retained by -KeepEnvironment" -ApiKey $ApiKey -Append
        }
    }

    return [int]$exitCode
}
