param(
    [int]$Port = 8085,
    [string]$ConfigPath = "config/config-dev.yaml",
    [string]$LogLevel = "info",
    [Alias("RequireSettingsHealth")]
    [switch]$RequireReadiness
)

$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    $scriptDir = $PSScriptRoot
    return (Resolve-Path (Join-Path $scriptDir "..")).Path
}

function Ensure-Directory {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Get-K8sEnvValue {
    param(
        [string]$Namespace,
        [string]$Kind,
        [string]$Name,
        [string]$Key,
        [switch]$DecodeBase64
    )

    try {
        $json = & kubectl -n $Namespace get $Kind $Name -o json 2>$null
        if (-not $json) {
            return $null
        }
        $object = $json | ConvertFrom-Json
        $rawValue = $object.data.$Key
        if (-not $rawValue) {
            return $null
        }
        if ($DecodeBase64) {
            return [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($rawValue))
        }
        return $rawValue
    } catch {
        return $null
    }
}

function Set-EnvValue {
    param(
        [string]$Name,
        [string]$Value,
        [switch]$Overwrite
    )

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return
    }
    if (-not $Overwrite -and -not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($Name))) {
        return
    }
    [Environment]::SetEnvironmentVariable($Name, $Value)
}

function Import-DotEnvFile {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    $loadedCount = 0
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ([string]::IsNullOrWhiteSpace($trimmed) -or $trimmed.StartsWith("#")) {
            continue
        }

        $match = [regex]::Match($trimmed, "^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$")
        if (-not $match.Success) {
            continue
        }

        $name = $match.Groups[1].Value
        $value = $match.Groups[2].Value.Trim()
        if (
            ($value.Length -ge 2) -and
            (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))
        ) {
            $value = $value.Substring(1, $value.Length - 2)
        }

        [Environment]::SetEnvironmentVariable($name, $value, "Process")
        $loadedCount++
    }

    if ($loadedCount -gt 0) {
        Write-Host "Loaded local .env values for local API startup." -ForegroundColor DarkGreen
    }
}

function Initialize-ListingKitObjectStorageEnvFromK8s {
    $namespace = "task-processor"
    $configName = "listingkit-workbench-config"
    $secretName = "listingkit-workbench-secret"

    $provider = Get-K8sEnvValue -Namespace $namespace -Kind "configmap" -Name $configName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PROVIDER"
    $publicBase = Get-K8sEnvValue -Namespace $namespace -Kind "configmap" -Name $configName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PUBLICBASE"
    $bucket = Get-K8sEnvValue -Namespace $namespace -Kind "configmap" -Name $configName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_BUCKET"
    $region = Get-K8sEnvValue -Namespace $namespace -Kind "configmap" -Name $configName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_REGION"
    $endpoint = Get-K8sEnvValue -Namespace $namespace -Kind "configmap" -Name $configName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ENDPOINT"
    $usePathStyle = Get-K8sEnvValue -Namespace $namespace -Kind "configmap" -Name $configName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_USEPATHSTYLE"
    $accessKey = Get-K8sEnvValue -Namespace $namespace -Kind "secret" -Name $secretName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ACCESSKEYID" -DecodeBase64
    $secretKey = Get-K8sEnvValue -Namespace $namespace -Kind "secret" -Name $secretName -Key "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_SECRETACCESSKEY" -DecodeBase64

    $ready =
        -not [string]::IsNullOrWhiteSpace($provider) -and
        -not [string]::IsNullOrWhiteSpace($bucket) -and
        -not [string]::IsNullOrWhiteSpace($endpoint) -and
        -not [string]::IsNullOrWhiteSpace($accessKey) -and
        -not [string]::IsNullOrWhiteSpace($secretKey)

    if (-not $ready) {
        Write-Host "K8s object storage env is incomplete; keeping local productimage publisher config." -ForegroundColor DarkYellow
        return
    }

    # The local API must use the same object store that persisted ListingKit uploads.
    # Import-DotEnvFile runs first, so stale local values must not shadow the live
    # ListingKit config and cause GetObject failures to look like missing uploads.
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PROVIDER" -Value $provider -Overwrite
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_PUBLICBASE" -Value $publicBase -Overwrite
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_BUCKET" -Value $bucket -Overwrite
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_REGION" -Value $region -Overwrite
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ENDPOINT" -Value $endpoint -Overwrite
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ACCESSKEYID" -Value $accessKey -Overwrite
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_SECRETACCESSKEY" -Value $secretKey -Overwrite
    Set-EnvValue -Name "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_USEPATHSTYLE" -Value $usePathStyle -Overwrite

    Write-Host "Loaded ListingKit object storage env from k8s config/secret for local API." -ForegroundColor DarkGreen
}

function Get-ListeningProcessIds {
    param([int]$ListenPort)

    $connections = @(Get-NetTCPConnection -State Listen -LocalPort $ListenPort -ErrorAction SilentlyContinue)
    if ($connections.Count -eq 0) {
        return @()
    }

    return $connections |
        Select-Object -ExpandProperty OwningProcess -Unique |
        Where-Object { $_ -gt 0 }
}

function Stop-ListeningProcesses {
    param([int]$ListenPort)

    $processIds = @(Get-ListeningProcessIds -ListenPort $ListenPort)
    foreach ($processId in $processIds) {
        $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
        if ($null -eq $process) {
            continue
        }

        Write-Host "Stopping existing process on port ${ListenPort}: PID ${processId} (${process.ProcessName})" -ForegroundColor DarkYellow
        Stop-Process -Id $processId -Force
        $process.WaitForExit()
    }
}

