[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [string]$Namespace = "task-processor",
    [string]$DeploymentName = "amazon-crawler-api",
    [string]$AppLabel = "amazon-crawler-api",
    [string]$OverlayPath = "deployments/kubernetes/amazon-crawler-api/overlays/prod",
    [switch]$SkipApply,
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$RepoRoot = (git rev-parse --show-toplevel 2>$null)
if (-not $RepoRoot) {
    throw "Unable to determine repository root. Run this script inside the git repository."
}

Set-Location $RepoRoot

$ImageName = "task-processor-amazon-crawler-api"
$Dockerfile = "deployments/docker/Dockerfile.amazon-crawler-api"

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
Write-Host "  Amazon Crawler API Build / Push / Deploy" -ForegroundColor Cyan
Write-Host "  Image: $VersionedImage" -ForegroundColor Cyan
Write-Host "  Namespace: $Namespace" -ForegroundColor Cyan
Write-Host "  App Label: $AppLabel" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

function Get-RolloutTargets {
    param(
        [string]$Namespace,
        [string]$AppLabel,
        [string]$FallbackDeploymentName
    )

    $targets = @()
    $resolved = kubectl -n $Namespace get deployment,daemonset -l "app=$AppLabel" -o name 2>$null
    if ($LASTEXITCODE -eq 0 -and $resolved) {
        $targets = @($resolved | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    }

    if (-not $targets -or $targets.Count -eq 0) {
        $targets = @("deployment/$FallbackDeploymentName")
    }

    return $targets
}

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

if (-not $SkipApply) {
    Invoke-Step "Apply Kubernetes manifests" {
        kubectl apply -k $OverlayPath
    }

    $rolloutTargets = Get-RolloutTargets -Namespace $Namespace -AppLabel $AppLabel -FallbackDeploymentName $DeploymentName
    if (-not $rolloutTargets -or $rolloutTargets.Count -eq 0) {
        throw "No deployment or daemonset targets found for app=$AppLabel"
    }

    Write-Host ""
    Write-Host "Resolved rollout targets:" -ForegroundColor Cyan
    $rolloutTargets | ForEach-Object { Write-Host "  - $_" -ForegroundColor Cyan }

    Invoke-Step "Update workload images" {
        foreach ($target in $rolloutTargets) {
            kubectl -n $Namespace set image $target "amazon-crawler-api=$VersionedImage"
            if ($LASTEXITCODE -ne 0) {
                throw "set image failed for $target"
            }
        }
    }

    Invoke-Step "Wait for rollout" {
        foreach ($target in $rolloutTargets) {
            kubectl -n $Namespace rollout status $target --timeout=5m
            if ($LASTEXITCODE -ne 0) {
                throw "rollout status failed for $target"
            }
        }
        kubectl -n $Namespace get pods -l "app=$DeploymentName" -o wide
    }
}

Write-Host ""
Write-Host "Deployment finished successfully." -ForegroundColor Green
Write-Host "  Version image: $VersionedImage" -ForegroundColor Green
if ($PublishLatest) {
    Write-Host "  Latest image:  $LatestImage" -ForegroundColor Green
} else {
    Write-Host "Skipped pushing :latest. Use -PublishLatest if you intentionally want to refresh the floating tag." -ForegroundColor Yellow
}
