param(
    [int]$ApiPort = 8085,
    [int]$UiPort = 3000,
    [int]$LocalDbPort = 15432,
    [int]$LocalRedisPort = 16379,
    [string]$ConfigPath = "config/config-dev.yaml",
    [string]$LogLevel = "info",
    [switch]$SkipRedis,
    [switch]$IncludeTemporal,
    [int]$LocalTemporalPort = 7233,
    [string]$BypassAuthGate = ""
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
    param([int[]]$ListenPorts)

    foreach ($listenPort in $ListenPorts) {
        $processIds = @(Get-ListeningProcessIds -ListenPort $listenPort)
        foreach ($processId in $processIds) {
            $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
            if ($null -eq $process) {
                continue
            }

            Write-Host "Stopping existing process on port ${listenPort}: PID ${processId} (${process.ProcessName})" -ForegroundColor DarkYellow
            Stop-Process -Id $processId -Force
            try {
                $process.WaitForExit()
            } catch {
            }
        }
    }
}

function Wait-ForListeningPort {
    param(
        [int]$ListenPort,
        [int]$TimeoutSeconds = 40
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if ((@(Get-NetTCPConnection -State Listen -LocalPort $ListenPort -ErrorAction SilentlyContinue).Count) -gt 0) {
            return
        }
        Start-Sleep -Milliseconds 500
    }

    throw "Timed out waiting for local port ${ListenPort} to start listening"
}

$repoRoot = Get-RepoRoot
$runtimeDir = Join-Path $repoRoot ".local\tmp\listingkit-local-portforward"
$stdoutLog = Join-Path $runtimeDir "stdout.log"
$stderrLog = Join-Path $runtimeDir "stderr.log"
$pidFile = Join-Path $runtimeDir "portforward-wrapper.pid"
$portforwardScript = Join-Path $repoRoot "scripts\start-listingkit-local-portforward.ps1"
$apiScript = Join-Path $repoRoot "scripts\start-listingkit-local-api.ps1"
$uiScript = Join-Path $repoRoot "scripts\start-listingkit-local-ui.ps1"
$apiPidFile = Join-Path $repoRoot ".local\tmp\listingkit-local-api\product-listing-api-local.pid"
$uiPidFile = Join-Path $repoRoot "web\listingkit-ui\.local-dev\listingkit-ui-local.pid"

Ensure-Directory -Path $runtimeDir

$portsToStop = @($UiPort, $ApiPort, $LocalDbPort, $LocalRedisPort)
if ($IncludeTemporal) {
    $portsToStop += $LocalTemporalPort
}
Stop-ListeningProcesses -ListenPorts $portsToStop

if (Test-Path -LiteralPath $stdoutLog) { Remove-Item -LiteralPath $stdoutLog -Force }
if (Test-Path -LiteralPath $stderrLog) { Remove-Item -LiteralPath $stderrLog -Force }
if (Test-Path -LiteralPath $pidFile) { Remove-Item -LiteralPath $pidFile -Force }

$portforwardArgs = @(
    "-ExecutionPolicy", "Bypass",
    "-File", $portforwardScript,
    "-LocalDbPort", $LocalDbPort,
    "-RemoteDbPort", 5432
)
if ($SkipRedis) {
    $portforwardArgs += "-SkipRedis"
} else {
    $portforwardArgs += @("-LocalRedisPort", $LocalRedisPort, "-RemoteRedisPort", 6379)
}
if ($IncludeTemporal) {
    $portforwardArgs += @("-IncludeTemporal", "-LocalTemporalPort", $LocalTemporalPort, "-RemoteTemporalPort", 7233)
}

