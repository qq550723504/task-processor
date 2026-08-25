param([Parameter(Mandatory)][uri]$IssuerURL)
$issuer = $IssuerURL.ToString().TrimEnd('/')
$d = Invoke-RestMethod -Uri ($issuer + '/.well-known/openid-configuration')
if ($d.issuer -ne $issuer -or [string]::IsNullOrWhiteSpace($d.authorization_endpoint) -or [string]::IsNullOrWhiteSpace($d.jwks_uri)) { throw 'invalid Casdoor OIDC discovery' }
[pscustomobject]@{issuer=$d.issuer; authorizationEndpoint=$d.authorization_endpoint; jwksUri=$d.jwks_uri} | ConvertTo-Json -Compress
