param(
    [ValidateSet("Preflight", "Crawl", "EndToEnd")]
    [string]$Mode = "Preflight",
    [string]$ApiBaseUrl = "",
    [string]$TokenFile = "",
    [string]$Url = "",
    [long]$SourceAccountID = 0,
    [long]$SheinStoreID = 0,
    [string]$ConfirmCreateTask = "",
    [int]$TimeoutSec = 300,
    [int]$PollIntervalSec = 5,
    [switch]$TestOnly
)

$ErrorActionPreference = "Stop"
$script:AcceptanceRepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$script:AcceptanceApiBaseUrl = if ([string]::IsNullOrWhiteSpace($ApiBaseUrl)) {
    if ([string]::IsNullOrWhiteSpace($env:LISTINGKIT_API_BASE_URL)) { "http://localhost:8085" } else { $env:LISTINGKIT_API_BASE_URL }
} else {
    $ApiBaseUrl
}
$script:AcceptanceTokenFile = if ([string]::IsNullOrWhiteSpace($TokenFile)) {
    Join-Path $script:AcceptanceRepoRoot ".local\listingkit-api-token.txt"
} else {
    $TokenFile
}
$script:AcceptanceTimeoutSec = $TimeoutSec
$script:AcceptancePollIntervalSec = $PollIntervalSec

function Normalize-ListingKitToken {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }

    $normalized = $Value.Trim()
    if ($normalized.StartsWith("Bearer ", [System.StringComparison]::OrdinalIgnoreCase)) {
        return $normalized.Substring(7).Trim()
    }
    return $normalized
}

function Resolve-ListingKitToken {
    param([string]$Path = $script:AcceptanceTokenFile)

    $value = Normalize-ListingKitToken -Value $env:LISTINGKIT_API_TOKEN
    if ([string]::IsNullOrWhiteSpace($value) -and (Test-Path -LiteralPath $Path)) {
        $value = Normalize-ListingKitToken -Value (Get-Content -LiteralPath $Path -Raw)
    }
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "No ListingKit API token found; set LISTINGKIT_API_TOKEN or provide the standard token file."
    }
    return $value
}

function Get-EndpointPath {
    param([string]$Endpoint)

    try {
        return ([Uri]$Endpoint).AbsolutePath
    } catch {
        return "/unknown"
    }
}

function Get-RedactedRuntimeError {
    param(
        [int]$StatusCode = 0,
        [string]$Endpoint = "",
        [string]$RawBody = ""
    )

    $path = Get-EndpointPath -Endpoint $Endpoint
    if ($StatusCode -gt 0) {
        return "HTTP $StatusCode from $path"
    }
    return "Request failed for $path"
}

function Invoke-AcceptanceRequest {
    param(
        [ValidateSet("Get", "Post")]
        [string]$Method,
        [string]$Path,
        [string]$Token,
        [hashtable]$Body = $null,
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl,
        [int]$RequestTimeoutSec = $script:AcceptanceTimeoutSec
    )

    $endpoint = "$($BaseUrl.TrimEnd('/'))$Path"
    $headers = @{ Authorization = "Bearer $Token" }
    try {
        $params = @{
            Uri         = $endpoint
            Method      = $Method
            Headers     = $headers
            TimeoutSec  = $RequestTimeoutSec
            ErrorAction = "Stop"
        }
        if ($null -ne $Body) {
            $params.ContentType = "application/json"
            $params.Body = $Body | ConvertTo-Json -Depth 30 -Compress
        }
        return Invoke-RestMethod @params
    } catch {
        $statusCode = 0
        if ($null -ne $_.Exception.Response -and $null -ne $_.Exception.Response.StatusCode) {
            $statusCode = [int]$_.Exception.Response.StatusCode
        }
        throw (Get-RedactedRuntimeError -StatusCode $statusCode -Endpoint $endpoint -RawBody "")
    }
}

function Assert-TaskCreationConfirmation {
    param(
        [ValidateSet("Preflight", "Crawl", "EndToEnd")]
        [string]$Mode,
        [string]$Confirmation
    )

    if ($Mode -eq "Preflight") {
        return
    }
    if ($Confirmation -ne "CREATE-1688-TASK") {
        throw "-ConfirmCreateTask CREATE-1688-TASK is required for $Mode mode."
    }
}

function Get-ResponseData {
    param([object]$Response)

    if ($null -eq $Response) {
        return $null
    }
    if ($null -ne $Response.data) {
        return $Response.data
    }
    return $Response
}

function New-ListingKitHandoffPayload {
    param(
        [hashtable]$ProductData,
        [long]$SourceAccountID,
        [long]$SheinStoreID,
        [string]$CrawlerTaskID
    )

    if ($null -eq $ProductData) {
        throw "crawler product_data is required"
    }
    if ([string]::IsNullOrWhiteSpace([string]$ProductData.url)) {
        throw "crawler product_data.url is required"
    }

    return [ordered]@{
        url               = [string]$ProductData.url
        product           = $ProductData
        source_run_id     = $CrawlerTaskID
        request_id        = "1688-runtime-$CrawlerTaskID"
        source_account_id = $SourceAccountID
        platforms         = @("shein")
        shein_store_id    = $SheinStoreID
    }
}

