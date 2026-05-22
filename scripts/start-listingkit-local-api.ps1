param(
    [int]$Port = 8085,
    [string]$ConfigPath = "config/config-dev.yaml",
    [string]$LogLevel = "info"
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
                return
            }
        } catch {
        }

        Start-Sleep -Milliseconds 500
    }

    throw "Timed out waiting for API readiness: $HealthURL"
}

$repoRoot = Get-RepoRoot
$runtimeDir = Join-Path $repoRoot ".local\tmp\listingkit-local-api"
$logDir = Join-Path $runtimeDir "logs"
$binPath = Join-Path $runtimeDir "product-listing-api-local.exe"
$stdoutLog = Join-Path $logDir "stdout.log"
$stderrLog = Join-Path $logDir "stderr.log"
$pidFile = Join-Path $runtimeDir "product-listing-api-local.pid"
$healthURL = "http://127.0.0.1:${Port}/health"

Ensure-Directory -Path $runtimeDir
Ensure-Directory -Path $logDir

Stop-ListeningProcesses -ListenPort $Port

if (Test-Path -LiteralPath $stdoutLog) { Remove-Item -LiteralPath $stdoutLog -Force }
if (Test-Path -LiteralPath $stderrLog) { Remove-Item -LiteralPath $stderrLog -Force }
if (Test-Path -LiteralPath $pidFile) { Remove-Item -LiteralPath $pidFile -Force }

$env:TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY = "1"

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
    Wait-ForApiReady -HealthURL $healthURL -StdoutLogPath $stdoutLog -TimeoutSeconds 180
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
Write-Host "Local API is ready." -ForegroundColor Green
Write-Host "  URL: ${healthURL}"
Write-Host "  PID: $($process.Id)"
Write-Host "  stdout: $stdoutLog"
Write-Host "  stderr: $stderrLog"
Write-Host "  shein proxy: ignored for this local process (TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY=1)"
Write-Host ""
Write-Host "Stop command:" -ForegroundColor Yellow
Write-Host "  Stop-Process -Id $($process.Id)"
