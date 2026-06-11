param(
    [string]$DbNamespace = "yudao-cloud",
    [string]$DbService = "postgresql-v18",
    [int]$LocalDbPort = 15432,
    [int]$RemoteDbPort = 5432,
    [switch]$SkipDatabase,
    [string]$RedisNamespace = "yudao-cloud",
    [string]$RedisService = "redis-master",
    [int]$LocalRedisPort = 16379,
    [int]$RemoteRedisPort = 6379,
    [switch]$SkipRedis,
    [string]$Namespace = "yudao-cloud",
    [string]$RabbitMQService = "rabbitmq",
    [int]$LocalAMQPPort = 15673,
    [int]$RemoteAMQPPort = 5672,
    [switch]$EnableManagement,
    [string]$ManagementService = "rabbitmq-management",
    [int]$LocalManagementPort = 15672,
    [int]$RemoteManagementPort = 15672
)

$ErrorActionPreference = "Stop"

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

function Start-PortForwardProcess {
    param(
        [string]$Namespace,
        [string]$Service,
        [int]$LocalPort,
        [int]$RemotePort,
        [string]$LogPrefix
    )

    $stdoutLog = Join-Path $env:TEMP "$LogPrefix.stdout.log"
    $stderrLog = Join-Path $env:TEMP "$LogPrefix.stderr.log"
    if (Test-Path -LiteralPath $stdoutLog) { Remove-Item -LiteralPath $stdoutLog -Force }
    if (Test-Path -LiteralPath $stderrLog) { Remove-Item -LiteralPath $stderrLog -Force }

    $args = @(
        "-n", $Namespace,
        "port-forward",
        "svc/$Service",
        "${LocalPort}:${RemotePort}"
    )

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
    if (-not $SkipDatabase) {
        $forwards += Start-PortForwardProcess `
            -Namespace $DbNamespace `
            -Service $DbService `
            -LocalPort $LocalDbPort `
            -RemotePort $RemoteDbPort `
            -LogPrefix "shein-postgres-portforward"
    }

    if (-not $SkipRedis) {
        $forwards += Start-PortForwardProcess `
            -Namespace $RedisNamespace `
            -Service $RedisService `
            -LocalPort $LocalRedisPort `
            -RemotePort $RemoteRedisPort `
            -LogPrefix "shein-redis-portforward"
    }

    $forwards += Start-PortForwardProcess `
        -Namespace $Namespace `
        -Service $RabbitMQService `
        -LocalPort $LocalAMQPPort `
        -RemotePort $RemoteAMQPPort `
        -LogPrefix "shein-rabbitmq-amqp-portforward"

    if ($EnableManagement) {
        $forwards += Start-PortForwardProcess `
            -Namespace $Namespace `
            -Service $ManagementService `
            -LocalPort $LocalManagementPort `
            -RemotePort $RemoteManagementPort `
            -LogPrefix "shein-rabbitmq-management-portforward"
    }

    Write-Host ""
    Write-Host "Local shein dependencies port-forward is ready." -ForegroundColor Green
    Write-Host ""
    Write-Host "Suggested env override for local shein-listing:" -ForegroundColor Yellow
    if (-not $SkipDatabase) {
        Write-Host "  `$env:TASK_PROCESSOR_DATABASE_HOST='127.0.0.1'"
        Write-Host "  `$env:TASK_PROCESSOR_DATABASE_PORT='${LocalDbPort}'"
    }
    if (-not $SkipRedis) {
        Write-Host "  `$env:TASK_PROCESSOR_REDIS_HOST='127.0.0.1'"
        Write-Host "  `$env:TASK_PROCESSOR_REDIS_PORT='${LocalRedisPort}'"
        Write-Host "  `$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST='127.0.0.1'"
        Write-Host "  `$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT='${LocalRedisPort}'"
    }
    Write-Host "  `$env:TASK_PROCESSOR_RABBITMQ_URL='amqp://admin:RabbitMQ%402026%23Prod@127.0.0.1:${LocalAMQPPort}/'"
    Write-Host "  `$env:TASK_PROCESSOR_MANAGEMENT_STORE_IDS='968'"
    Write-Host "  go run ./cmd/shein-listing -config config/config-dev.yaml -log-level debug"
    if ($EnableManagement) {
        Write-Host ""
        Write-Host "RabbitMQ management UI: http://127.0.0.1:${LocalManagementPort}" -ForegroundColor Yellow
    }
    Write-Host ""
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