function Invoke-Preflight {
    param(
        [string]$Token,
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl
    )

    $checks = @(
        @{ Path = "/health"; Authenticated = $false },
        @{ Path = "/readyz"; Authenticated = $false },
        @{ Path = "/api/v1/listing-kits/settings-health"; Authenticated = $true }
    )
    foreach ($check in $checks) {
        $requestToken = if ($check.Authenticated) { $Token } else { "" }
        Invoke-AcceptanceRequest -Method Get -Path $check.Path -Token $requestToken -BaseUrl $BaseUrl | Out-Null
        Write-Output ("PASS GET {0}" -f $check.Path)
    }
}

function Invoke-Crawl {
    param(
        [string]$Url,
        [long]$SourceAccountID,
        [string]$Confirmation,
        [string]$Token = "",
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl,
        [int]$RequestTimeoutSec = $script:AcceptanceTimeoutSec,
        [int]$PollIntervalSec = $script:AcceptancePollIntervalSec
    )

    Assert-TaskCreationConfirmation -Mode "Crawl" -Confirmation $Confirmation
    if ([string]::IsNullOrWhiteSpace($Url)) { throw "-Url is required" }
    if ($SourceAccountID -le 0) { throw "-SourceAccountID must be positive" }
    if ($RequestTimeoutSec -le 0) { throw "-TimeoutSec must be positive" }
    if ($PollIntervalSec -lt 0) { throw "-PollIntervalSec must not be negative" }

    $submitted = Invoke-AcceptanceRequest -Method Post -Path "/api/v1/crawl" -Token $Token -BaseUrl $BaseUrl -RequestTimeoutSec $RequestTimeoutSec -Body @{
        url               = $Url
        source_account_id = $SourceAccountID
    }
    $submittedData = Get-ResponseData -Response $submitted
    $taskID = [string]$submittedData.task_id
    if ([string]::IsNullOrWhiteSpace($taskID)) { throw "crawler response did not contain a task id" }

    $deadline = [DateTime]::UtcNow.AddSeconds($RequestTimeoutSec)
    while ($true) {
        $response = Invoke-AcceptanceRequest -Method Get -Path "/api/v1/tasks/$([Uri]::EscapeDataString($taskID))" -Token $Token -BaseUrl $BaseUrl -RequestTimeoutSec $RequestTimeoutSec
        $data = Get-ResponseData -Response $response
        $status = [string]$data.status
        if ($status -eq "success") {
            if ($null -eq $data.product_data) { throw "crawler task $taskID succeeded without product_data" }
            return [pscustomobject]@{ TaskID = $taskID; Status = $status; ProductData = $data.product_data }
        }
        if ($status -eq "failed") {
            throw "crawler task $taskID failed"
        }
        if ([DateTime]::UtcNow -ge $deadline) {
            throw "crawler task $taskID timed out"
        }
        if ($PollIntervalSec -gt 0) { Start-Sleep -Seconds $PollIntervalSec }
    }
}

function Invoke-EndToEnd {
    param(
        [string]$Url,
        [long]$SourceAccountID,
        [long]$SheinStoreID,
        [string]$Confirmation,
        [string]$Token = "",
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl,
        [int]$RequestTimeoutSec = $script:AcceptanceTimeoutSec,
        [int]$PollIntervalSec = $script:AcceptancePollIntervalSec
    )

    Assert-TaskCreationConfirmation -Mode "EndToEnd" -Confirmation $Confirmation
    if ($SheinStoreID -le 0) { throw "-SheinStoreID must be positive for EndToEnd mode" }
    $crawler = Invoke-Crawl -Url $Url -SourceAccountID $SourceAccountID -Confirmation $Confirmation -Token $Token -BaseUrl $BaseUrl -RequestTimeoutSec $RequestTimeoutSec -PollIntervalSec $PollIntervalSec
    $payload = New-ListingKitHandoffPayload -ProductData $crawler.ProductData -SourceAccountID $SourceAccountID -SheinStoreID $SheinStoreID -CrawlerTaskID $crawler.TaskID
    $response = Invoke-AcceptanceRequest -Method Post -Path "/api/v1/product-sourcing/1688/listingkit/tasks" -Token $Token -BaseUrl $BaseUrl -RequestTimeoutSec $RequestTimeoutSec -Body $payload
    return [pscustomobject]@{ CrawlerTaskID = $crawler.TaskID; CrawlerStatus = $crawler.Status; Handoff = $response }
}

function Invoke-Main {
    $token = Resolve-ListingKitToken
    if ($Mode -eq "Preflight") {
        Invoke-Preflight -Token $token
        return
    }
    if ($Mode -eq "Crawl") {
        $result = Invoke-Crawl -Url $Url -SourceAccountID $SourceAccountID -Confirmation $ConfirmCreateTask -Token $token
        Write-Output ("PASS CRAWL task_id={0} status={1}" -f $result.TaskID, $result.Status)
        return
    }
    $result = Invoke-EndToEnd -Url $Url -SourceAccountID $SourceAccountID -SheinStoreID $SheinStoreID -Confirmation $ConfirmCreateTask -Token $token
    $handoffData = Get-ResponseData -Response $result.Handoff
    $sourceIdentity = $handoffData.source_identity
    Write-Output ("PASS END_TO_END crawler_task_id={0} crawler_status={1} listingkit_task_id={2} source_id={3} source_key={4} product_url={5}" -f `
        $result.CrawlerTaskID,
        $result.CrawlerStatus,
        [string]$handoffData.task_id,
        [string]$sourceIdentity.id,
        [string]$sourceIdentity.key,
        [string]$handoffData.product_url)
}

if (-not $TestOnly) {
    try {
        Invoke-Main
        exit 0
    } catch {
        Write-Error $_.Exception.Message
        exit 1
    }
}
