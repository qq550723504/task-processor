param(
    [string]$TokenFile = ""
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

    $normalized = ($Value | ForEach-Object { $_.Trim() })
    if ($normalized.StartsWith("Bearer ", [System.StringComparison]::OrdinalIgnoreCase)) {
        $normalized = $normalized.Substring(7).Trim()
    }
    return $normalized
}

if ([string]::IsNullOrWhiteSpace($TokenFile)) {
    $TokenFile = Get-DefaultTokenFile
}

$secureToken = Read-Host "Paste ListingKit API bearer token" -AsSecureString
$bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureToken)
$plainToken = ""
try {
    $plainToken = [System.Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
} finally {
    if ($bstr -ne [System.IntPtr]::Zero) {
        [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}

$token = Normalize-BearerToken -Value $plainToken
if ([string]::IsNullOrWhiteSpace($token)) {
    throw "Token is empty; nothing was saved."
}

$tokenPath = [System.IO.Path]::GetFullPath($TokenFile)
$tokenDir = Split-Path -Parent $tokenPath
if (-not (Test-Path -LiteralPath $tokenDir)) {
    New-Item -ItemType Directory -Path $tokenDir | Out-Null
}

[System.IO.File]::WriteAllText($tokenPath, $token, [System.Text.UTF8Encoding]::new($false))

Write-Host "Saved ListingKit API token to $tokenPath"
Write-Host "Run scripts\listingkit-auth-check.ps1 to verify it."
