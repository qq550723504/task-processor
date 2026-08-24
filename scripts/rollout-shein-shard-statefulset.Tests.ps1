$rolloutBatchesPath = Join-Path $PSScriptRoot "lib/shein-rollout-batches.ps1"
. $rolloutBatchesPath

Describe "Get-RolloutBatches" {
    It "returns only the highest ordinal for a 10-replica canary" {
        $batches = @(Get-RolloutBatches -Replicas 10 -BatchSize 4 -CanaryOnly)

        $batches.Count | Should Be 1
        $batches[0].Partition | Should Be 9
        $batches[0].Ordinals.Count | Should Be 1
        $batches[0].Ordinals[0] | Should Be 9
    }

    It "keeps the existing descending batches for normal rollout" {
        $batches = @(Get-RolloutBatches -Replicas 10 -BatchSize 4)

        $batches.Count | Should Be 3
        $batches[0].Partition | Should Be 6
        ($batches[0].Ordinals -join ',') | Should Be '9,8,7,6'
        $batches[1].Partition | Should Be 2
        ($batches[1].Ordinals -join ',') | Should Be '5,4,3,2'
        $batches[2].Partition | Should Be 0
        ($batches[2].Ordinals -join ',') | Should Be '1,0'
    }

    It "rejects non-positive replicas" {
        $thrown = $false
        try { Get-RolloutBatches -Replicas 0 -BatchSize 1 } catch { $thrown = $true }
        $thrown | Should Be $true
    }

    It "rejects non-positive batch size" {
        $thrown = $false
        try { Get-RolloutBatches -Replicas 1 -BatchSize 0 } catch { $thrown = $true }
        $thrown | Should Be $true
    }
}
