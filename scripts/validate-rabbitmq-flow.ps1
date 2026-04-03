param(
    [Parameter(Mandatory = $true)]
    [string]$ManagementBaseUrl,

    [Parameter(Mandatory = $true)]
    [string]$ProcessorApiBaseUrl,

    [Parameter(Mandatory = $true)]
    [string]$ConsumerBaseUrl,

    [string]$AdminToken = "",

    [long]$TaskId = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function New-Headers {
    param([string]$Token)

    if ([string]::IsNullOrWhiteSpace($Token)) {
        return @{}
    }

    return @{
        Authorization = "Bearer $Token"
    }
}

function Invoke-JsonGet {
    param(
        [string]$Name,
        [string]$Url,
        [hashtable]$Headers
    )

    Write-Host "==> $Name"
    Write-Host "    GET $Url"
    try {
        $response = Invoke-RestMethod -Method Get -Uri $Url -Headers $Headers -TimeoutSec 15
        return [pscustomobject]@{
            Name    = $Name
            Url     = $Url
            Success = $true
            Data    = $response
            Error   = $null
        }
    } catch {
        return [pscustomobject]@{
            Name    = $Name
            Url     = $Url
            Success = $false
            Data    = $null
            Error   = $_.Exception.Message
        }
    }
}

function Show-Result {
    param($Result)

    if ($Result.Success) {
        Write-Host "    OK"
        $Result.Data | ConvertTo-Json -Depth 8
    } else {
        Write-Host "    FAILED: $($Result.Error)" -ForegroundColor Yellow
    }
    Write-Host ""
}

$adminHeaders = New-Headers -Token $AdminToken

$checks = @(
    @{ Name = "Management Health"; Url = "$ManagementBaseUrl/admin-api/listing/task-management/health"; Headers = $adminHeaders },
    @{ Name = "Management Metrics"; Url = "$ManagementBaseUrl/admin-api/listing/task-management/metrics"; Headers = $adminHeaders },
    @{ Name = "Task Processor Nodes"; Url = "$ManagementBaseUrl/admin-api/listing/task-management/task-processor-nodes"; Headers = $adminHeaders },
    @{ Name = "Processor Local Health"; Url = "$ProcessorApiBaseUrl/api/v1/management/tasks/health"; Headers = @{} },
    @{ Name = "Consumer Health"; Url = "$ConsumerBaseUrl/health"; Headers = @{} },
    @{ Name = "Consumer Stats"; Url = "$ConsumerBaseUrl/stats"; Headers = @{} }
)

if ($TaskId -gt 0) {
    $checks += @{ Name = "Processor Task Status"; Url = "$ProcessorApiBaseUrl/api/v1/management/tasks/$TaskId/status"; Headers = @{} }
}

$results = foreach ($check in $checks) {
    Invoke-JsonGet -Name $check.Name -Url $check.Url -Headers $check.Headers
}

foreach ($result in $results) {
    Show-Result -Result $result
}

$failed = @($results | Where-Object { -not $_.Success })
if ($failed.Count -gt 0) {
    Write-Host "Validation finished with failures: $($failed.Name -join ', ')" -ForegroundColor Yellow
    exit 1
}

Write-Host "Validation finished successfully." -ForegroundColor Green
