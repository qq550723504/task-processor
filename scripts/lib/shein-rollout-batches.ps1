function Get-RolloutBatches {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [int]$Replicas,
        [Parameter(Mandatory = $true)]
        [int]$BatchSize,
        [switch]$CanaryOnly
    )

    if ($Replicas -le 0) {
        throw "Replicas must be > 0"
    }
    if ($BatchSize -le 0) {
        throw "BatchSize must be > 0"
    }

    if ($CanaryOnly) {
        return [pscustomobject]@{
            Partition = $Replicas - 1
            Ordinals  = @($Replicas - 1)
        }
    }

    $effectiveBatchSize = [Math]::Min($BatchSize, $Replicas)
    $batches = @()
    for ($start = $Replicas; $start -gt 0; $start -= $effectiveBatchSize) {
        $partition = [Math]::Max(0, $start - $effectiveBatchSize)
        $ordinals = @()
        for ($ordinal = $start - 1; $ordinal -ge $partition; $ordinal--) {
            $ordinals += $ordinal
        }
        $batches += [pscustomobject]@{
            Partition = $partition
            Ordinals  = $ordinals
        }
    }
    return $batches
}