Write-Host "Starting local port-forward..." -ForegroundColor Cyan
$portforwardProcess = Start-Process `
    -FilePath "powershell" `
    -ArgumentList $portforwardArgs `
    -WorkingDirectory $repoRoot `
    -WindowStyle Hidden `
    -PassThru `
    -RedirectStandardOutput $stdoutLog `
    -RedirectStandardError $stderrLog

try {
    Wait-ForListeningPort -ListenPort $LocalDbPort -TimeoutSeconds 40
    if (-not $SkipRedis) {
        Wait-ForListeningPort -ListenPort $LocalRedisPort -TimeoutSeconds 40
    }
    if ($IncludeTemporal) {
        Wait-ForListeningPort -ListenPort $LocalTemporalPort -TimeoutSeconds 40
    }
} catch {
    if (-not $portforwardProcess.HasExited) {
        Stop-Process -Id $portforwardProcess.Id -Force
        try {
            $portforwardProcess.WaitForExit()
        } catch {
        }
    }
    throw
}

Set-Content -LiteralPath $pidFile -Value $portforwardProcess.Id -NoNewline

if ($IncludeTemporal) {
    $env:LISTINGKIT_TEMPORAL_ENABLED = "true"
    $env:LISTINGKIT_TEMPORAL_ADDRESS = "127.0.0.1:${LocalTemporalPort}"
}

Write-Host "Starting local API..." -ForegroundColor Cyan
& powershell -ExecutionPolicy Bypass -File $apiScript -Port $ApiPort -ConfigPath $ConfigPath -LogLevel $LogLevel
if ($LASTEXITCODE -ne 0) {
    throw "Failed to start local API"
}

Write-Host "Starting local UI..." -ForegroundColor Cyan
$uiArgs = @(
    "-ExecutionPolicy", "Bypass",
    "-File", $uiScript,
    "-Port", $UiPort,
    "-ApiBase", "http://localhost:${ApiPort}/api/v1/listing-kits",
    "-ServiceApiBase", "http://localhost:${ApiPort}/api/v1"
)
if (-not [string]::IsNullOrWhiteSpace($BypassAuthGate)) {
    $uiArgs += @("-BypassAuthGate", $BypassAuthGate)
}
& powershell @uiArgs
if ($LASTEXITCODE -ne 0) {
    throw "Failed to start local UI"
}

$apiPid = if (Test-Path -LiteralPath $apiPidFile) { (Get-Content -LiteralPath $apiPidFile -Raw).Trim() } else { "" }
$uiPid = if (Test-Path -LiteralPath $uiPidFile) { (Get-Content -LiteralPath $uiPidFile -Raw).Trim() } else { "" }
$apiHealth = Invoke-WebRequest -Uri "http://127.0.0.1:${ApiPort}/health" -UseBasicParsing -TimeoutSec 5

try {
    $uiResponse = Invoke-WebRequest -Uri "http://127.0.0.1:${UiPort}" -MaximumRedirection 0 -UseBasicParsing -TimeoutSec 5
    $uiStatus = [int]$uiResponse.StatusCode
} catch {
    if ($_.Exception.Response) {
        $uiStatus = [int]$_.Exception.Response.StatusCode
    } else {
        throw
    }
}

Write-Host ""
Write-Host "Local ListingKit dev stack is ready." -ForegroundColor Green
Write-Host "  UI: http://localhost:${UiPort} (status ${uiStatus})"
Write-Host "  API: http://localhost:${ApiPort}/health (status $($apiHealth.StatusCode))"
Write-Host "  Port-forward PID: $($portforwardProcess.Id)"
if (-not [string]::IsNullOrWhiteSpace($apiPid)) {
    Write-Host "  API PID: $apiPid"
}
if (-not [string]::IsNullOrWhiteSpace($uiPid)) {
    Write-Host "  UI PID: $uiPid"
}
Write-Host "  Port-forward logs: $stdoutLog / $stderrLog"
if ($IncludeTemporal) {
    Write-Host "  Temporal: 127.0.0.1:${LocalTemporalPort}"
}
Write-Host "  API logs: $repoRoot\.local\tmp\listingkit-local-api\logs\stdout.log / $repoRoot\.local\tmp\listingkit-local-api\logs\stderr.log"
Write-Host "  UI logs: $repoRoot\web\listingkit-ui\.local-dev\ui-stdout.log / $repoRoot\web\listingkit-ui\.local-dev\ui-stderr.log"
Write-Host ""
Write-Host "Stop command:" -ForegroundColor Yellow
$stopPids = @($portforwardProcess.Id)
if (-not [string]::IsNullOrWhiteSpace($apiPid)) { $stopPids += $apiPid }
if (-not [string]::IsNullOrWhiteSpace($uiPid)) { $stopPids += $uiPid }
Write-Host "  Stop-Process -Id $($stopPids -join ',')"
