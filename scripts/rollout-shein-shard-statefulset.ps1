# Batch rollout the SHEIN shard StatefulSet using StatefulSet partition updates.
# Usage:
#   .\scripts\rollout-shein-shard-statefulset.ps1 -Image xuwei190/task-processor-shein-listing:tag
#   .\scripts\rollout-shein-shard-statefulset.ps1 -Image xuwei190/task-processor-shein-listing:tag -BatchSize 4

[CmdletBinding()]
param(
    [string]$Namespace = "task-processor",
    [string]$StatefulSetName = "shein-listing-shard",
    [string]$ContainerName = "shein-listing",
    [Parameter(Mandatory = $true)]
    [string]$Image,
    [int]$BatchSize = 4,
    [int]$BatchTimeoutSeconds = 600,
    [int]$PollSeconds = 5
)

$ErrorActionPreference = "Stop"

function Invoke-Step {
    param(
        [string]$Title,
        [scriptblock]$Action
    )

    Write-Host ""
    Write-Host $Title -ForegroundColor Yellow
    . $Action
}

function Get-JsonPathValue {
    param(
        [string]$Resource,
        [string]$JsonPath
    )

    $value = kubectl -n $Namespace get $Resource -o "jsonpath=$JsonPath"
    if ($LASTEXITCODE -ne 0) {
        throw "kubectl get $Resource failed"
    }
    return [string]$value
}

function Set-Partition {
    param([int]$Partition)

    $patch = @{
        spec = @{
            updateStrategy = @{
                type = "RollingUpdate"
                rollingUpdate = @{
                    partition = $Partition
                }
            }
        }
    } | ConvertTo-Json -Depth 6 -Compress

    $patchFile = [System.IO.Path]::GetTempFileName()
    try {
        Set-Content -LiteralPath $patchFile -Value $patch -Encoding UTF8
        kubectl -n $Namespace patch "statefulset/$StatefulSetName" --type merge --patch-file $patchFile
        if ($LASTEXITCODE -ne 0) {
            throw "failed to patch partition=$Partition"
        }
    }
    finally {
        Remove-Item -LiteralPath $patchFile -Force -ErrorAction SilentlyContinue
    }
}

function Wait-BatchReady {
    param(
        [int[]]$Ordinals,
        [string]$ExpectedRevision
    )

    $deadline = (Get-Date).AddSeconds($BatchTimeoutSeconds)

    while ((Get-Date) -lt $deadline) {
        $pending = @()

        foreach ($ordinal in $Ordinals) {
            $podName = "$StatefulSetName-$ordinal"
            $revision = kubectl -n $Namespace get "pod/$podName" -o "jsonpath={.metadata.labels.controller-revision-hash}" 2>$null
            $ready = kubectl -n $Namespace get "pod/$podName" -o "jsonpath={.status.containerStatuses[0].ready}" 2>$null
            $deleting = kubectl -n $Namespace get "pod/$podName" -o "jsonpath={.metadata.deletionTimestamp}" 2>$null

            if ($LASTEXITCODE -ne 0 -or $revision -ne $ExpectedRevision -or $ready -ne "true" -or -not [string]::IsNullOrWhiteSpace($deleting)) {
                $pending += $podName
            }
        }

        if ($pending.Count -eq 0) {
            return
        }

        Write-Host ("Waiting for batch: " + ($pending -join ", ")) -ForegroundColor DarkYellow
        Start-Sleep -Seconds $PollSeconds
    }

    throw "batch rollout timed out for ordinals: $($Ordinals -join ', ')"
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SHEIN Shard StatefulSet Batch Rollout" -ForegroundColor Cyan
Write-Host "  StatefulSet: $StatefulSetName" -ForegroundColor Cyan
Write-Host "  Image: $Image" -ForegroundColor Cyan
Write-Host "  Batch size: $BatchSize" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$replicas = [int](Get-JsonPathValue -Resource "statefulset/$StatefulSetName" -JsonPath "{.spec.replicas}")
if ($replicas -le 0) {
    throw "invalid replicas for ${StatefulSetName}: $replicas"
}
if ($BatchSize -le 0) {
    throw "BatchSize must be > 0"
}
if ($BatchSize -gt $replicas) {
    $BatchSize = $replicas
}

Invoke-Step "[1/4] Freezing rollout at partition=$replicas ..." {
    Set-Partition -Partition $replicas
}

Invoke-Step "[2/4] Updating StatefulSet image..." {
    kubectl -n $Namespace set image "statefulset/$StatefulSetName" "$ContainerName=$Image"
    if ($LASTEXITCODE -ne 0) {
        throw "kubectl set image failed for statefulset/$StatefulSetName"
    }
}

$script:updateRevision = ""
Invoke-Step "[3/4] Reading update revision..." {
    $deadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $deadline) {
        $script:updateRevision = Get-JsonPathValue -Resource "statefulset/$StatefulSetName" -JsonPath "{.status.updateRevision}"
        if (-not [string]::IsNullOrWhiteSpace($script:updateRevision)) {
            break
        }
        Start-Sleep -Seconds 2
    }

    if ([string]::IsNullOrWhiteSpace($script:updateRevision)) {
        throw "failed to resolve updateRevision"
    }

    Write-Host "updateRevision=$script:updateRevision" -ForegroundColor Green
}

Invoke-Step "[4/4] Rolling out batches..." {
    for ($start = $replicas; $start -gt 0; $start -= $BatchSize) {
        $partition = [Math]::Max(0, $start - $BatchSize)
        $batchOrdinals = @()
        for ($ordinal = $start - 1; $ordinal -ge $partition; $ordinal--) {
            $batchOrdinals += $ordinal
        }

        Write-Host "Updating ordinals: $($batchOrdinals -join ', ') with partition=$partition" -ForegroundColor Cyan
        Set-Partition -Partition $partition
        Wait-BatchReady -Ordinals $batchOrdinals -ExpectedRevision $script:updateRevision
    }
}

Write-Host ""
Write-Host "Batch rollout finished successfully." -ForegroundColor Green
