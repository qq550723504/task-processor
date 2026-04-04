# Task Docker image build and push to Docker Hub
# Usage: .\push-task-dockerhub.ps1 [-DockerHubUser yourname] [-Tag v1.0.0]

[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$ImageName = "task-processor-task"
$Dockerfile = "deployments/docker/Dockerfile.task"

if (-not $Tag) {
    $Tag = git rev-parse --short HEAD 2>$null
    if (-not $Tag) { $Tag = Get-Date -Format 'yyyyMMdd' }
}

$FullImage = "$DockerHubUser/${ImageName}:$Tag"
$LatestImage = "$DockerHubUser/${ImageName}:latest"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Build & Push Task Image" -ForegroundColor Cyan
Write-Host "  Image: $FullImage" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

Write-Host "`n[1/3] Building image..." -ForegroundColor Yellow
$dockerArgs = @("build", "-f", $Dockerfile, "-t", $FullImage)
if ($PublishLatest) {
    $dockerArgs += @("-t", $LatestImage)
}
$dockerArgs += "."
& docker @dockerArgs
if ($LASTEXITCODE -ne 0) { Write-Host "Build failed" -ForegroundColor Red; exit 1 }

Write-Host "`n[2/3] Pushing $FullImage ..." -ForegroundColor Yellow
docker push $FullImage
if ($LASTEXITCODE -ne 0) { Write-Host "Push failed" -ForegroundColor Red; exit 1 }

if ($PublishLatest) {
    Write-Host "`n[3/3] Pushing $LatestImage ..." -ForegroundColor Yellow
    docker push $LatestImage
    if ($LASTEXITCODE -ne 0) { Write-Host "Push failed" -ForegroundColor Red; exit 1 }
}

Write-Host "`nDone" -ForegroundColor Green
Write-Host "  $FullImage" -ForegroundColor Green
if ($PublishLatest) {
    Write-Host "  $LatestImage" -ForegroundColor Green
} else {
    Write-Host "  (skipped :latest; use -PublishLatest to refresh it)" -ForegroundColor Yellow
}