function Wait-ForApiReady {
    param(
        [string]$HealthURL,
        [string]$ReadinessURL,
        [switch]$RequireReadiness,
        [string]$StdoutLogPath,
        [int]$TimeoutSeconds = 180
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $StdoutLogPath) {
            $stdout = Get-Content -LiteralPath $StdoutLogPath -Raw -ErrorAction SilentlyContinue
            if ($stdout -match "API service listening on port") {
                return
            }
        }

        try {
            $response = Invoke-WebRequest -Uri $HealthURL -UseBasicParsing -TimeoutSec 3
            if ($response.StatusCode -eq 200) {
                if (-not $RequireReadiness) {
                    return
                }

                $readiness = Invoke-WebRequest -Uri $ReadinessURL -UseBasicParsing -TimeoutSec 3
                if ($readiness.StatusCode -eq 200) {
                    return
                }
            }
        } catch {
        }

        Start-Sleep -Milliseconds 500
    }

    if ($RequireReadiness) {
        throw "Timed out waiting for ListingKit readiness: $ReadinessURL"
    }
    throw "Timed out waiting for API HTTP listener: $HealthURL"
}

$repoRoot = Get-RepoRoot
$runtimeDir = Join-Path $repoRoot ".local\tmp\listingkit-local-api"
$logDir = Join-Path $runtimeDir "logs"
$binPath = Join-Path $runtimeDir "product-listing-api-local.exe"
$stdoutLog = Join-Path $logDir "stdout.log"
$stderrLog = Join-Path $logDir "stderr.log"
$pidFile = Join-Path $runtimeDir "product-listing-api-local.pid"
$healthURL = "http://127.0.0.1:${Port}/health"
$readinessURL = "http://127.0.0.1:${Port}/readyz"

Ensure-Directory -Path $runtimeDir
Ensure-Directory -Path $logDir

Stop-ListeningProcesses -ListenPort $Port

if (Test-Path -LiteralPath $stdoutLog) { Remove-Item -LiteralPath $stdoutLog -Force }
if (Test-Path -LiteralPath $stderrLog) { Remove-Item -LiteralPath $stderrLog -Force }
if (Test-Path -LiteralPath $pidFile) { Remove-Item -LiteralPath $pidFile -Force }

$env:TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY = "1"
$env:TASK_PROCESSOR_BROWSER_PROXYSERVER = ""
$env:TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE = "false"
$env:TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE = "false"
$env:LISTINGKIT_TEMPORAL_TASK_QUEUE = "listingkit-local-$env:COMPUTERNAME-$Port"
Import-DotEnvFile -Path (Join-Path $repoRoot ".env")
Initialize-ListingKitObjectStorageEnvFromK8s

Write-Host "Building local product-listing-api..." -ForegroundColor Cyan
& go build -o $binPath .\cmd\product-listing-api
if ($LASTEXITCODE -ne 0) {
    throw "go build failed"
}

Write-Host "Starting local product-listing-api on port ${Port}..." -ForegroundColor Cyan
$process = Start-Process `
    -FilePath $binPath `
    -ArgumentList @("-config", $ConfigPath, "-port", $Port.ToString(), "-log-level", $LogLevel) `
    -WorkingDirectory $repoRoot `
    -WindowStyle Hidden `
    -PassThru `
    -RedirectStandardOutput $stdoutLog `
    -RedirectStandardError $stderrLog

try {
    Wait-ForApiReady `
        -HealthURL $healthURL `
        -ReadinessURL $readinessURL `
        -RequireReadiness:$RequireReadiness `
        -StdoutLogPath $stdoutLog `
        -TimeoutSeconds 180
} catch {
    if (-not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit()
    }

    Write-Host ""
    Write-Host "API failed to become ready. Recent stdout:" -ForegroundColor Red
    if (Test-Path -LiteralPath $stdoutLog) {
        Get-Content -LiteralPath $stdoutLog -Tail 50
    }
    Write-Host ""
    Write-Host "API failed to become ready. Recent stderr:" -ForegroundColor Red
    if (Test-Path -LiteralPath $stderrLog) {
        Get-Content -LiteralPath $stderrLog -Tail 50
    }
    throw
}

Set-Content -LiteralPath $pidFile -Value $process.Id -NoNewline

Write-Host ""
if ($RequireReadiness) {
    Write-Host "Local API and ListingKit readiness are ready." -ForegroundColor Green
} else {
    Write-Host "Local API HTTP listener is ready." -ForegroundColor Green
}
Write-Host "  URL: ${healthURL}"
Write-Host "  PID: $($process.Id)"
Write-Host "  stdout: $stdoutLog"
Write-Host "  stderr: $stderrLog"
Write-Host "  shein proxy: ignored for this local process (TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY=1)"
Write-Host "  browser proxy: cleared for this local process (TASK_PROCESSOR_BROWSER_PROXYSERVER='')"
Write-Host "  api runtime auto-migrate: disabled for this local process (TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE=false)"
Write-Host "  listingkit auto-migrate: disabled for this local process (TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE=false)"
Write-Host "  listingkit temporal task queue: $env:LISTINGKIT_TEMPORAL_TASK_QUEUE"
if (-not $RequireReadiness) {
    Write-Host "  ListingKit readiness: not verified (use -RequireReadiness after DB/Redis/Temporal port-forward is ready)" -ForegroundColor DarkYellow
}
Write-Host ""
Write-Host "Stop command:" -ForegroundColor Yellow
Write-Host "  Stop-Process -Id $($process.Id)"
