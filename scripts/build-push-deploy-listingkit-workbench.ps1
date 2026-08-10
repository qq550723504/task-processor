# Build, push, and deploy ListingKit Workbench to K3S.
# Usage:
#   .\scripts\build-push-deploy-listingkit-workbench.ps1
#   .\scripts\build-push-deploy-listingkit-workbench.ps1 -Tag v20260428-1 -PublishLatest

[CmdletBinding()]
param(
    [string]$DockerHubUser = $(if ($env:DOCKERHUB_USER) { $env:DOCKERHUB_USER } else { "xuwei190" }),
    [string]$Tag = "",
    [string]$Namespace = "task-processor",
    [switch]$SkipTests,
    [switch]$SkipApply,
    [switch]$PublishLatest
)

$ErrorActionPreference = "Stop"

$ApiImageName = "task-processor-product-listing-api"
$PreflightImageName = "task-processor-listingkit-identity-preflight"
$UiImageName = "task-processor-listingkit-ui"
$ApiDockerfile = "deployments/docker/Dockerfile.product-listing-api"
$PreflightDockerfile = "deployments/docker/Dockerfile.listingkit-identity-preflight"
$UiDockerfile = "deployments/docker/Dockerfile.listingkit-ui"
$IdentityPreflightDriver = Join-Path $PSScriptRoot "listingkit-identity-preflight-job.sh"
$ImmutableApiApplyDriver = Join-Path $PSScriptRoot "listingkit-apply-api-deployment.sh"
$IdentityPreflightManifest = "deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml"
$ApiDeploymentManifest = "deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml"

if ($PSBoundParameters.ContainsKey("Tag") -and [string]::IsNullOrWhiteSpace($Tag)) {
    throw "ListingKit release requires a non-empty immutable API image tag"
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

if ([string]::IsNullOrWhiteSpace($Tag) -or $Tag -eq "latest") {
    throw "ListingKit release requires an immutable API image tag; latest is not a release candidate"
}

$ApiImage = "$DockerHubUser/${ApiImageName}:$Tag"
$PreflightImage = "$DockerHubUser/${PreflightImageName}:$Tag"
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

function Resolve-BashExecutable {
    $configuredBash = [Environment]::GetEnvironmentVariable("LISTINGKIT_BASH", "Process")
    if (-not [string]::IsNullOrWhiteSpace($configuredBash)) {
        return $configuredBash
    }

    $gitBash = Join-Path ${env:ProgramFiles} "Git\bin\bash.exe"
    if (Test-Path -LiteralPath $gitBash -PathType Leaf) {
        return $gitBash
    }

    throw "Git Bash is required to run the ListingKit identity preflight release gate; set LISTINGKIT_BASH only when using an explicit trusted Bash executable"
}

function Resolve-PushedImageDigest {
    param([string]$Image)

    $digestReference = (docker image inspect --format '{{index .RepoDigests 0}}' $Image 2>$null).Trim()
    if ($LASTEXITCODE -ne 0 -or $digestReference -notmatch '^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[A-Fa-f0-9]{64}$') {
        throw "could not resolve a digest-pinned reference for $Image"
    }
    return $digestReference
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  ListingKit Workbench Build / Deploy" -ForegroundColor Cyan
Write-Host "  API image: $ApiImage" -ForegroundColor Cyan
Write-Host "  UI image:  $UiImage" -ForegroundColor Cyan
Write-Host "  Namespace: $Namespace" -ForegroundColor Cyan
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

Invoke-Step "[5/9] Building identity preflight runner..." {
    docker build -f $PreflightDockerfile -t $PreflightImage .
    if ($LASTEXITCODE -ne 0) { throw "identity preflight runner docker build failed" }
}

Invoke-Step "[6/9] Pushing images..." {
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

    docker push $PreflightImage
    if ($LASTEXITCODE -ne 0) { throw "docker push $PreflightImage failed" }
}

if (-not $SkipApply) {
    $BashExecutable = Resolve-BashExecutable
    $ApiCandidateImage = Resolve-PushedImageDigest $ApiImage
    $PreflightRunnerImage = Resolve-PushedImageDigest $PreflightImage

    Invoke-Step "[7/10] Running identity preflight release gate..." {
        & $BashExecutable $IdentityPreflightDriver `
            --manifest $IdentityPreflightManifest `
            --namespace $Namespace `
            --image $ApiCandidateImage `
            --runner-image $PreflightRunnerImage
        if ($LASTEXITCODE -ne 0) { throw "identity preflight failed" }
    }

    Invoke-Step "[8/10] Applying immutable API deployment..." {
        & $BashExecutable $ImmutableApiApplyDriver `
            --manifest $ApiDeploymentManifest `
            --namespace $Namespace `
            --image $ApiCandidateImage
        if ($LASTEXITCODE -ne 0) { throw "immutable API deployment apply failed" }
    }

    Invoke-Step "[9/10] Updating matching UI deployment image..." {
        kubectl -n $Namespace set image deployment/listingkit-ui "listingkit-ui=$UiImage"
        if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed for listingkit-ui" }
    }

    Invoke-Step "[10/10] Waiting for rollouts..." {
        kubectl -n $Namespace rollout status deployment/product-listing-api --timeout=5m
        if ($LASTEXITCODE -ne 0) { throw "product-listing-api rollout failed" }

        kubectl -n $Namespace rollout status deployment/listingkit-ui --timeout=5m
        if ($LASTEXITCODE -ne 0) { throw "listingkit-ui rollout failed" }

        kubectl -n $Namespace get pods -l "app in (product-listing-api,listingkit-ui)" -o wide
        if ($LASTEXITCODE -ne 0) { throw "kubectl get pods failed" }
    }
} else {
    Write-Host "Skipped every Kubernetes mutation because -SkipApply was specified." -ForegroundColor Yellow
}

Write-Host ""
if ($SkipApply) {
    Write-Host "Images built and pushed; Kubernetes was not changed." -ForegroundColor Yellow
} else {
    Write-Host "Gated deployment finished successfully." -ForegroundColor Green
    Write-Host "  API image: $ApiImage" -ForegroundColor Green
    Write-Host "  UI image:  $UiImage" -ForegroundColor Green
}
if (-not $PublishLatest) {
    Write-Host "Skipped pushing :latest. Use -PublishLatest if you intentionally want to refresh the floating tags." -ForegroundColor Yellow
}
