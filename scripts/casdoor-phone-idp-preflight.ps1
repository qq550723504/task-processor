param([Parameter(Mandatory)][uri]$IssuerURL)
$issuer = $IssuerURL.ToString().TrimEnd('/')
$d = Invoke-RestMethod -Uri ($issuer + '/.well-known/openid-configuration')
if ([string]::IsNullOrWhiteSpace($d.authorization_endpoint) -or [string]::IsNullOrWhiteSpace($d.jwks_uri)) { throw 'invalid Casdoor OIDC discovery' }
$auth = [uri]$d.authorization_endpoint
$jwks = [uri]$d.jwks_uri
# Endpoints must be HTTPS on the expected issuer host; an http:// or
# unrelated-host discovery document must fail the preflight.
if ($d.issuer -ne $issuer -or $auth.Scheme -ne 'https' -or $jwks.Scheme -ne 'https' -or $auth.Host -ne $IssuerURL.Host -or $jwks.Host -ne $IssuerURL.Host) { throw 'invalid Casdoor OIDC discovery' }
[pscustomobject]@{issuer=$d.issuer; authorizationEndpoint=$d.authorization_endpoint; jwksUri=$d.jwks_uri} | ConvertTo-Json -Compress
