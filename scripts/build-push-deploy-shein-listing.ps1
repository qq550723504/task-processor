# Build, push, and deploy the SHEIN listing service to K3S.
# Usage:
#   .\build-push-deploy-shein-listing.ps1
#   .\build-push-deploy-shein-listing.ps1 -Tag v20260402-1
#   .\build-push-deploy-shein-listing.ps1 -DockerHubUser xuwei190 -Namespace task-processor

[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [string]$Namespace = "task-processor",
    [string]$DeploymentName = "shein-listing",
    [string]$OverlayPath = "deployments/kubernetes/shein-listing/overlays/prod",
    [switch]$SkipTests,
    [switch]$SkipApply,
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$ImageName = "task-processor-shein-listing"
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
Write-Host "  SHEIN Listing Build / Push / Deploy" -ForegroundColor Cyan
Write-Host "  Image: $FullImage" -ForegroundColor Cyan
Write-Host "  Namespace: $Namespace" -ForegroundColor Cyan
Write-Host "  Deployment: $DeploymentName" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

if (-not $SkipTests) {
    Invoke-Step "[1/7] Running targeted tests..." {
        go test ./internal/app/consumer/...
        if ($LASTEXITCODE -ne 0) { throw "go test ./internal/app/consumer/... failed" }

        go test ./cmd/shein-listing/...
        if ($LASTEXITCODE -ne 0) { throw "go test ./cmd/shein-listing/... failed" }
    }
}

Invoke-Step "[2/7] Building image..." {
    $dockerArgs = @(
      "build",
      "--build-arg", "SERVICE_CMD=./cmd/shein-listing/main.go",
      "-f", $Dockerfile,
      "-t", $FullImage
    )
    if ($PublishLatest) {
        $dockerArgs += @("-t", $LatestImage)
    }
    $dockerArgs += "."
    & docker @dockerArgs
    if ($LASTEXITCODE -ne 0) { throw "docker build failed" }
}

Invoke-Step "[3/7] Pushing version tag..." {
    docker push $FullImage
    if ($LASTEXITCODE -ne 0) { throw "docker push $FullImage failed" }
}

if ($PublishLatest) {
    Invoke-Step "[4/7] Pushing latest tag..." {
        docker push $LatestImage
        if ($LASTEXITCODE -ne 0) { throw "docker push $LatestImage failed" }
    }
}

if (-not $SkipApply) {
    Invoke-Step "[5/7] Applying Kubernetes manifests..." {
        kubectl apply -k $OverlayPath
        if ($LASTEXITCODE -ne 0) { throw "kubectl apply -k $OverlayPath failed" }
    }
}

Invoke-Step "[6/7] Updating deployment image..." {
    kubectl -n $Namespace set image deployment/$DeploymentName "$DeploymentName=$FullImage"
    if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed" }
}

Invoke-Step "[7/7] Waiting for rollout..." {
    kubectl -n $Namespace rollout status deployment/$DeploymentName --timeout=5m
    if ($LASTEXITCODE -ne 0) { throw "kubectl rollout status failed" }

    kubectl -n $Namespace get pods -l "app=$DeploymentName" -o wide
    if ($LASTEXITCODE -ne 0) { throw "kubectl get pods failed" }
}

Write-Host ""
Write-Host "Deployment finished successfully." -ForegroundColor Green
if (-not $PublishLatest) {
    Write-Host "Skipped pushing :latest. Use -PublishLatest if you intentionally want to refresh the floating tag." -ForegroundColor Yellow
}
Write-Host "  Version image: $FullImage" -ForegroundColor Green
if ($PublishLatest) {
    Write-Host "  Latest image:  $LatestImage" -ForegroundColor Green
}
