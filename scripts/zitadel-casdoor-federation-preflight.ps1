[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [uri]$ZitadelURL,

    [Parameter(Mandatory)]
    [string]$ProviderID,

    [Parameter(Mandatory)]
    [uri]$ExpectedProviderIssuer
)

$ErrorActionPreference = 'Stop'
$base = $ZitadelURL.AbsoluteUri.TrimEnd('/')
$expectedIssuer = $ExpectedProviderIssuer.AbsoluteUri.TrimEnd('/')
if ($ZitadelURL.Scheme -ne 'https' -or $ExpectedProviderIssuer.Scheme -ne 'https') {
    throw 'ZITADEL and provider issuer URLs must use HTTPS'
}

$token = [string]$env:ZITADEL_ADMIN_TOKEN
if ([string]::IsNullOrWhiteSpace($token)) {
    throw 'ZITADEL_ADMIN_TOKEN is required'
}

$headers = @{
    Authorization = "Bearer $token"
    'Content-Type' = 'application/json'
}
$providers = Invoke-RestMethod -Method Post -Uri "$base/admin/v1/idps/_search" -Headers $headers -Body '{}'
$provider = @($providers.result | Where-Object { $_.id -eq $ProviderID })[0]
$policy = Invoke-RestMethod -Method Get -Uri "$base/admin/v1/policies/login" -Headers $headers

if ($null -eq $provider) {
    throw "ZITADEL provider not found: $ProviderID"
}
if ([string]$provider.config.issuer -ne $expectedIssuer) {
    throw 'provider issuer does not match the expected staging issuer'
}
if (@($provider.config.scopes) -notcontains 'openid') {
    throw 'provider does not request the required openid scope'
}
if (-not [bool]$provider.config.isCreationAllowed) {
    throw 'provider account creation is disabled'
}
if ([bool]$provider.config.isLinkingAllowed) {
    throw 'provider account linking must be disabled'
}
if ([bool]$provider.config.isAutoUpdate) {
    throw 'provider automatic profile update must be disabled'
}
if (-not [bool]$policy.externalLogin) {
    throw 'ZITADEL external login policy is disabled'
}

[pscustomobject]@{
    providerID = [string]$provider.id
    issuer = [string]$provider.config.issuer
    scopes = @($provider.config.scopes)
    creationAllowed = [bool]$provider.config.isCreationAllowed
    linkingAllowed = [bool]$provider.config.isLinkingAllowed
    automaticUpdate = [bool]$provider.config.isAutoUpdate
    externalLogin = [bool]$policy.externalLogin
} | ConvertTo-Json -Compress
