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
    [switch]$UseDeviceAuthorization,
    [string]$IssuerURL = "",
    [string]$ClientID = "",
    [string]$ExpectedTenantID = "",
    [switch]$OpenBrowser,
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
. (Join-Path $PSScriptRoot "lib\listingkit-device-auth.ps1")

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

function Resolve-AcceptanceToken {
    if (-not $UseDeviceAuthorization) {
        return Resolve-ListingKitToken
    }
    if ([string]::IsNullOrWhiteSpace($IssuerURL) -or
        [string]::IsNullOrWhiteSpace($ClientID) -or
        [string]::IsNullOrWhiteSpace($ExpectedTenantID)) {
        throw "-IssuerURL, -ClientID, and -ExpectedTenantID are required with -UseDeviceAuthorization"
    }
    Assert-ListingKitDeviceAPIBaseUrl -ApiBaseUrl $script:AcceptanceApiBaseUrl
    return Resolve-ListingKitDeviceToken -IssuerURL $IssuerURL -ClientID $ClientID -TimeoutSec $script:AcceptanceTimeoutSec -OpenBrowser:$OpenBrowser
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
        [System.Collections.IDictionary]$Body = $null,
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
    if ($Confirmation -cne "CREATE-1688-TASK") {
        throw "-ConfirmCreateTask CREATE-1688-TASK is required for $Mode mode."
    }
}

