param(
    [string]$Namespace = "temporal",
    [string]$Service = "temporal-frontend",
    [int]$LocalPort = 7233,
    [int]$RemotePort = 7233
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

$stdoutLog = Join-Path $env:TEMP ("listingkit-temporal-portforward.{0}.stdout.log" -f $script:PortForwardSessionId)
$stderrLog = Join-Path $env:TEMP ("listingkit-temporal-portforward.{0}.stderr.log" -f $script:PortForwardSessionId)
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

try {
    Wait-ForPortForwardReady -LogPath $stdoutLog -TimeoutSeconds 20

    Write-Host ""
    Write-Host "Remote Temporal port-forward is ready." -ForegroundColor Green
    Write-Host "  Frontend: 127.0.0.1:${LocalPort}"
    Write-Host "  Remote: ${Namespace}/${Service}:${RemotePort}"
    Write-Host "  PID: $($process.Id)"
    Write-Host "  stdout: $stdoutLog"
    Write-Host "  stderr: $stderrLog"
    Write-Host ""
    Write-Host "Suggested env override for local API:" -ForegroundColor Yellow
    Write-Host "  `$env:LISTINGKIT_TEMPORAL_ENABLED='true'"
    Write-Host "  `$env:LISTINGKIT_TEMPORAL_ADDRESS='127.0.0.1:${LocalPort}'"
    Write-Host ""
    Write-Host "This window keeps the port-forward alive. Press Ctrl+C when you are done." -ForegroundColor Yellow

    while ($true) {
        Start-Sleep -Seconds 2
        if ($process.HasExited) {
            $stderr = ""
            if (Test-Path -LiteralPath $stderrLog) {
                $stderr = Get-Content -LiteralPath $stderrLog -Raw -ErrorAction SilentlyContinue
            }
            throw "kubectl port-forward for ${Namespace}/${Service} exited unexpectedly. ${stderr}"
        }
    }
} finally {
    if ($process -and -not $process.HasExited) {
        Write-Host "Stopping kubectl port-forward PID $($process.Id) ..." -ForegroundColor DarkYellow
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit()
    }
}
