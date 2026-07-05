param(
    [int]$Port = 3000,
    [string]$ApiBase = "http://localhost:8085/api/v1/listing-kits",
    [string]$ServiceApiBase = "http://localhost:8085/api/v1",
    [string]$BypassAuthGate = "1"
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

function Remove-FileIfExists {
    param(
        [string]$Path,
        [int]$TimeoutSeconds = 10
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ($true) {
        try {
            Remove-Item -LiteralPath $Path -Force
            return
        } catch {
            if ((Get-Date) -ge $deadline) {
                throw
            }
            Start-Sleep -Milliseconds 250
        }
    }
}

function Set-EnvIfMissing {
    param(
        [string]$Name,
        [string]$Value
    )

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return
    }
    if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($Name))) {
        return
    }
    [Environment]::SetEnvironmentVariable($Name, $Value)
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
        try {
            $process.WaitForExit()
        } catch {
        }
    }
}

function Wait-ForUiReady {
    param(
        [string]$RootUrl,
        [string]$StdoutLogPath,
        [int]$TimeoutSeconds = 180
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $StdoutLogPath) {
            $stdout = Get-Content -LiteralPath $StdoutLogPath -Raw -ErrorAction SilentlyContinue
            if ($stdout -match "Ready in" -or $stdout -match "Local:\s+http://") {
                return
            }
        }

        try {
            $response = Invoke-WebRequest -Uri $RootUrl -MaximumRedirection 0 -UseBasicParsing -TimeoutSec 3
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 400) {
                return
            }
        } catch {
            if ($_.Exception.Response) {
                $statusCode = [int]$_.Exception.Response.StatusCode
                if ($statusCode -in 200, 302, 307, 308) {
                    return
                }
            }
        }

        Start-Sleep -Milliseconds 500
    }

    throw "Timed out waiting for UI readiness: $RootUrl"
}

$repoRoot = Get-RepoRoot
$uiRoot = Join-Path $repoRoot "web\listingkit-ui"
$runtimeDir = Join-Path $uiRoot ".local-dev"
$stdoutLog = Join-Path $runtimeDir "ui-stdout.log"
$stderrLog = Join-Path $runtimeDir "ui-stderr.log"
$pidFile = Join-Path $runtimeDir "listingkit-ui-local.pid"
$rootUrl = "http://127.0.0.1:${Port}"
$nextScript = Join-Path $uiRoot "node_modules\.bin\next.ps1"

Ensure-Directory -Path $runtimeDir

if (-not (Test-Path -LiteralPath $nextScript)) {
    throw "Next.js launcher not found: $nextScript. Run npm install in web/listingkit-ui first."
}

Stop-ListeningProcesses -ListenPort $Port

Remove-FileIfExists -Path $stdoutLog
Remove-FileIfExists -Path $stderrLog
Remove-FileIfExists -Path $pidFile

Set-EnvIfMissing -Name "LISTINGKIT_API_BASE" -Value $ApiBase
Set-EnvIfMissing -Name "LISTINGKIT_SERVICE_API_BASE" -Value $ServiceApiBase
if (-not [string]::IsNullOrWhiteSpace($BypassAuthGate)) {
    Set-EnvIfMissing -Name "LISTINGKIT_UI_BYPASS_AUTH_GATE" -Value $BypassAuthGate
}

$command = @"
`$env:LISTINGKIT_API_BASE = '$ApiBase'
`$env:LISTINGKIT_SERVICE_API_BASE = '$ServiceApiBase'
if ('${BypassAuthGate}' -ne '') { `$env:LISTINGKIT_UI_BYPASS_AUTH_GATE = '${BypassAuthGate}' }
& '$nextScript' dev -p $Port
"@

Write-Host "Starting local listingkit-ui on port ${Port}..." -ForegroundColor Cyan
$process = Start-Process `
    -FilePath "powershell" `
    -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $command) `
    -WorkingDirectory $uiRoot `
    -WindowStyle Hidden `
    -PassThru `
    -RedirectStandardOutput $stdoutLog `
    -RedirectStandardError $stderrLog

try {
    Wait-ForUiReady -RootUrl $rootUrl -StdoutLogPath $stdoutLog -TimeoutSeconds 180
} catch {
    if (-not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
        try {
            $process.WaitForExit()
        } catch {
        }
    }

    Write-Host ""
    Write-Host "UI failed to become ready. Recent stdout:" -ForegroundColor Red
    if (Test-Path -LiteralPath $stdoutLog) {
        Get-Content -LiteralPath $stdoutLog -Tail 80
    }
    Write-Host ""
    Write-Host "UI failed to become ready. Recent stderr:" -ForegroundColor Red
    if (Test-Path -LiteralPath $stderrLog) {
        Get-Content -LiteralPath $stderrLog -Tail 80
    }
    throw
}

$listenerPid = @(Get-ListeningProcessIds -ListenPort $Port | Select-Object -First 1)
if ($listenerPid.Count -eq 0 -or $listenerPid[0] -le 0) {
    throw "UI became ready but no listening process was found on port ${Port}"
}

Set-Content -LiteralPath $pidFile -Value $listenerPid[0] -NoNewline

Write-Host ""
Write-Host "Local UI is ready." -ForegroundColor Green
Write-Host "  URL: ${rootUrl}"
Write-Host "  launcher PID: $($process.Id)"
Write-Host "  listener PID: $($listenerPid[0])"
Write-Host "  stdout: $stdoutLog"
Write-Host "  stderr: $stderrLog"
Write-Host "  LISTINGKIT_API_BASE: $ApiBase"
Write-Host "  LISTINGKIT_SERVICE_API_BASE: $ServiceApiBase"
if (-not [string]::IsNullOrWhiteSpace($BypassAuthGate)) {
    Write-Host "  LISTINGKIT_UI_BYPASS_AUTH_GATE: $BypassAuthGate"
}
Write-Host ""
Write-Host "Stop command:" -ForegroundColor Yellow
Write-Host "  Stop-Process -Id $($listenerPid[0])"
