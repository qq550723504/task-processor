[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [long]$StoreId,

    [Parameter(Mandatory = $true)]
    [string]$OwnerNodeId,

    [ValidateSet("lite", "heavy")]
    [string]$Tier = "heavy",

    [string]$Namespace = "task-processor",
    [string]$DeploymentName = "",
    [string]$Image = "",
    [string]$ConfigMapName = "",
    [string]$ExcludeNode = "",
    [switch]$Apply,
    [int]$RolloutTimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent $PSScriptRoot
$TemplatesRoot = Join-Path $RepoRoot "deployments\kubernetes\shein-listing\templates"
$OverlaysRoot = Join-Path $RepoRoot "deployments\kubernetes\shein-listing\overlays"
$TemplatePath = Join-Path $TemplatesRoot "single-store-deployment.yaml.tpl"
$OverlayDir = Join-Path $OverlaysRoot ("prod-single-store-{0}" -f $StoreId)
$OutputPath = Join-Path $OverlayDir "deployment.yaml"

if (-not (Test-Path -LiteralPath $TemplatePath)) {
    throw "template not found: $TemplatePath"
}

if ([string]::IsNullOrWhiteSpace($DeploymentName)) {
    $DeploymentName = "shein-listing-store-$StoreId"
}

if ([string]::IsNullOrWhiteSpace($ConfigMapName)) {
    $ConfigMapName = "shein-listing-config-$Tier"
}

function Get-DaemonSetImage {
    param(
        [string]$TierName,
        [string]$KubeNamespace
    )

    $daemonSetName = "shein-listing-$TierName"
    $imageValue = kubectl -n $KubeNamespace get daemonset $daemonSetName -o jsonpath="{.spec.template.spec.containers[0].image}" 2>$null
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($imageValue)) {
        throw "failed to read image from daemonset/$daemonSetName in namespace $KubeNamespace"
    }
    return $imageValue.Trim()
}

function Get-ResourceProfile {
    param([string]$TierName)

    if ($TierName -eq "lite") {
        return @{
            CpuRequest    = "500m"
            MemoryRequest = "1500Mi"
            CpuLimit      = "1800m"
            MemoryLimit   = "3Gi"
        }
    }

    return @{
        CpuRequest    = "1"
        MemoryRequest = "2500Mi"
        CpuLimit      = "3"
        MemoryLimit   = "5Gi"
    }
}

function New-AffinityBlock {
    param(
        [string]$TierName,
        [string]$ExcludedNode
    )

    $lines = @(
        "      affinity:",
        "        nodeAffinity:",
        "          requiredDuringSchedulingIgnoredDuringExecution:",
        "            nodeSelectorTerms:",
        "              - matchExpressions:",
        "                  - key: node-role.kubernetes.io/agent",
        "                    operator: In",
        "                    values: [""true""]",
        "                  - key: task-processor/crawler-tier",
        "                    operator: In",
        "                    values: [""$TierName""]"
    )

    if (-not [string]::IsNullOrWhiteSpace($ExcludedNode)) {
        $lines += @(
            "                  - key: kubernetes.io/hostname",
            "                    operator: NotIn",
            "                    values: [""$ExcludedNode""]"
        )
    }

    return ($lines -join "`n")
}

if ([string]::IsNullOrWhiteSpace($Image)) {
    $Image = Get-DaemonSetImage -TierName $Tier -KubeNamespace $Namespace
}

$resourceProfile = Get-ResourceProfile -TierName $Tier
$affinityBlock = New-AffinityBlock -TierName $Tier -ExcludedNode $ExcludeNode
$template = Get-Content -LiteralPath $TemplatePath -Raw

$rendered = $template.
    Replace("__DEPLOYMENT_NAME__", $DeploymentName).
    Replace("__NAMESPACE__", $Namespace).
    Replace("__STORE_ID__", [string]$StoreId).
    Replace("__OWNER_NODE_ID__", $OwnerNodeId).
    Replace("__IMAGE__", $Image).
    Replace("__CONFIGMAP_NAME__", $ConfigMapName).
    Replace("__CPU_REQUEST__", $resourceProfile.CpuRequest).
    Replace("__MEMORY_REQUEST__", $resourceProfile.MemoryRequest).
    Replace("__CPU_LIMIT__", $resourceProfile.CpuLimit).
    Replace("__MEMORY_LIMIT__", $resourceProfile.MemoryLimit).
    Replace("__AFFINITY_BLOCK__", $affinityBlock)

New-Item -ItemType Directory -Path $OverlayDir -Force | Out-Null
Set-Content -LiteralPath $OutputPath -Value $rendered -Encoding UTF8

Write-Host "Generated deployment manifest:" -ForegroundColor Green
Write-Host "  $OutputPath" -ForegroundColor Green
Write-Host "  StoreId: $StoreId" -ForegroundColor Green
Write-Host "  OwnerNodeId: $OwnerNodeId" -ForegroundColor Green
Write-Host "  Tier: $Tier" -ForegroundColor Green
Write-Host "  Image: $Image" -ForegroundColor Green
if (-not [string]::IsNullOrWhiteSpace($ExcludeNode)) {
    Write-Host "  ExcludeNode: $ExcludeNode" -ForegroundColor Green
}

if (-not $Apply) {
    Write-Host ""
    Write-Host "Skipped kubectl apply. Re-run with -Apply to deploy." -ForegroundColor Yellow
    return
}

kubectl apply -f $OutputPath
if ($LASTEXITCODE -ne 0) {
    throw "kubectl apply failed for $OutputPath"
}

kubectl -n $Namespace rollout status deployment/$DeploymentName --timeout=([string]::Format("{0}s", $RolloutTimeoutSeconds))
if ($LASTEXITCODE -ne 0) {
    throw "rollout failed for deployment/$DeploymentName"
}

kubectl -n $Namespace get pods -l "app=$DeploymentName" -o wide
if ($LASTEXITCODE -ne 0) {
    throw "failed to get pods for app=$DeploymentName"
}

kubectl -n $Namespace logs deployment/$DeploymentName --tail=80
if ($LASTEXITCODE -ne 0) {
    throw "failed to read logs for deployment/$DeploymentName"
}
