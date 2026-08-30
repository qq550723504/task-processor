param(
    [int]$Port = 3000,
    [string]$ApiBase = "http://localhost:8085/api/v1/listing-kits",
    [string]$ServiceApiBase = "http://localhost:8085/api/v1",
    [switch]$IsolatedAcceptance,
    [string]$RuntimeDirectory = ""
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
        if (-not [string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name))) {
            continue
        }

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
        Write-Host "Loaded local .env values for local UI startup." -ForegroundColor DarkGreen
    }
}

function Import-DeployedListingKitAuthSecrets {
    if ([string]::IsNullOrWhiteSpace($env:KUBECONFIG)) {
        Write-Host "KUBECONFIG is not set; keeping local Auth.js credentials." -ForegroundColor DarkYellow
        return
    }

    try {
        $json = & kubectl -n task-processor get secret listingkit-workbench-secret -o json 2>$null
        if (-not $json) {
            throw "the deployed ListingKit secret was not returned"
        }
        $secret = $json | ConvertFrom-Json
        # Keep AUTH_SECRET from .env so a UI restart can still decrypt the
        # existing localhost Auth.js session. Only the OAuth client secret has
        # to match the deployed ZITADEL application for a future login.
        foreach ($name in @("ZITADEL_CLIENT_SECRET")) {
            $encoded = $secret.data.$name
            if ([string]::IsNullOrWhiteSpace($encoded)) {
                throw "missing $name"
            }
            $value = [System.Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded))
            [Environment]::SetEnvironmentVariable($name, $value, "Process")
        }
        Write-Host "Loaded the deployed ListingKit ZITADEL client credential for this local UI process." -ForegroundColor DarkGreen
    } catch {
        Write-Host "Could not load deployed ListingKit Auth.js credentials: $($_.Exception.Message)" -ForegroundColor DarkYellow
    }
}

function Get-ListeningProcessIds {
    param([int]$ListenPort)

    $connections = @(Get-ListeningConnections -ListenPort $ListenPort)
    if ($connections.Count -eq 0) {
        return @()
    }

    return $connections |
        Select-Object -ExpandProperty OwningProcess -Unique |
        Where-Object { $_ -gt 0 }
}

function Get-ListeningConnections {
    param([int]$ListenPort)

    return @(Get-NetTCPConnection -State Listen -LocalPort $ListenPort -ErrorAction SilentlyContinue)
}

function Assert-LoopbackListener {
    param([int]$ListenPort)

    $connections = @(Get-ListeningConnections -ListenPort $ListenPort)
    if ($connections.Count -eq 0) {
        throw "No listener was found on isolated acceptance port $ListenPort"
    }
    if (@($connections | Where-Object { $_.LocalAddress -notin @("127.0.0.1", "::1") }).Count -gt 0) {
        throw "Isolated acceptance port $ListenPort is not bound exclusively to loopback"
    }
}

