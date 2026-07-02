# Build and push the standalone Listing scheduler image.
# Usage:
#   .\build-push-listing-scheduler.ps1
#   .\build-push-listing-scheduler.ps1 -Tag v20260702-scheduler

[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [string]$RuntimeBaseImage = $(if ($env:SHEIN_RUNTIME_BASE_IMAGE) { $env:SHEIN_RUNTIME_BASE_IMAGE } else { "xuwei190/crawler-env:latest" }),
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$ImageName = "task-processor-listing-scheduler"
$Dockerfile = "deployments/docker/Dockerfile.listing"

if (-not $Tag) {
    $GitSha = (git rev-parse --short HEAD 2>$null)
    $Dirty = (git status --short --untracked-files=no 2>$null)
    if ($GitSha) {
        $Tag = if ([string]::IsNullOrWhiteSpace($Dirty)) { $GitSha } else { "$GitSha-dirty" }
    }
    if (-not $Tag) {
        $Tag = Get-Date -Format "yyyyMMdd-HHmmss"
    }
}

$FullImage = "$DockerHubUser/${ImageName}:$Tag"
$LatestImage = "$DockerHubUser/${ImageName}:latest"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Listing Scheduler Build / Push" -ForegroundColor Cyan
Write-Host "  Image: $FullImage" -ForegroundColor Cyan
Write-Host "  Runtime base: $RuntimeBaseImage" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$dockerArgs = @(
    "build",
    "--build-arg", "RUNTIME_BASE_IMAGE=$RuntimeBaseImage",
    "--build-arg", "SERVICE_CMD=./cmd/listing-scheduler/main.go",
    "-f", $Dockerfile,
    "-t", $FullImage
)
if ($PublishLatest) {
    $dockerArgs += @("-t", $LatestImage)
}
$dockerArgs += "."

& docker @dockerArgs
if ($LASTEXITCODE -ne 0) { throw "docker build failed" }

docker push $FullImage
if ($LASTEXITCODE -ne 0) { throw "docker push $FullImage failed" }

if ($PublishLatest) {
    docker push $LatestImage
    if ($LASTEXITCODE -ne 0) { throw "docker push $LatestImage failed" }
}

Write-Host "Listing scheduler image pushed: $FullImage" -ForegroundColor Green
if ($PublishLatest) {
    Write-Host "Latest image pushed: $LatestImage" -ForegroundColor Green
}
