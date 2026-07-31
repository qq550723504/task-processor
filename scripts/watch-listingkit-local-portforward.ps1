param(
    [int]$CheckIntervalSeconds = 5,
    [string]$DbNamespace = "platform-data",
    [string]$DbPod = "shared-postgresql-0",
    [string]$RedisNamespace = "platform-data",
    [string]$RedisPod = "redis-0",
    [string]$TemporalNamespace = "temporal",
    [string]$TemporalPod = "temporal-frontend-67fc4466d4-pqwfn"
)

$ErrorActionPreference = "Stop"

$runtimeDir = Join-Path $PSScriptRoot "..\.local\tmp\listingkit-portforward-watch"
New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null

$specs = @(
    @{ Name = "db"; Namespace = $DbNamespace; Pod = $DbPod; LocalPort = 15432; RemotePort = 5432 },
    @{ Name = "redis"; Namespace = $RedisNamespace; Pod = $RedisPod; LocalPort = 16379; RemotePort = 6379 },
    @{ Name = "temporal"; Namespace = $TemporalNamespace; Pod = $TemporalPod; LocalPort = 7233; RemotePort = 7233 }
)

function Stop-SpecProcess {
    param($Spec)

    $target = "pod/$($Spec.Pod)"
    $mapping = "$($Spec.LocalPort):$($Spec.RemotePort)"
    Get-CimInstance Win32_Process |
        Where-Object {
            $_.Name -eq "kubectl.exe" -and
            $_.CommandLine -match "port-forward" -and
            $_.CommandLine -match [regex]::Escape($target) -and
            $_.CommandLine -match [regex]::Escape($mapping)
        } |
        ForEach-Object {
            Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
        }
}

function Start-SpecProcess {
    param($Spec)

    Stop-SpecProcess -Spec $Spec
    $stdout = Join-Path $runtimeDir "$($Spec.Name).stdout.log"
    $stderr = Join-Path $runtimeDir "$($Spec.Name).stderr.log"
    $process = Start-Process -FilePath "kubectl" `
        -ArgumentList @("-n", $Spec.Namespace, "port-forward", "pod/$($Spec.Pod)", "$($Spec.LocalPort):$($Spec.RemotePort)") `
        -WindowStyle Hidden `
        -PassThru `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr

    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $deadline) {
        if ($process.HasExited) {
            throw "Port-forward for $($Spec.Name) exited while starting."
        }
        $listening = Get-NetTCPConnection -State Listen -LocalPort $Spec.LocalPort -ErrorAction SilentlyContinue
        if ($listening) {
            return $process
        }
        Start-Sleep -Milliseconds 400
    }

    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    throw "Timed out starting port-forward for $($Spec.Name)."
}

function Test-Postgres {
    $client = [System.Net.Sockets.TcpClient]::new("127.0.0.1", 15432)
    try {
        $stream = $client.GetStream()
        $stream.ReadTimeout = 4000
        $payload = [System.Text.Encoding]::ASCII.GetBytes("user`0postgres`0database`0ruoyi-vue-pro`0`0")
        $length = 8 + $payload.Length
        $message = New-Object byte[] $length
        [Array]::Copy([BitConverter]::GetBytes([System.Net.IPAddress]::HostToNetworkOrder($length)), 0, $message, 0, 4)
        [Array]::Copy([BitConverter]::GetBytes([System.Net.IPAddress]::HostToNetworkOrder(196608)), 0, $message, 4, 4)
        [Array]::Copy($payload, 0, $message, 8, $payload.Length)
        $stream.Write($message, 0, $message.Length)
        $response = New-Object byte[] 1
        return $stream.Read($response, 0, 1) -eq 1 -and [char]$response[0] -eq "R"
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

function Test-Redis {
    $client = [System.Net.Sockets.TcpClient]::new("127.0.0.1", 16379)
    try {
        $stream = $client.GetStream()
        $stream.ReadTimeout = 4000
        $message = [System.Text.Encoding]::ASCII.GetBytes("*1`r`n`$4`r`nPING`r`n")
        $stream.Write($message, 0, $message.Length)
        $response = New-Object byte[] 128
        return $stream.Read($response, 0, $response.Length) -gt 0
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

function Test-Temporal {
    try {
        $client = [System.Net.Sockets.TcpClient]::new("127.0.0.1", 7233)
        $client.Dispose()
        return $true
    } catch {
        return $false
    }
}

$processes = @{}
while ($true) {
    $healthy =
        $processes.Count -eq $specs.Count -and
        ($processes.Values | Where-Object { $_.HasExited }).Count -eq 0 -and
        (Test-Postgres) -and
        (Test-Redis) -and
        (Test-Temporal)

    if (-not $healthy) {
        Write-Host "[$(Get-Date -Format s)] Rebuilding ListingKit port-forwards..."
        foreach ($spec in $specs) {
            Stop-SpecProcess -Spec $spec
        }
        $processes = @{}
        foreach ($spec in $specs) {
            $processes[$spec.Name] = Start-SpecProcess -Spec $spec
        }
        Write-Host "[$(Get-Date -Format s)] ListingKit port-forwards are ready."
    }

    Start-Sleep -Seconds $CheckIntervalSeconds
}
