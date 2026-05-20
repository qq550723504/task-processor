param(
    [string]$RedisNamespace = "yudao-cloud",
    [string]$RedisService = "redis-master",
    [int]$LocalPort = 16379,
    [int]$RemotePort = 6379,
    [int]$RedisDB = 9,
    [string]$RedisPassword = "",
    [string]$RedisKey = "sds:auth:global",
    [string]$WorkDir = "D:\code\task-processor\tools\cloakbrowser-poc"
)

$ErrorActionPreference = "Stop"

function Wait-ForTcpPort {
    param(
        [string]$Address,
        [int]$Port,
        [int]$TimeoutSeconds = 20
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $client = $null
        try {
            $client = [System.Net.Sockets.TcpClient]::new()
            $task = $client.ConnectAsync($Host, $Port)
            if ($task.Wait(1000) -and $client.Connected) {
                return
            }
        } catch {
        } finally {
            if ($client) {
                $client.Dispose()
            }
        }
        Start-Sleep -Milliseconds 500
    }

    throw "Timed out waiting for TCP port ${Address}:${Port}"
}

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

    throw "Timed out waiting for kubectl port-forward readiness"
}

if (-not (Test-Path -LiteralPath $WorkDir)) {
    throw "WorkDir not found: $WorkDir"
}

$stdoutLog = Join-Path $env:TEMP "sds-redis-portforward.stdout.log"
$stderrLog = Join-Path $env:TEMP "sds-redis-portforward.stderr.log"
if (Test-Path $stdoutLog) { Remove-Item -LiteralPath $stdoutLog -Force }
if (Test-Path $stderrLog) { Remove-Item -LiteralPath $stderrLog -Force }

$portForwardArgs = @(
    "-n", $RedisNamespace,
    "port-forward",
    "svc/$RedisService",
    "${LocalPort}:${RemotePort}"
)

$portForward = $null
try {
    Write-Host "Starting kubectl port-forward to ${RedisNamespace}/${RedisService} on localhost:${LocalPort} ..."
    $portForward = Start-Process `
        -FilePath "kubectl" `
        -ArgumentList $portForwardArgs `
        -WindowStyle Hidden `
        -PassThru `
        -RedirectStandardOutput $stdoutLog `
        -RedirectStandardError $stderrLog

    Wait-ForPortForwardReady -LogPath $stdoutLog -TimeoutSeconds 20

    Write-Host "Port-forward is ready. Running SDS refresh script ..."
    $env:TASK_PROCESSOR_SDS_REDIS_HOST = "127.0.0.1"
    $env:TASK_PROCESSOR_SDS_REDIS_PORT = "$LocalPort"
    $env:TASK_PROCESSOR_SDS_REDIS_DB = "$RedisDB"
    $env:TASK_PROCESSOR_SDS_REDIS_KEY = $RedisKey
    if ([string]::IsNullOrWhiteSpace($RedisPassword)) {
        Remove-Item Env:TASK_PROCESSOR_SDS_REDIS_PASSWORD -ErrorAction SilentlyContinue
    } else {
        $env:TASK_PROCESSOR_SDS_REDIS_PASSWORD = $RedisPassword
    }

    Push-Location $WorkDir
    try {
        npm run sds-login:redis
    } finally {
        Pop-Location
    }
} finally {
    if ($portForward -and -not $portForward.HasExited) {
        Write-Host "Stopping kubectl port-forward ..."
        Stop-Process -Id $portForward.Id -Force
        $portForward.WaitForExit()
    }
}
