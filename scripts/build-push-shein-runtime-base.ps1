# Build and push the pre-baked SHEIN runtime base image.
# Usage:
#   .\scripts\build-push-shein-runtime-base.ps1
#   .\scripts\build-push-shein-runtime-base.ps1 -Tag 20260512-runtime-v1

[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "latest",
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$ImageName = "task-processor-shein-runtime"
$Dockerfile = "deployments/docker/Dockerfile.shein-runtime"
$FullImage = "$DockerHubUser/${ImageName}:$Tag"
$LatestImage = "$DockerHubUser/${ImageName}:latest"

function Invoke-Step {
    param(
        [string]$Title,
        [scriptblock]$Action
    )

    Write-Host ""
    Write-Host $Title -ForegroundColor Yellow
    & $Action
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SHEIN Runtime Base Build / Push" -ForegroundColor Cyan
Write-Host "  Image: $FullImage" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

Invoke-Step "[1/2] Building runtime base image..." {
    $dockerArgs = @(
        "build",
        "-f", $Dockerfile,
        "-t", $FullImage
    )
    if ($PublishLatest -and $Tag -ne "latest") {
        $dockerArgs += @("-t", $LatestImage)
    }
    $dockerArgs += "."

    & docker @dockerArgs
    if ($LASTEXITCODE -ne 0) { throw "docker build failed" }
}

Invoke-Step "[2/2] Pushing runtime base image..." {
    docker push $FullImage
    if ($LASTEXITCODE -ne 0) { throw "docker push $FullImage failed" }

    if ($PublishLatest -and $Tag -ne "latest") {
        docker push $LatestImage
        if ($LASTEXITCODE -ne 0) { throw "docker push $LatestImage failed" }
    }
}

Write-Host ""
Write-Host "Runtime base image finished successfully." -ForegroundColor Green
Write-Host "  Version image: $FullImage" -ForegroundColor Green
if ($PublishLatest -and $Tag -ne "latest") {
    Write-Host "  Latest image:  $LatestImage" -ForegroundColor Green
}
