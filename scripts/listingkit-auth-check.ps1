param(
    [string]$ApiBaseUrl = "",
    [string]$TokenFile = "",
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Get-DefaultTokenFile {
    $repoRoot = Get-RepoRoot
    return (Join-Path $repoRoot ".local\listingkit-api-token.txt")
}

function Normalize-BearerToken {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }

    $normalized = $Value.Trim()
    if ($normalized.StartsWith("Bearer ", [System.StringComparison]::OrdinalIgnoreCase)) {
        $normalized = $normalized.Substring(7).Trim()
    }
    return $normalized
}

function Read-ListingKitToken {
    param([string]$Path)

    $envToken = Normalize-BearerToken -Value $env:LISTINGKIT_API_TOKEN
    if (-not [string]::IsNullOrWhiteSpace($envToken)) {
        return @{
            Token = $envToken
            Source = "LISTINGKIT_API_TOKEN"
        }
    }

    if (Test-Path -LiteralPath $Path) {
        $fileToken = Normalize-BearerToken -Value (Get-Content -LiteralPath $Path -Raw)
        if (-not [string]::IsNullOrWhiteSpace($fileToken)) {
            return @{
                Token = $fileToken
                Source = $Path
            }
        }
    }

    return @{
        Token = ""
        Source = ""
    }
}

function Write-Info {
    param([string]$Message)

    if (-not $Quiet) {
        Write-Host $Message
    }
}

if ([string]::IsNullOrWhiteSpace($ApiBaseUrl)) {
    if ([string]::IsNullOrWhiteSpace($env:LISTINGKIT_API_BASE_URL)) {
        $ApiBaseUrl = "http://localhost:8085"
    } else {
        $ApiBaseUrl = $env:LISTINGKIT_API_BASE_URL
    }
}

if ([string]::IsNullOrWhiteSpace($TokenFile)) {
    $TokenFile = Get-DefaultTokenFile
}

$tokenResult = Read-ListingKitToken -Path $TokenFile
if ([string]::IsNullOrWhiteSpace($tokenResult.Token)) {
    Write-Host "No ListingKit API token found."
    Write-Host "Set LISTINGKIT_API_TOKEN for this shell, or run scripts\listingkit-save-token.ps1 after copying a valid browser/API bearer token."
    exit 2
}

$base = $ApiBaseUrl.TrimEnd("/")
$uri = "$base/api/v1/listing-kits/settings-health"
$headers = @{
    Authorization = "Bearer $($tokenResult.Token)"
}

try {
    $response = Invoke-WebRequest -Uri $uri -Headers $headers -Method Get -UseBasicParsing -TimeoutSec 20
    Write-Info "ListingKit API token is valid. $uri returned HTTP $($response.StatusCode)."
    exit 0
} catch {
    $statusCode = $null
    $body = ""
    if ($_.Exception.Response) {
        $statusCode = [int]$_.Exception.Response.StatusCode
        try {
            $stream = $_.Exception.Response.GetResponseStream()
            if ($stream) {
                $reader = [System.IO.StreamReader]::new($stream)
                try {
                    $body = $reader.ReadToEnd()
                } finally {
                    $reader.Dispose()
                }
            }
        } catch {
            $body = ""
        }
    }

    if ($statusCode -eq 401 -or $statusCode -eq 403) {
        Write-Host "ListingKit API token was rejected by $uri with HTTP $statusCode."
        if (-not [string]::IsNullOrWhiteSpace($body)) {
            Write-Host $body
        }
        exit 3
    }

    if ($statusCode) {
        Write-Host "ListingKit API auth check reached $uri but returned HTTP $statusCode."
        if (-not [string]::IsNullOrWhiteSpace($body)) {
            Write-Host $body
        }
        exit 4
    }

    Write-Host "Could not reach ListingKit API at $uri."
    Write-Host $_.Exception.Message
    exit 5
}
