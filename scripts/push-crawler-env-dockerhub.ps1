[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [string]$Namespace = "task-processor",
    [string]$DeploymentName = "",
    [string]$OverlayPath = "",
    [switch]$SkipApply,
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$RepoRoot = (git rev-parse --show-toplevel 2>$null)
if (-not $RepoRoot) {
    throw "Unable to determine repository root. Run this script inside the git repository."
}

Set-Location $RepoRoot

$ImageName = "crawler-env"
$Dockerfile = "deployments/docker/Dockerfile.crawler-env"

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

$VersionedImage = "$DockerHubUser/${ImageName}:$Tag"
$LatestImage = "$DockerHubUser/${ImageName}:latest"
$ShouldDeploy = (-not $SkipApply) -and -not [string]::IsNullOrWhiteSpace($OverlayPath) -and -not [string]::IsNullOrWhiteSpace($DeploymentName)

function Invoke-Step {
    param(
        [string]$Title,
        [scriptblock]$Action
    )

    Write-Host ""
    Write-Host "==> $Title" -ForegroundColor Cyan
    & $Action
    if ($LASTEXITCODE -ne 0) {
        throw "$Title failed"
    }
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Crawler Env Build / Push / Deploy" -ForegroundColor Cyan
Write-Host "  Image: $VersionedImage" -ForegroundColor Cyan
if ($ShouldDeploy) {
    Write-Host "  Namespace: $Namespace" -ForegroundColor Cyan
    Write-Host "  Deployment: $DeploymentName" -ForegroundColor Cyan
} else {
    Write-Host "  Deployment: skipped (set -OverlayPath and -DeploymentName to enable)" -ForegroundColor Yellow
}
Write-Host "========================================" -ForegroundColor Cyan

Invoke-Step "Build Docker image" {
    $dockerArgs = @("build", "-f", $Dockerfile, "-t", $VersionedImage)
    if ($PublishLatest) {
        $dockerArgs += @("-t", $LatestImage)
    }
    $dockerArgs += "."
    & docker @dockerArgs
}

Invoke-Step "Push versioned image" {
    docker push $VersionedImage
}

if ($PublishLatest) {
    Invoke-Step "Push latest image" {
        docker push $LatestImage
    }
}

if ($ShouldDeploy) {
    Invoke-Step "Apply Kubernetes manifests" {
        kubectl apply -k $OverlayPath
    }

    Invoke-Step "Update deployment image" {
        kubectl -n $Namespace set image deployment/$DeploymentName "$DeploymentName=$VersionedImage"
    }

    Invoke-Step "Wait for rollout" {
        kubectl -n $Namespace rollout status deployment/$DeploymentName --timeout=5m
        kubectl -n $Namespace get pods -l "app=$DeploymentName" -o wide
    }
}

Write-Host ""
Write-Host "Script finished successfully." -ForegroundColor Green
Write-Host "  Version image: $VersionedImage" -ForegroundColor Green
if ($PublishLatest) {
    Write-Host "  Latest image:  $LatestImage" -ForegroundColor Green
} else {
    Write-Host "Skipped pushing :latest. Use -PublishLatest if you intentionally want to refresh the floating tag." -ForegroundColor Yellow
}
