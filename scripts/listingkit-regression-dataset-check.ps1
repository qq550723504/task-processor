param(
    [string]$ApiBaseUrl = "",
    [string]$DatasetPath = "docs/product/validation/listingkit-production-regression-dataset.example.json",
    [string]$TokenFile = "",
    [switch]$ValidateOnly,
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Normalize-BearerToken {
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

function Read-ListingKitToken {
    param([string]$Path)

    $environmentToken = Normalize-BearerToken -Value $env:LISTINGKIT_API_TOKEN
    if (-not [string]::IsNullOrWhiteSpace($environmentToken)) {
        return $environmentToken
    }
    if (Test-Path -LiteralPath $Path) {
        return Normalize-BearerToken -Value (Get-Content -LiteralPath $Path -Raw)
    }
    return ""
}

function Write-CheckResult {
    param(
        [string]$CaseID,
        [bool]$Ready,
        [string]$SubmitMode
    )

    if (-not $Quiet) {
        Write-Host "PASS $CaseID (ready=$Ready, submit_mode=$SubmitMode)" -ForegroundColor Green
    }
}

function Test-RegressionDatasetManifest {
    param([object]$Dataset)

    if ($Dataset.schema_version -ne 1) {
        throw "Unsupported dataset schema_version: $($Dataset.schema_version)"
    }

    $cases = @($Dataset.cases)
    if ($cases.Count -eq 0) {
        throw "Regression dataset manifest must contain at least one case"
    }

    $caseIDs = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    foreach ($case in $cases) {
        $caseID = ([string]$case.id).Trim()
        $taskID = ([string]$case.task_id).Trim()
        $platform = ([string]$case.platform).Trim().ToLowerInvariant()
        $submitMode = ([string]$case.expected_submit_mode).Trim().ToLowerInvariant()

        if ([string]::IsNullOrWhiteSpace($caseID) -or [string]::IsNullOrWhiteSpace($taskID)) {
            throw "Each regression case requires id and task_id"
        }
        if (-not $caseIDs.Add($caseID)) {
            throw "Regression dataset case IDs must be unique: $caseID"
        }
        if ($taskID -match '^REPLACE_WITH_') {
            throw "Regression case $caseID still has a placeholder task_id"
        }
        if ($platform -ne "shein") {
            throw "Regression case $caseID uses unsupported platform: $platform"
        }
        if ($case.expected_preview_ready -isnot [bool]) {
            throw "Regression case $caseID expected_preview_ready must be a boolean"
        }
        if ($submitMode -notin @("save_draft", "publish")) {
            throw "Regression case $caseID expected_submit_mode must be save_draft or publish"
        }
    }

    return $cases.Count
}

$repoRoot = Get-RepoRoot
if ([string]::IsNullOrWhiteSpace($ApiBaseUrl)) {
    $ApiBaseUrl = if ([string]::IsNullOrWhiteSpace($env:LISTINGKIT_API_BASE_URL)) {
        "https://pod.shuomiai.com"
    } else {
        $env:LISTINGKIT_API_BASE_URL
    }
}
if ([string]::IsNullOrWhiteSpace($TokenFile)) {
    $TokenFile = Join-Path $repoRoot ".local\listingkit-api-token.txt"
}
if (-not [System.IO.Path]::IsPathRooted($DatasetPath)) {
    $DatasetPath = Join-Path $repoRoot $DatasetPath
}
if (-not (Test-Path -LiteralPath $DatasetPath)) {
    throw "Regression dataset manifest was not found: $DatasetPath"
}

$dataset = Get-Content -LiteralPath $DatasetPath -Raw | ConvertFrom-Json
$caseCount = Test-RegressionDatasetManifest -Dataset $dataset
if ($ValidateOnly) {
    if (-not $Quiet) {
        Write-Host "ListingKit regression dataset manifest is valid: $caseCount case(s)." -ForegroundColor Green
    }
    exit 0
}

$token = Read-ListingKitToken -Path $TokenFile
if ([string]::IsNullOrWhiteSpace($token)) {
    Write-Host "No ListingKit API token found. Set LISTINGKIT_API_TOKEN or use scripts\listingkit-save-token.ps1."
    exit 2
}

$headers = @{ Authorization = "Bearer $token" }
$baseUrl = $ApiBaseUrl.TrimEnd("/")
$failures = @()

foreach ($case in @($dataset.cases)) {
    $caseID = [string]$case.id
    $taskID = [string]$case.task_id
    $platform = [string]$case.platform
    if ([string]::IsNullOrWhiteSpace($caseID) -or [string]::IsNullOrWhiteSpace($taskID) -or [string]::IsNullOrWhiteSpace($platform)) {
        $failures += "Each regression case requires id, task_id, and platform"
        continue
    }

    $previewURI = "{0}/api/v1/listing-kits/tasks/{1}/preview?platform={2}" -f $baseUrl, [Uri]::EscapeDataString($taskID), [Uri]::EscapeDataString($platform)
    try {
        $response = Invoke-WebRequest -Uri $previewURI -Headers $headers -Method Get -UseBasicParsing -TimeoutSec 30
        $preview = $response.Content | ConvertFrom-Json
    } catch {
        $status = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { 0 }
        $failures += "${caseID}: preview request failed (HTTP $status)"
        continue
    }

    if ([string]$preview.task_id -ne $taskID) {
        $failures += "${caseID}: preview returned an unexpected task_id"
        continue
    }

    $ready = $null -ne $preview.shein -and $null -ne $preview.shein.submit_readiness -and [bool]$preview.shein.submit_readiness.ready
    if ($ready -ne [bool]$case.expected_preview_ready) {
        $failures += "${caseID}: readiness was $ready, expected $($case.expected_preview_ready)"
        continue
    }

    $submitMode = ""
    if ($null -ne $preview.shein -and $null -ne $preview.shein.final_review) {
        $submitMode = [string]$preview.shein.final_review.submit_mode
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$case.expected_submit_mode) -and $submitMode -ne [string]$case.expected_submit_mode) {
        $failures += "${caseID}: submit mode was $submitMode, expected $($case.expected_submit_mode)"
        continue
    }

    Write-CheckResult -CaseID $caseID -Ready $ready -SubmitMode $submitMode
}

if ($failures.Count -gt 0) {
    $failures | ForEach-Object { Write-Host "FAIL $_" -ForegroundColor Red }
    exit 1
}

if (-not $Quiet) {
    Write-Host "ListingKit regression dataset check passed: $(@($dataset.cases).Count) case(s)." -ForegroundColor Green
}
