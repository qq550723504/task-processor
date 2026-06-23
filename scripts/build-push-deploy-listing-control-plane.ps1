# Build, push, and deploy the listing control-plane without rolling SHEIN browser workers.
# Usage:
#   .\scripts\build-push-deploy-listing-control-plane.ps1
#   .\scripts\build-push-deploy-listing-control-plane.ps1 -Tag v20260623-1 -SkipTests

[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [string]$Namespace = "task-processor",
    [string]$DeploymentName = "shein-listing-control-plane",
    [string]$ContainerName = "listing-control-plane",
    [string]$ManifestPath = "deployments/kubernetes/shein-listing/overlays/prod-auto-shard-statefulset/listing-control-plane.yaml",
    [switch]$SkipTests,
    [switch]$SkipApply,
    [switch]$PublishLatest,
    [switch]$Fast
)

$ErrorActionPreference = "Stop"

$ImageName = "task-processor-listing-control-plane"
$Dockerfile = "deployments/docker/Dockerfile.listing-control-plane"

if ($Fast) {
    $SkipTests = $true
    $SkipApply = $true
}

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
Write-Host "  Listing Control Plane Build / Push / Deploy" -ForegroundColor Cyan
Write-Host "  Image: $FullImage" -ForegroundColor Cyan
Write-Host "  Namespace: $Namespace" -ForegroundColor Cyan
Write-Host "  Deployment: $DeploymentName" -ForegroundColor Cyan
Write-Host "  Manifest: $ManifestPath" -ForegroundColor Cyan
Write-Host "  Fast mode: $Fast" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

if (-not $SkipTests) {
    Invoke-Step "[1/6] Running targeted tests..." {
        $previousGoWork = $env:GOWORK
        $env:GOWORK = "off"
        try {
            go test ./cmd/listing-control-plane/... ./internal/app/runtime/listingcontrol ./internal/listingcontrol ./internal/listingadmin
            if ($LASTEXITCODE -ne 0) { throw "listing control-plane tests failed" }
        }
        finally {
            $env:GOWORK = $previousGoWork
        }
    }
}

Invoke-Step "[2/6] Building image..." {
    $dockerArgs = @(
        "build",
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

Invoke-Step "[3/6] Pushing version tag..." {
    docker push $FullImage
    if ($LASTEXITCODE -ne 0) { throw "docker push $FullImage failed" }
}

if ($PublishLatest) {
    Invoke-Step "[4/6] Pushing latest tag..." {
        docker push $LatestImage
        if ($LASTEXITCODE -ne 0) { throw "docker push $LatestImage failed" }
    }
}

if (-not $SkipApply) {
    Invoke-Step "[5/6] Applying Kubernetes manifests..." {
        kubectl apply -f $ManifestPath
        if ($LASTEXITCODE -ne 0) { throw "kubectl apply -f $ManifestPath failed" }
    }
}

Invoke-Step "[6/6] Updating control-plane rollout..." {
    kubectl -n $Namespace set image "deployment/$DeploymentName" "$ContainerName=$FullImage"
    if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed for deployment/$DeploymentName" }

    kubectl -n $Namespace rollout status "deployment/$DeploymentName" --timeout=5m
    if ($LASTEXITCODE -ne 0) { throw "kubectl rollout status failed for deployment/$DeploymentName" }

    kubectl -n $Namespace get pods -l "app=$DeploymentName" -o wide
    if ($LASTEXITCODE -ne 0) { throw "kubectl get pods failed for app=$DeploymentName" }
}

Write-Host ""
Write-Host "Control-plane deployment finished successfully." -ForegroundColor Green
Write-Host "  Version image: $FullImage" -ForegroundColor Green
if ($PublishLatest) {
    Write-Host "  Latest image:  $LatestImage" -ForegroundColor Green
}
if ($Fast) {
    Write-Host "Fast mode skipped tests and manifest apply. Use without -Fast for a full release." -ForegroundColor Yellow
}