function Assert-1688OfferUrl {
    param([string]$Url)

    if ([string]::IsNullOrWhiteSpace($Url)) {
        throw "-Url must be a valid 1688 offer URL"
    }
    $candidate = $Url.Trim()
    if (-not $candidate.StartsWith("http://", [System.StringComparison]::OrdinalIgnoreCase) -and
        -not $candidate.StartsWith("https://", [System.StringComparison]::OrdinalIgnoreCase)) {
        $candidate = "https://$candidate"
    }
    try {
        $parsed = [Uri]$candidate
    } catch {
        throw "-Url must be a valid 1688 offer URL"
    }
    if ($parsed.Scheme -notin @("http", "https") -or
        $parsed.Host -notmatch "(^|\.)1688\.com$" -or
        $parsed.AbsolutePath -notmatch "(?i)(^|/)offer/\d+(?:\.html)?$") {
        throw "-Url must be a valid 1688 offer URL"
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

function Assert-AuthenticatedTenant {
    param(
        [string]$Token,
        [string]$ExpectedTenantID,
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl
    )

    $context = Get-ResponseData -Response (Invoke-AcceptanceRequest -Method Get -Path "/api/v1/listing-kits/auth-context" -Token $Token -BaseUrl $BaseUrl)
    if ([string]$context.tenant_id -cne $ExpectedTenantID) {
        throw "authenticated tenant does not match -ExpectedTenantID"
    }
    if ([string]::IsNullOrWhiteSpace([string]$context.user_id)) {
        throw "authenticated identity is incomplete"
    }
}

function Get-SourceIdentityEvidence {
    param([object]$SourceIdentity)

    if ($null -eq $SourceIdentity) {
        throw "handoff response did not contain source_identity"
    }
    $sourceType = [string]$SourceIdentity.SourceType
    $sourcePlatform = [string]$SourceIdentity.SourcePlatform
    $sourceID = [string]$SourceIdentity.SourceID
    $sourceVersion = [string]$SourceIdentity.SourceVersion
    if ([string]::IsNullOrWhiteSpace($sourceType) -or
        [string]::IsNullOrWhiteSpace($sourcePlatform) -or
        [string]::IsNullOrWhiteSpace($sourceID)) {
        throw "handoff response did not contain complete source_identity"
    }
    $sourceKeyParts = @($sourceType, $sourcePlatform, $sourceID)
    if (-not [string]::IsNullOrWhiteSpace($sourceVersion)) {
        $sourceKeyParts += @("version", $sourceVersion)
    }
    return [pscustomobject]@{
        SourceID = $sourceID
        SourceKey = $sourceKeyParts -join ":"
    }
}

function Get-RemainingRequestTimeoutSec {
    param(
        [DateTime]$Deadline,
        [string]$TaskID = ""
    )

    $remaining = ($Deadline - [DateTime]::UtcNow).TotalSeconds
    if ($remaining -le 0) {
        if ([string]::IsNullOrWhiteSpace($TaskID)) {
            throw "crawler task timed out"
        }
        throw "crawler task $TaskID timed out"
    }
    return [int][Math]::Ceiling($remaining)
}

function Get-CappedPollSleepSec {
    param(
        [int]$PollIntervalSec,
        [DateTime]$Deadline
    )

    if ($PollIntervalSec -le 0) {
        return 0
    }
    $remaining = ($Deadline - [DateTime]::UtcNow).TotalSeconds
    if ($remaining -le 0) {
        return 0
    }
    return [Math]::Min($PollIntervalSec, [int][Math]::Floor($remaining))
}

function New-ListingKitHandoffPayload {
    param(
        [object]$ProductData,
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

function Invoke-PublicPreflight {
    param(
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl
    )

    foreach ($path in @("/health", "/readyz")) {
        Invoke-AcceptanceRequest -Method Get -Path $path -Token "" -BaseUrl $BaseUrl | Out-Null
        Write-Output ("PASS GET {0}" -f $path)
    }
}

function Invoke-AuthenticatedPreflight {
    param(
        [string]$Token,
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl
    )

    $response = Invoke-AcceptanceRequest -Method Get -Path "/api/v1/listing-kits/settings-health" -Token $Token -BaseUrl $BaseUrl
    $health = Get-ResponseData -Response $response
    $healthStatus = ([string]$health.status).Trim().ToLowerInvariant()
    if ($healthStatus -ne "ready") {
        throw "settings-health status is '$healthStatus'; task creation is not allowed"
    }
    Write-Output "PASS GET /api/v1/listing-kits/settings-health"
}

function Invoke-Preflight {
    param(
        [string]$Token,
        [string]$BaseUrl = $script:AcceptanceApiBaseUrl,
        [string]$ExpectedTenantID = ""
    )

    if (-not [string]::IsNullOrWhiteSpace($ExpectedTenantID)) {
        Assert-AuthenticatedTenant -Token $Token -ExpectedTenantID $ExpectedTenantID -BaseUrl $BaseUrl
    }
    Invoke-PublicPreflight -BaseUrl $BaseUrl
    if ([string]::IsNullOrWhiteSpace($Token)) {
        throw "No ListingKit API token found; set LISTINGKIT_API_TOKEN or provide the standard token file."
    }
    Invoke-AuthenticatedPreflight -Token $Token -BaseUrl $BaseUrl
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
    Assert-1688OfferUrl -Url $Url
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
        $remainingTimeoutSec = Get-RemainingRequestTimeoutSec -Deadline $deadline -TaskID $taskID
        $response = Invoke-AcceptanceRequest -Method Get -Path "/api/v1/tasks/$([Uri]::EscapeDataString($taskID))" -Token $Token -BaseUrl $BaseUrl -RequestTimeoutSec $remainingTimeoutSec
        $data = Get-ResponseData -Response $response
        $responseTaskID = [string]$data.task_id
        if ([string]::IsNullOrWhiteSpace($responseTaskID)) { $responseTaskID = [string]$data.TaskID }
        if ([string]::IsNullOrWhiteSpace($responseTaskID)) {
            throw "crawler task $taskID response did not contain a task id"
        }
        if ($responseTaskID -cne $taskID) {
            throw "crawler task $taskID response task id '$responseTaskID' does not match"
        }
        $status = ([string]$data.status).Trim().ToLowerInvariant()
        if ($status -eq "success") {
            $productData = $data.product_data
            if ($null -eq $productData) { $productData = $data.ProductData }
            if ($null -eq $productData) { throw "crawler task $taskID succeeded without product_data" }
            return [pscustomobject]@{ TaskID = $taskID; Status = $status; ProductData = $productData }
        }
        if ($status -eq "failed") {
            throw "crawler task $taskID failed"
        }
        if ($status -ne "pending" -and $status -ne "processing") {
            throw "crawler task $taskID returned unexpected status '$status'"
        }
        $sleepSec = Get-CappedPollSleepSec -PollIntervalSec $PollIntervalSec -Deadline $deadline
        if ($sleepSec -gt 0) { Start-Sleep -Seconds $sleepSec }
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
    $handoffData = Get-ResponseData -Response $response
    if ([string]::IsNullOrWhiteSpace([string]$handoffData.task_id)) { throw "handoff response did not contain a task id" }
    return [pscustomobject]@{ CrawlerTaskID = $crawler.TaskID; CrawlerStatus = $crawler.Status; Handoff = $response }
}

function Invoke-Main {
    if ($Mode -eq "Preflight") {
        if ($UseDeviceAuthorization) {
            $token = Resolve-AcceptanceToken
            Invoke-Preflight -Token $token -ExpectedTenantID $ExpectedTenantID
            return
        }
        Invoke-PublicPreflight
        $token = Resolve-AcceptanceToken
        Invoke-AuthenticatedPreflight -Token $token
        return
    }
    $token = Resolve-AcceptanceToken
    if ($UseDeviceAuthorization) {
        Assert-AuthenticatedTenant -Token $token -ExpectedTenantID $ExpectedTenantID
    }
    if ($Mode -eq "Crawl") {
        $result = Invoke-Crawl -Url $Url -SourceAccountID $SourceAccountID -Confirmation $ConfirmCreateTask -Token $token
        Write-Output ("PASS CRAWL task_id={0} status={1}" -f $result.TaskID, $result.Status)
        return
    }
    $result = Invoke-EndToEnd -Url $Url -SourceAccountID $SourceAccountID -SheinStoreID $SheinStoreID -Confirmation $ConfirmCreateTask -Token $token
    $handoffData = Get-ResponseData -Response $result.Handoff
    $sourceEvidence = Get-SourceIdentityEvidence -SourceIdentity $handoffData.source_identity
    Write-Output ("PASS END_TO_END crawler_task_id={0} crawler_status={1} listingkit_task_id={2} source_id={3} source_key={4} product_url={5}" -f `
        $result.CrawlerTaskID,
        $result.CrawlerStatus,
        [string]$handoffData.task_id,
        $sourceEvidence.SourceID,
        $sourceEvidence.SourceKey,
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
