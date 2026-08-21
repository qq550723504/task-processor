param(
    [string]$ApiBaseUrl = "http://127.0.0.1:18086",
    [string]$IssuerURL = $env:LISTINGKIT_ZITADEL_ISSUER_URL,
    [string]$ClientID = $env:LISTINGKIT_ZITADEL_CLIENT_ID,
    [string]$ProjectID = $env:TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID,
    [string]$Url,
    [string]$Confirm = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($IssuerURL) -or [string]::IsNullOrWhiteSpace($ClientID) -or [string]::IsNullOrWhiteSpace($ProjectID)) {
    throw "IssuerURL, ClientID, and ProjectID are required"
}

$api = [Uri]$ApiBaseUrl
if ($api.Scheme -ne "https" -and -not ($api.Scheme -eq "http" -and @("localhost", "127.0.0.1", "::1") -contains $api.Host)) {
    throw "-ApiBaseUrl must use HTTPS unless it is a literal loopback test endpoint"
}
$arguments = @(
    "run", "./cmd/1688-local-agent",
    "-api-base-url", $ApiBaseUrl,
    "-issuer-url", $IssuerURL,
    "-client-id", $ClientID,
    "-project-id", $ProjectID
)
if (-not [string]::IsNullOrWhiteSpace($Url)) {
    if ($Confirm -cne "CREATE-LOCAL-AGENT-JOB") {
        throw "Set -Confirm CREATE-LOCAL-AGENT-JOB to create a local-agent job."
    }
    $offer = [Uri]$Url
    if ($offer.Scheme -cne "https" -or $offer.Host -cne "detail.1688.com" -or $offer.AbsolutePath -notmatch '^/offer/[0-9]+\.html$' -or $offer.Query -ne "" -or $offer.Fragment -ne "") {
        throw "-Url must be a public HTTPS detail.1688.com offer URL"
    }
    $arguments += @("-url", $Url)
} elseif (-not [string]::IsNullOrWhiteSpace($Confirm)) {
    throw "-Confirm is only valid when -Url creates a local-agent job."
}

& go @arguments
if ($LASTEXITCODE -ne 0) {
    throw "1688 local agent failed"
}
