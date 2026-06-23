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
    [string]$RuntimeBaseImage = $(if ($env:SHEIN_RUNTIME_BASE_IMAGE) { $env:SHEIN_RUNTIME_BASE_IMAGE } else { "xuwei190/crawler-env:latest" }),
    [string[]]$DeploymentNames = @(
        "shein-listing-store-a",
        "shein-listing-store-b",
        "shein-listing-store-c",
        "shein-listing-store-d"
    ),
    [string]$OverlayPath = "deployments/kubernetes/shein-listing/overlays/prod-store-auto-shard",
    [switch]$SkipTests,
    [switch]$SkipApply,
    [switch]$PublishLatest,
    [switch]$Fast,
    [switch]$SequentialRollout,
    [switch]$UseShardStatefulSet,
    [string]$ShardStatefulSetName = "shein-listing-shard",
    [string]$OwnershipControllerDeploymentName = "shein-listing-ownership-controller",
    [string]$ControlPlaneDeploymentName = "shein-listing-control-plane",
    [int]$ShardBatchSize = 4
)

$ErrorActionPreference = "Stop"

$ImageName = "task-processor-shein-listing"
$Dockerfile = "deployments/docker/Dockerfile.listing"
$ShardRolloutScript = Join-Path $PSScriptRoot "rollout-shein-shard-statefulset.ps1"

if ($Fast) {
    $SkipTests = $true
    $SkipApply = $true
}

