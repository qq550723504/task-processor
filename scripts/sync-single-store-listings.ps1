[CmdletBinding()]
param(
    [string]$ConfigPath = "",
    [string]$Namespace = "task-processor",
    [switch]$Apply,
    [switch]$Prune,
    [int]$RolloutTimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $RepoRoot "deployments\kubernetes\shein-listing\stores.single-pod.json"
}

$DeployScriptPath = Join-Path $PSScriptRoot "deploy-single-store-listing.ps1"

if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw "config not found: $ConfigPath"
}

if (-not (Test-Path -LiteralPath $DeployScriptPath)) {
    throw "deploy script not found: $DeployScriptPath"
}

$rawConfig = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
$storeItems = @()

if ($null -ne $rawConfig.stores) {
    $storeItems = @($rawConfig.stores)
} elseif ($rawConfig -is [System.Collections.IEnumerable]) {
    $storeItems = @($rawConfig)
} else {
    throw "invalid config format: expected { `"stores`": [...] } or a JSON array"
}

$enabledStores = @($storeItems | Where-Object {
    $enabled = $_.enabled
    if ($null -eq $enabled) { return $true }
    return [bool]$enabled
})

if ($enabledStores.Count -eq 0) {
    Write-Warning "no enabled stores found in $ConfigPath"
}

$desiredDeploymentNames = New-Object System.Collections.Generic.List[string]

foreach ($store in $enabledStores) {
    if ($null -eq $store.storeId) {
        throw "storeId is required for every enabled store entry"
    }
    if ([string]::IsNullOrWhiteSpace([string]$store.ownerNodeId)) {
        throw "ownerNodeId is required for storeId=$($store.storeId)"
    }

    $deploymentName = if ([string]::IsNullOrWhiteSpace([string]$store.deploymentName)) {
        "shein-listing-store-$($store.storeId)"
    } else {
        [string]$store.deploymentName
    }

    $desiredDeploymentNames.Add($deploymentName)

    $invokeArgs = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $DeployScriptPath,
        "-StoreId", [string]$store.storeId,
        "-OwnerNodeId", [string]$store.ownerNodeId,
        "-Tier", $(if ([string]::IsNullOrWhiteSpace([string]$store.tier)) { "heavy" } else { [string]$store.tier }),
        "-Namespace", $Namespace,
        "-DeploymentName", $deploymentName,
        "-RolloutTimeoutSeconds", [string]$RolloutTimeoutSeconds
    )

    if (-not [string]::IsNullOrWhiteSpace([string]$store.image)) {
        $invokeArgs += @("-Image", [string]$store.image)
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$store.configMapName)) {
        $invokeArgs += @("-ConfigMapName", [string]$store.configMapName)
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$store.excludeNode)) {
        $invokeArgs += @("-ExcludeNode", [string]$store.excludeNode)
    }
    if ($Apply) {
        $invokeArgs += "-Apply"
    }

    Write-Host ""
    Write-Host "Syncing store $($store.storeId) -> $deploymentName" -ForegroundColor Cyan
    & powershell @invokeArgs
    if ($LASTEXITCODE -ne 0) {
        throw "failed to sync storeId=$($store.storeId)"
    }
}

if (-not ($Apply -and $Prune)) {
    if (-not $Apply) {
        Write-Host ""
        Write-Host "Skipped prune because -Apply was not set." -ForegroundColor Yellow
    }
    return
}

$currentDeploymentNames = @(
    kubectl -n $Namespace get deployment -o jsonpath="{range .items[*]}{.metadata.name}{'\n'}{end}" 2>$null |
        Where-Object { $_ -like "shein-listing-store-*" } |
        ForEach-Object { $_.Trim() } |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
)

$staleDeploymentNames = @($currentDeploymentNames | Where-Object { $_ -notin $desiredDeploymentNames })

foreach ($staleDeploymentName in $staleDeploymentNames) {
    Write-Host ""
    Write-Host "Pruning stale deployment $staleDeploymentName" -ForegroundColor Yellow
    kubectl -n $Namespace delete deployment $staleDeploymentName
    if ($LASTEXITCODE -ne 0) {
        throw "failed to delete deployment/$staleDeploymentName"
    }
}
