param(
    [string]$DbNamespace = "yudao-cloud",
    [string]$DbService = "postgresql-v18",
    [int]$LocalDbPort = 15432,
    [int]$RemoteDbPort = 5432,
    [string]$DbUser = "root",
    [string]$DbName = "ruoyi-vue-pro",
    [switch]$IncludeRedis,
    [string]$RedisNamespace = "yudao-cloud",
    [string]$RedisService = "redis-master",
    [int]$LocalRedisPort = 16379,
    [int]$RemoteRedisPort = 6379
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
    }
}

$forwards = @()

try {
    $forwards += Start-PortForwardProcess `
        -Namespace $DbNamespace `
        -Service $DbService `
        -LocalPort $LocalDbPort `
        -RemotePort $RemoteDbPort `
        -LogPrefix "listingkit-db-portforward"

    if ($IncludeRedis) {
        $forwards += Start-PortForwardProcess `
            -Namespace $RedisNamespace `
            -Service $RedisService `
            -LocalPort $LocalRedisPort `
            -RemotePort $RemoteRedisPort `
            -LogPrefix "listingkit-redis-portforward"
    }

    Write-Host ""
    Write-Host "Port-forward is ready." -ForegroundColor Green
    Write-Host ""
    Write-Host "Suggested env overrides for local product-listing-api:" -ForegroundColor Yellow
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_HOST='127.0.0.1'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_PORT='${LocalDbPort}'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_USER='${DbUser}'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_NAME='${DbName}'"
    Write-Host "  `$env:TASK_PROCESSOR_DATABASE_PASSWORD='<fill-real-password>'"
    if ($IncludeRedis) {
        Write-Host "  `$env:TASK_PROCESSOR_REDIS_HOST='127.0.0.1'"
        Write-Host "  `$env:TASK_PROCESSOR_REDIS_PORT='${LocalRedisPort}'"
        Write-Host "  `$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST='127.0.0.1'"
        Write-Host "  `$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT='${LocalRedisPort}'"
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
    Write-Host "This window keeps the port-forward alive. Press Ctrl+C when you are done." -ForegroundColor Yellow

    while ($true) {
        Start-Sleep -Seconds 2
        foreach ($forward in $forwards) {
            if ($forward.Process.HasExited) {
                $stderr = ""
                if (Test-Path -LiteralPath $forward.StderrLog) {
                    $stderr = Get-Content -LiteralPath $forward.StderrLog -Raw -ErrorAction SilentlyContinue
                }
                throw "kubectl port-forward exited unexpectedly. ${stderr}"
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