if (-not $UseShardStatefulSet -and $OverlayPath -like "*prod-auto-shard-statefulset*") {
    $UseShardStatefulSet = $true
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

function Wait-RolloutsParallel {
    param(
        [string]$Namespace,
        [string[]]$DeploymentNames,
        [int]$TimeoutSeconds = 300
    )

    $jobs = @()
    try {
        foreach ($deploymentName in $DeploymentNames) {
            $jobs += Start-Job -Name "rollout-$deploymentName" -ScriptBlock {
                param($Ns, $Name, $Timeout)
                kubectl -n $Ns rollout status "deployment/$Name" --timeout="${Timeout}s"
                if ($LASTEXITCODE -ne 0) {
                    throw "kubectl rollout status failed for deployment/$Name"
                }
            } -ArgumentList $Namespace, $deploymentName, $TimeoutSeconds
        }

        Wait-Job -Job $jobs | Out-Null

        $failedJobs = @($jobs | Where-Object { $_.State -ne "Completed" })
        foreach ($job in $jobs) {
            Receive-Job -Job $job
        }

        if ($failedJobs.Count -gt 0) {
            $failedNames = ($failedJobs | ForEach-Object { $_.Name }) -join ", "
            throw "Parallel rollout wait failed: $failedNames"
        }
    }
    finally {
        foreach ($job in $jobs) {
            Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
        }
    }
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SHEIN Listing Build / Push / Deploy" -ForegroundColor Cyan
Write-Host "  Image: $FullImage" -ForegroundColor Cyan
Write-Host "  Runtime base: $RuntimeBaseImage" -ForegroundColor Cyan
Write-Host "  Namespace: $Namespace" -ForegroundColor Cyan
Write-Host "  Overlay: $OverlayPath" -ForegroundColor Cyan
Write-Host "  Deployments: $($DeploymentNames -join ', ')" -ForegroundColor Cyan
Write-Host "  Fast mode: $Fast" -ForegroundColor Cyan
Write-Host "  Sequential rollout: $SequentialRollout" -ForegroundColor Cyan
Write-Host "  Shard StatefulSet mode: $UseShardStatefulSet" -ForegroundColor Cyan
if ($UseShardStatefulSet) {
    Write-Host "  Shard StatefulSet: $ShardStatefulSetName" -ForegroundColor Cyan
    Write-Host "  Shard batch size: $ShardBatchSize" -ForegroundColor Cyan
    Write-Host "  Ownership controller: $OwnershipControllerDeploymentName" -ForegroundColor Cyan
    Write-Host "  Control plane: $ControlPlaneDeploymentName" -ForegroundColor Cyan
}
Write-Host "========================================" -ForegroundColor Cyan

if (-not $SkipTests) {
    Invoke-Step "[1/7] Running targeted tests..." {
        $previousGoWork = $env:GOWORK
        $env:GOWORK = "off"
        try {
        go test ./internal/app/consumer/...
        if ($LASTEXITCODE -ne 0) { throw "go test ./internal/app/consumer/... failed" }

        go test ./cmd/shein-listing/...
        if ($LASTEXITCODE -ne 0) { throw "go test ./cmd/shein-listing/... failed" }

        go test ./internal/listingcontrol ./internal/app/runtime/listingcontrol
        if ($LASTEXITCODE -ne 0) { throw "go test ./internal/listingcontrol ./internal/app/runtime/listingcontrol failed" }
        }
        finally {
            $env:GOWORK = $previousGoWork
        }
    }
}

Invoke-Step "[2/7] Building image..." {
    $dockerArgs = @(
      "build",
      "--build-arg", "RUNTIME_BASE_IMAGE=$RuntimeBaseImage",
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

Invoke-Step "[6/7] Updating workloads..." {
    if ($UseShardStatefulSet) {
        if (-not (Test-Path $ShardRolloutScript)) {
            throw "shard rollout script not found: $ShardRolloutScript"
        }
        kubectl -n $Namespace set image deployment/$OwnershipControllerDeploymentName "shein-listing=$FullImage"
        if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed for deployment/$OwnershipControllerDeploymentName" }

        kubectl -n $Namespace set image deployment/$ControlPlaneDeploymentName "listing-control-plane=$FullImage"
        if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed for deployment/$ControlPlaneDeploymentName" }

        & $ShardRolloutScript -Namespace $Namespace -StatefulSetName $ShardStatefulSetName -ContainerName "shein-listing" -Image $FullImage -BatchSize $ShardBatchSize
        if ($LASTEXITCODE -ne 0) { throw "shard StatefulSet rollout failed" }
    } else {
        foreach ($deploymentName in $DeploymentNames) {
            kubectl -n $Namespace set image deployment/$deploymentName "shein-listing=$FullImage"
            if ($LASTEXITCODE -ne 0) { throw "kubectl set image failed for deployment/$deploymentName" }
        }
    }
}

Invoke-Step "[7/7] Waiting for rollouts..." {
    if ($UseShardStatefulSet) {
        kubectl -n $Namespace rollout status deployment/$OwnershipControllerDeploymentName --timeout=5m
        if ($LASTEXITCODE -ne 0) { throw "kubectl rollout status failed for deployment/$OwnershipControllerDeploymentName" }

        kubectl -n $Namespace rollout status deployment/$ControlPlaneDeploymentName --timeout=5m
        if ($LASTEXITCODE -ne 0) { throw "kubectl rollout status failed for deployment/$ControlPlaneDeploymentName" }

        kubectl -n $Namespace get pods -l "app=$ShardStatefulSetName" -o wide
        if ($LASTEXITCODE -ne 0) { throw "kubectl get pods failed for app=$ShardStatefulSetName" }

        kubectl -n $Namespace get pods -l "app=$ControlPlaneDeploymentName" -o wide
        if ($LASTEXITCODE -ne 0) { throw "kubectl get pods failed for app=$ControlPlaneDeploymentName" }
    } else {
        if ($SequentialRollout) {
            foreach ($deploymentName in $DeploymentNames) {
                kubectl -n $Namespace rollout status deployment/$deploymentName --timeout=5m
                if ($LASTEXITCODE -ne 0) { throw "kubectl rollout status failed for deployment/$deploymentName" }
            }
        } else {
            Wait-RolloutsParallel -Namespace $Namespace -DeploymentNames $DeploymentNames -TimeoutSeconds 300
        }

        foreach ($deploymentName in $DeploymentNames) {
            kubectl -n $Namespace get pods -l "app=$deploymentName" -o wide
            if ($LASTEXITCODE -ne 0) { throw "kubectl get pods failed for app=$deploymentName" }
        }
    }
}

Write-Host ""
Write-Host "Deployment finished successfully." -ForegroundColor Green
if ($Fast) {
    Write-Host "Fast mode skipped tests and manifest apply. Use without -Fast for a full release." -ForegroundColor Yellow
}
if (-not $PublishLatest) {
    Write-Host "Skipped pushing :latest. Use -PublishLatest if you intentionally want to refresh the floating tag." -ForegroundColor Yellow
}
Write-Host "  Version image: $FullImage" -ForegroundColor Green
if ($PublishLatest) {
    Write-Host "  Latest image:  $LatestImage" -ForegroundColor Green
}
