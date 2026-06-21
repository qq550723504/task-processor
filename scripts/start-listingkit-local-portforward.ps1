param(
    [string]$DbNamespace = "yudao-cloud",
    [string]$DbService = "postgresql-v18",
    [int]$LocalDbPort = 15432,
    [int]$RemoteDbPort = 5432,
    [string]$DbUser = "postgres",
    [string]$DbName = "ruoyi-vue-pro",
    [switch]$SkipRedis,
    [string]$RedisNamespace = "yudao-cloud",
    [string]$RedisService = "redis-master",
    [int]$LocalRedisPort = 16379,
    [int]$RemoteRedisPort = 6379,
    [switch]$IncludeTemporal,
    [string]$TemporalNamespace = "temporal",
    [string]$TemporalService = "temporal-frontend",
    [int]$LocalTemporalPort = 7233,
    [int]$RemoteTemporalPort = 7233
)

$ErrorActionPreference = "Stop"
$script:PortForwardSessionId = "{0}-{1}" -f (Get-Date -Format "yyyyMMdd-HHmmss"), $PID

function Wait-ForPortForwardReady {
    param(
        [string]$LogPath,
        [int]$TimeoutSeconds = 20
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $LogPath) {
            $content = Get-Content -LiteralPath $LogPath -Raw -ErrorAction SilentlyContinue
            if ($content -match "Forwarding from 127\.0\.0\.1") {
                return
            }
        }
        Start-Sleep -Milliseconds 500
    }

    throw "Timed out waiting for kubectl port-forward readiness: $LogPath"
}

