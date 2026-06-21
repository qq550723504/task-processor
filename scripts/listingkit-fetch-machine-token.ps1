param(
    [string]$EnvFile = ".env",
    [string]$IssuerUrl = "",
    [string]$TokenUrl = "",
    [string]$ClientId = "",
    [string]$ClientSecret = "",
    [string]$Scopes = "",
    [string]$TokenFile = "",
    [string]$ApiBaseUrl = "",
    [switch]$SkipAuthCheck
)

$ErrorActionPreference = "Stop"

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Get-DefaultTokenFile {
    $repoRoot = Get-RepoRoot
    return (Join-Path $repoRoot ".local\listingkit-api-token.txt")
}

function Read-DotEnvFile {
    param([string]$Path)

    $values = @{}
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return $values
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ([string]::IsNullOrWhiteSpace($trimmed) -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed -notmatch "^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$") {
            continue
        }

        $name = $Matches[1]
        $value = $Matches[2].Trim()
        if (
            ($value.StartsWith('"') -and $value.EndsWith('"')) -or
            ($value.StartsWith("'") -and $value.EndsWith("'"))
        ) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $values[$name] = $value
    }

    return $values
}

function Get-ConfigValue {
    param(
        [hashtable]$DotEnv,
        [string]$ExplicitValue,
        [string[]]$Names,
        [string]$DefaultValue = ""
    )

    if (-not [string]::IsNullOrWhiteSpace($ExplicitValue)) {
        return $ExplicitValue.Trim()
    }

    foreach ($name in $Names) {
        $envValue = [Environment]::GetEnvironmentVariable($name)
        if (-not [string]::IsNullOrWhiteSpace($envValue)) {
            return $envValue.Trim()
        }
        if ($DotEnv.ContainsKey($name) -and -not [string]::IsNullOrWhiteSpace($DotEnv[$name])) {
            return [string]$DotEnv[$name].Trim()
        }
    }

    return $DefaultValue
}

function Require-ConfigValue {
    param(
        [string]$Name,
        [string]$Value
    )

    if ([string]::IsNullOrWhiteSpace($Value)) {
        throw "Missing required ListingKit machine auth config: $Name"
    }
}

function Ensure-OpenIdScope {
    param([string]$Value)

    $parts = @($Value -split "\s+" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($parts -notcontains "openid") {
        $parts = @("openid") + $parts
    }
    if ($parts.Count -eq 0) {
        return "openid profile"
    }
    return ($parts -join " ")
}

if ([string]::IsNullOrWhiteSpace($TokenFile)) {
    $TokenFile = Get-DefaultTokenFile
}

$repoRoot = Get-RepoRoot
$envFilePath = if ([System.IO.Path]::IsPathRooted($EnvFile)) {
    $EnvFile
} else {
    Join-Path $repoRoot $EnvFile
}
$dotEnv = Read-DotEnvFile -Path $envFilePath

$issuer = Get-ConfigValue -DotEnv $dotEnv -ExplicitValue $IssuerUrl -Names @(
    "LISTINGKIT_MACHINE_ZITADEL_ISSUER_URL",
    "ZITADEL_ISSUER_URL"
)
$clientIDValue = Get-ConfigValue -DotEnv $dotEnv -ExplicitValue $ClientId -Names @(
    "LISTINGKIT_MACHINE_CLIENT_ID"
)
$clientSecretValue = Get-ConfigValue -DotEnv $dotEnv -ExplicitValue $ClientSecret -Names @(
    "LISTINGKIT_MACHINE_CLIENT_SECRET"
)
$scopeValue = Get-ConfigValue -DotEnv $dotEnv -ExplicitValue $Scopes -Names @(
    "LISTINGKIT_MACHINE_SCOPES",
    "ZITADEL_SCOPES"
) -DefaultValue "openid profile"

Require-ConfigValue -Name "LISTINGKIT_MACHINE_CLIENT_ID" -Value $clientIDValue
Require-ConfigValue -Name "LISTINGKIT_MACHINE_CLIENT_SECRET" -Value $clientSecretValue

$tokenEndpoint = Get-ConfigValue -DotEnv $dotEnv -ExplicitValue $TokenUrl -Names @(
    "LISTINGKIT_MACHINE_TOKEN_URL"
)
if ([string]::IsNullOrWhiteSpace($tokenEndpoint)) {
    Require-ConfigValue -Name "LISTINGKIT_MACHINE_ZITADEL_ISSUER_URL or ZITADEL_ISSUER_URL" -Value $issuer
    $tokenEndpoint = $issuer.TrimEnd("/") + "/oauth/v2/token"
}

$scopeValue = Ensure-OpenIdScope -Value $scopeValue
if ($scopeValue -notmatch "urn:zitadel:iam:org:project:id:.+:aud") {
    Write-Host "Warning: LISTINGKIT_MACHINE_SCOPES does not contain urn:zitadel:iam:org:project:id:{projectid}:aud; Go API introspection may reject the token." -ForegroundColor DarkYellow
}

$basicBytes = [System.Text.Encoding]::UTF8.GetBytes("${clientIDValue}:${clientSecretValue}")
$headers = @{
    Authorization = "Basic " + [Convert]::ToBase64String($basicBytes)
}
$body = @{
    grant_type = "client_credentials"
    scope      = $scopeValue
}

Write-Host "Requesting ListingKit machine token from $tokenEndpoint ..."
$response = Invoke-RestMethod `
    -Uri $tokenEndpoint `
    -Method Post `
    -Headers $headers `
    -ContentType "application/x-www-form-urlencoded" `
    -Body $body `
    -TimeoutSec 30

$accessToken = [string]$response.access_token
if ([string]::IsNullOrWhiteSpace($accessToken)) {
    throw "ZITADEL token response did not contain access_token"
}

$tokenPath = [System.IO.Path]::GetFullPath($TokenFile)
$tokenDir = Split-Path -Parent $tokenPath
if (-not (Test-Path -LiteralPath $tokenDir)) {
    New-Item -ItemType Directory -Path $tokenDir | Out-Null
}
[System.IO.File]::WriteAllText($tokenPath, $accessToken, [System.Text.UTF8Encoding]::new($false))

$expiresIn = if ($null -ne $response.expires_in) { [string]$response.expires_in } else { "unknown" }
Write-Host "Saved ListingKit machine token to $tokenPath"
Write-Host "Token expires in seconds: $expiresIn"

if (-not $SkipAuthCheck) {
    $authCheckScript = Join-Path $PSScriptRoot "listingkit-auth-check.ps1"
    $authCheckArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $authCheckScript, "-TokenFile", $tokenPath)
    if (-not [string]::IsNullOrWhiteSpace($ApiBaseUrl)) {
        $authCheckArgs += @("-ApiBaseUrl", $ApiBaseUrl)
    }
    & powershell @authCheckArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}
