param(
    [string]$ContainerName = "task-processor-temporal-dev",
    [string]$Image = "temporalio/temporal:latest",
    [int]$FrontendPort = 7233,
    [int]$UIPort = 8233
)

$existing = docker ps -aq --filter "name=^${ContainerName}$"
if ($LASTEXITCODE -ne 0) {
    throw "failed to query docker containers"
}

if ($existing) {
    $running = docker ps -q --filter "name=^${ContainerName}$"
    if ($LASTEXITCODE -ne 0) {
        throw "failed to query running docker containers"
    }
    if ($running) {
        Write-Host "Temporal dev server is already running in container ${ContainerName}."
        Write-Host "Frontend: localhost:${FrontendPort}"
        Write-Host "UI: http://localhost:${UIPort}/"
        exit 0
    }

    docker rm -f $ContainerName | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "failed to remove existing container ${ContainerName}"
    }
}

docker run -d `
    --name $ContainerName `
    -p "${FrontendPort}:7233" `
    -p "${UIPort}:8233" `
    $Image `
    server start-dev --ip 0.0.0.0 | Out-Null

if ($LASTEXITCODE -ne 0) {
    throw "failed to start Temporal dev server container"
}

$deadline = (Get-Date).AddSeconds(20)
while ((Get-Date) -lt $deadline) {
    try {
        $tcp = Test-NetConnection -ComputerName "127.0.0.1" -Port $FrontendPort -WarningAction SilentlyContinue
        if ($tcp.TcpTestSucceeded) {
            Write-Host "Temporal dev server is ready."
            Write-Host "Frontend: localhost:${FrontendPort}"
            Write-Host "UI: http://localhost:${UIPort}/"
            exit 0
        }
    } catch {
    }
    Start-Sleep -Milliseconds 500
}

docker logs $ContainerName
throw "Temporal dev server did not become ready within timeout"