function Stop-VerifiedUiProcessTree {
    param(
        [System.Diagnostics.Process]$RootProcess,
        [int]$ListenPort,
        [string]$ExpectedCommandContains
    )

    if ($null -ne $RootProcess -and -not $RootProcess.HasExited) {
        if ([System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT) {
            & taskkill.exe /PID $RootProcess.Id /T /F 2>$null | Out-Null
        } else {
            Stop-Process -Id $RootProcess.Id -Force -ErrorAction SilentlyContinue
        }
        try { $RootProcess.WaitForExit() } catch {}
    }
    foreach ($listenerPid in @(Get-ListeningProcessIds -ListenPort $ListenPort)) {
        $record = Get-CimInstance Win32_Process -Filter "ProcessId = $listenerPid" -ErrorAction SilentlyContinue
        if ($null -ne $record -and ([string]$record.CommandLine).IndexOf($ExpectedCommandContains, [System.StringComparison]::OrdinalIgnoreCase) -ge 0) {
            Stop-Process -Id $listenerPid -Force -ErrorAction SilentlyContinue
        }
    }
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

function Initialize-UiLaunchEnvironment {
    param(
        [string]$RepoRoot,
        [switch]$IsolatedAcceptance
    )

    if ($IsolatedAcceptance) {
        $env:KUBECONFIG = ""
        Write-Host "Isolated acceptance mode: repository .env and deployed Kubernetes credentials are disabled." -ForegroundColor DarkGreen
        return
    }

    Import-DotEnvFile -Path (Join-Path $RepoRoot ".env")
    Import-DeployedListingKitAuthSecrets
}

function Assert-PortAvailable {
    param([int]$ListenPort)

    $processIds = @(Get-ListeningProcessIds -ListenPort $ListenPort)
    if ($processIds.Count -gt 0) {
        throw "Port $ListenPort is already owned by PID(s) $($processIds -join ', '); isolated acceptance refuses to stop unrelated processes"
    }
}

function Resolve-IsolatedRuntimeDirectory {
    param(
        [string]$RepoRoot,
        [string]$RequestedPath
    )

    if ([string]::IsNullOrWhiteSpace($RequestedPath)) {
        throw "-RuntimeDirectory is required with -IsolatedAcceptance"
    }
    $allowedRoot = [System.IO.Path]::GetFullPath((Join-Path $RepoRoot ".local\image-agent-acceptance"))
    $resolved = [System.IO.Path]::GetFullPath($RequestedPath)
    $allowedPrefix = $allowedRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    if (-not $resolved.StartsWith($allowedPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "isolated runtime directory must be below $allowedRoot"
    }
    return $resolved
}

function Wait-ForUiReady {
    param(
        [string]$RootUrl,
        [int]$ProcessId,
        [int]$TimeoutSeconds = 180
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if ($ProcessId -gt 0 -and $null -eq (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)) {
            throw "UI process exited before its HTTP endpoint became ready"
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
$runtimeDir = if ($IsolatedAcceptance) {
    Resolve-IsolatedRuntimeDirectory -RepoRoot $repoRoot -RequestedPath $RuntimeDirectory
} else {
    Join-Path $uiRoot ".local-dev"
}
$stdoutLog = Join-Path $runtimeDir "ui-stdout.log"
$stderrLog = Join-Path $runtimeDir "ui-stderr.log"
$pidFile = Join-Path $runtimeDir "listingkit-ui-local.pid"
$rootUrl = "http://127.0.0.1:${Port}"
$nextCli = Join-Path $uiRoot "node_modules\next\dist\bin\next"
$nodeExecutable = (Get-Command node.exe -ErrorAction Stop).Source

Ensure-Directory -Path $runtimeDir

if (-not (Test-Path -LiteralPath $nextCli)) {
    throw "Next.js CLI not found: $nextCli. Run npm install in web/listingkit-ui first."
}

if ($IsolatedAcceptance) {
    Assert-PortAvailable -ListenPort $Port
} else {
    Stop-ListeningProcesses -ListenPort $Port
}

Remove-FileIfExists -Path $stdoutLog
Remove-FileIfExists -Path $stderrLog
Remove-FileIfExists -Path $pidFile

Initialize-UiLaunchEnvironment -RepoRoot $repoRoot -IsolatedAcceptance:$IsolatedAcceptance
Set-EnvIfMissing -Name "LISTINGKIT_API_BASE" -Value $ApiBase
Set-EnvIfMissing -Name "LISTINGKIT_SERVICE_API_BASE" -Value $ServiceApiBase

$nextArguments = @($nextCli, "dev", "-p", $Port.ToString())
if ($IsolatedAcceptance) {
    $nextArguments += @("-H", "127.0.0.1")
}

Write-Host "Starting local listingkit-ui on port ${Port}..." -ForegroundColor Cyan
$process = Start-Process `
    -FilePath $nodeExecutable `
    -ArgumentList $nextArguments `
    -WorkingDirectory $uiRoot `
    -WindowStyle Hidden `
    -PassThru `
    -RedirectStandardOutput $stdoutLog `
    -RedirectStandardError $stderrLog

try {
    Wait-ForUiReady -RootUrl $rootUrl -ProcessId $process.Id -TimeoutSeconds 180
    if ($IsolatedAcceptance) {
        Assert-LoopbackListener -ListenPort $Port
    }
    $listenerPid = @(Get-ListeningProcessIds -ListenPort $Port | Select-Object -First 1)
    if ($listenerPid.Count -eq 0 -or $listenerPid[0] -le 0) {
        throw "UI became ready but no listening process was found on port ${Port}"
    }
} catch {
    Stop-VerifiedUiProcessTree -RootProcess $process -ListenPort $Port -ExpectedCommandContains $uiRoot

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
Write-Host ""
Write-Host "Stop command:" -ForegroundColor Yellow
Write-Host "  Stop-Process -Id $($listenerPid[0])"
