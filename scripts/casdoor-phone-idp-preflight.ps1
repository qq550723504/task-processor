[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [uri]$IssuerURL
)

$ErrorActionPreference = 'Stop'
$issuer = $IssuerURL.AbsoluteUri.TrimEnd('/')
if ($IssuerURL.Scheme -ne 'https') {
    throw 'IssuerURL must use HTTPS'
}

$discoveryUri = "$issuer/.well-known/openid-configuration"
$discovery = Invoke-RestMethod -Uri $discoveryUri -Method Get
if ([string]$discovery.issuer -ne $issuer) {
    throw 'OIDC discovery issuer does not match IssuerURL'
}

$authorizationEndpoint = [uri][string]$discovery.authorization_endpoint
$jwksUri = [uri][string]$discovery.jwks_uri
if ($authorizationEndpoint.Scheme -ne 'https' -or $jwksUri.Scheme -ne 'https') {
    throw 'OIDC authorization and JWKS endpoints must use HTTPS'
}
if ($authorizationEndpoint.Host -ne $IssuerURL.Host -or $jwksUri.Host -ne $IssuerURL.Host) {
    throw 'OIDC endpoints must remain on the configured issuer host'
}

$jwks = Invoke-RestMethod -Uri $jwksUri -Method Get
$keys = @($jwks.keys)
if ($keys.Count -eq 0) {
    throw 'OIDC JWKS contains no keys'
}

[pscustomobject]@{
    issuer = $issuer
    authorizationEndpoint = $authorizationEndpoint.AbsoluteUri
    jwksUri = $jwksUri.AbsoluteUri
    jwksKeyCount = $keys.Count
} | ConvertTo-Json -Compress