function Stop-ExistingPortForwardProcess {
    param(
        [string]$Namespace,
        [string]$Service,
        [int]$LocalPort,
        [int]$RemotePort
    )

    $portForwardPattern = [regex]::Escape("svc/$Service")
    $portMappingPattern = [regex]::Escape("${LocalPort}:${RemotePort}")
    $namespacePattern = "(^|\s)-n\s+$([regex]::Escape($Namespace))(\s|$)"

    $existingProcesses = Get-CimInstance Win32_Process |
        Where-Object {
            $_.Name -eq "kubectl.exe" -and
            $_.CommandLine -match "port-forward" -and
            $_.CommandLine -match $portForwardPattern -and
            $_.CommandLine -match $portMappingPattern -and
            $_.CommandLine -match $namespacePattern
        }

    foreach ($existingProcess in $existingProcesses) {
        Write-Host "Stopping existing kubectl port-forward PID $($existingProcess.ProcessId) for ${Namespace}/${Service} on localhost:${LocalPort} ..." -ForegroundColor DarkYellow
        Stop-Process -Id $existingProcess.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

function Start-PortForwardProcess {
    param(
        [string]$Namespace,
        [string]$Service,
        [int]$LocalPort,
        [int]$RemotePort,
        [string]$LogPrefix
    )

    $stdoutLog = Join-Path $env:TEMP ("{0}.{1}.stdout.log" -f $LogPrefix, $script:PortForwardSessionId)
    $stderrLog = Join-Path $env:TEMP ("{0}.{1}.stderr.log" -f $LogPrefix, $script:PortForwardSessionId)

    $args = @(
        "-n", $Namespace,
        "port-forward",
        "svc/$Service",
        "${LocalPort}:${RemotePort}"
    )

    Stop-ExistingPortForwardProcess -Namespace $Namespace -Service $Service -LocalPort $LocalPort -RemotePort $RemotePort

    Write-Host "Starting kubectl port-forward to ${Namespace}/${Service} on localhost:${LocalPort} ..." -ForegroundColor Cyan
    $process = Start-Process `
        -FilePath "kubectl" `
        -ArgumentList $args `
        -WindowStyle Hidden `
        -PassThru `
        -RedirectStandardOutput $stdoutLog `
        -RedirectStandardError $stderrLog

    Wait-ForPortForwardReady -LogPath $stdoutLog -TimeoutSeconds 20

    [PSCustomObject]@{
        Process   = $process
        StdoutLog = $stdoutLog
        StderrLog = $stderrLog
        Service   = $Service
        LocalPort = $LocalPort
    }
}

$forwards = @()

try {
    Write-Host "Fixed local ports: DB=15432, Redis=16379, API=8085, UI=3000. Use one set only." -ForegroundColor Yellow
    $forwards += Start-PortForwardProcess `
        -Namespace $DbNamespace `
        -Service $DbService `
        -LocalPort $LocalDbPort `
        -RemotePort $RemoteDbPort `
        -LogPrefix "listingkit-db-portforward"

    if (-not $SkipRedis) {
        $forwards += Start-PortForwardProcess `
            -Namespace $RedisNamespace `
            -Service $RedisService `
            -LocalPort $LocalRedisPort `
            -RemotePort $RemoteRedisPort `
            -LogPrefix "listingkit-redis-portforward"
    }

    if ($IncludeTemporal) {
        $forwards += Start-PortForwardProcess `
            -Namespace $TemporalNamespace `
            -Service $TemporalService `
            -LocalPort $LocalTemporalPort `
            -RemotePort $RemoteTemporalPort `
            -LogPrefix "listingkit-temporal-portforward"
    }

    Write-Host ""
    Write-Host "Port-forward is ready." -ForegroundColor Green
    Write-Host "Port-forward session id: $script:PortForwardSessionId" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "Suggested env overrides for local product-listing-api:" -ForegroundColor Yellow
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_HOST='127.0.0.1'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_PORT='${LocalDbPort}'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_USER='${DbUser}'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_NAME='${DbName}'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_PASSWORD='<fill-real-password>'"
    Write-Host "  `$env:TASK_PROCESSOR_REDIS_HOST='127.0.0.1'"
    Write-Host "  `$env:TASK_PROCESSOR_REDIS_PORT='${LocalRedisPort}'"
    Write-Host "  `$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST='127.0.0.1'"
    Write-Host "  `$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT='${LocalRedisPort}'"
    if ($IncludeTemporal) {
        Write-Host "  `$env:LISTINGKIT_TEMPORAL_ENABLED='true'"
        Write-Host "  `$env:LISTINGKIT_TEMPORAL_ADDRESS='127.0.0.1:${LocalTemporalPort}'"
    }
    Write-Host "  go run ./cmd/product-listing-api -config config/config-dev.yaml -port 8085 -log-level info"
    Write-Host ""
    Write-Host "Suggested env overrides for local listingkit-ui:" -ForegroundColor Yellow
    Write-Host "  Set-Location web/listingkit-ui"
    Write-Host "  `$env:LISTINGKIT_API_BASE='http://localhost:8085/api/v1/listing-kits'"
    Write-Host "  `$env:LISTINGKIT_SERVICE_API_BASE='http://localhost:8085/api/v1'"
    Write-Host "  `$env:LISTINGKIT_UI_BYPASS_AUTH_GATE='1'"
    Write-Host "  npm run dev"
    Write-Host ""
    if ($SkipRedis) {
        Write-Host "Redis port-forward is skipped by -SkipRedis. Use this only for static or pure read-only UI checks." -ForegroundColor DarkYellow
        Write-Host ""
    }
    if ($IncludeTemporal) {
        Write-Host "Remote Temporal is forwarded to 127.0.0.1:${LocalTemporalPort}." -ForegroundColor DarkYellow
        Write-Host ""
    }
    Write-Host "This window keeps the port-forward alive. Press Ctrl+C when you are done." -ForegroundColor Yellow

    while ($true) {
        Start-Sleep -Seconds 2
        foreach ($forward in $forwards) {
            if ($forward.Process.HasExited) {
                $stderr = ""
                if (Test-Path -LiteralPath $forward.StderrLog) {
                    $stderr = Get-Content -LiteralPath $forward.StderrLog -Raw -ErrorAction SilentlyContinue
                }
                throw "kubectl port-forward for $($forward.Service) exited unexpectedly. ${stderr}"
            }
        }
    }
} finally {
    foreach ($forward in $forwards) {
        if ($forward.Process -and -not $forward.Process.HasExited) {
            Write-Host "Stopping kubectl port-forward PID $($forward.Process.Id) ..." -ForegroundColor DarkYellow
            Stop-Process -Id $forward.Process.Id -Force
            $forward.Process.WaitForExit()
        }
    }
}
