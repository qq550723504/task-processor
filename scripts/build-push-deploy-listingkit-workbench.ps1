# Build, push, and deploy ListingKit Workbench to K3S.
# Usage:
#   .\scripts\build-push-deploy-listingkit-workbench.ps1
#   .\scripts\build-push-deploy-listingkit-workbench.ps1 -Tag v20260428-1 -PublishLatest

[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [string]$Namespace = "task-processor",
    [string]$OverlayPath = "deployments/kubernetes/listingkit-workbench/overlays/prod",
    [switch]$SkipTests,
    [switch]$SkipApply,
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$ApiImageName = "task-processor-product-listing-api"
$UiImageName = "task-processor-listingkit-ui"
$ApiDockerfile = "deployments/docker/Dockerfile.product-listing-api"
$UiDockerfile = "deployments/docker/Dockerfile.listingkit-ui"

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

$ApiImage = "$DockerHubUser/${ApiImageName}:$Tag"
$UiImage = "$DockerHubUser/${UiImageName}:$Tag"
$ApiLatestImage = "$DockerHubUser/${ApiImageName}:latest"
$UiLatestImage = "$DockerHubUser/${UiImageName}:latest"

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
Write-Host "  ListingKit Workbench Build / Deploy" -ForegroundColor Cyan
Write-Host "  API image: $ApiImage" -ForegroundColor Cyan
Write-Host "  UI image:  $UiImage" -ForegroundColor Cyan
Write-Host "  Namespace: $Namespace" -ForegroundColor Cyan
Write-Host "  Overlay:   $OverlayPath" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

if (-not $SkipTests) {
    Invoke-Step "[1/8] Running backend tests..." {
        go test ./internal/app/httpapi ./internal/listingkit
        if ($LASTEXITCODE -ne 0) { throw "backend tests failed" }
    }

    Invoke-Step "[2/8] Building frontend..." {
        npm --prefix web/listingkit-ui run build
        if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
    }
}

Invoke-Step "[3/8] Building API image..." {
    $dockerArgs = @("build", "-f", $ApiDockerfile, "-t", $ApiImage)
    if ($PublishLatest) {
        $dockerArgs += @("-t", $ApiLatestImage)
    }
    $dockerArgs += "."
    docker @dockerArgs
    if ($LASTEXITCODE -ne 0) { throw "API docker build failed" }
}

Invoke-Step "[4/8] Building UI image..." {
    $dockerArgs = @("build", "-f", $UiDockerfile, "-t", $UiImage)
    if ($PublishLatest) {
        $dockerArgs += @("-t", $UiLatestImage)
    }
    $dockerArgs += "."
    docker @dockerArgs
    if ($LASTEXITCODE -ne 0) { throw "UI docker build failed" }
}

Invoke-Step "[5/8] Pushing images..." {
    docker push $ApiImage
    if ($LASTEXITCODE -ne 0) { throw "docker push $ApiImage failed" }

    docker push $UiImage
    if ($LASTEXITCODE -ne 0) { throw "docker push $UiImage failed" }

    if ($PublishLatest) {
        docker push $ApiLatestImage
        if ($LASTEXITCODE -ne 0) { throw "docker push $ApiLatestImage failed" }

        docker push $UiLatestImage
        if ($LASTEXITCODE -ne 0) { throw "docker push $UiLatestImage failed" }
    }
}

if (-not $SkipApply) {
    Invoke-Step "[6/8] Applying Kubernetes manifests..." {
        kubectl apply -k $OverlayPath
        if ($LASTEXITCODE -ne 0) { throw "kubectl apply -k $OverlayPath failed" }
    }
}

Invoke-Step "[7/8] Updating deployment images..." {
    kubectl -n $Namespace set image deployment/product-listing-api "product-listing-api=$ApiImage"
    if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed for product-listing-api" }

    kubectl -n $Namespace set image deployment/listingkit-ui "listingkit-ui=$UiImage"
    if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed for listingkit-ui" }
}

Invoke-Step "[8/8] Waiting for rollouts..." {
    kubectl -n $Namespace rollout status deployment/product-listing-api --timeout=5m
    if ($LASTEXITCODE -ne 0) { throw "product-listing-api rollout failed" }

    kubectl -n $Namespace rollout status deployment/listingkit-ui --timeout=5m
    if ($LASTEXITCODE -ne 0) { throw "listingkit-ui rollout failed" }

    kubectl -n $Namespace get pods -l "app in (product-listing-api,listingkit-ui)" -o wide
    if ($LASTEXITCODE -ne 0) { throw "kubectl get pods failed" }
}

Write-Host ""
Write-Host "Deployment finished successfully." -ForegroundColor Green
Write-Host "  API image: $ApiImage" -ForegroundColor Green
Write-Host "  UI image:  $UiImage" -ForegroundColor Green
if (-not $PublishLatest) {
    Write-Host "Skipped pushing :latest. Use -PublishLatest if you intentionally want to refresh the floating tags." -ForegroundColor Yellow
}
