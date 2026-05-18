param(
    [string]$ContainerName = "task-processor-temporal-dev"
)

$existing = docker ps -aq --filter "name=^${ContainerName}$"
if ($LASTEXITCODE -ne 0) {
    throw "failed to query docker containers"
}

if (-not $existing) {
    Write-Host "Temporal dev server container ${ContainerName} is not present."
    exit 0
}

docker rm -f $ContainerName | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "failed to remove container ${ContainerName}"
}

Write-Host "Stopped Temporal dev server container ${ContainerName}."
