param(
  # Base URL of the ZITADEL Admin API (e.g. https://id.staging.shuomiai.com).
  [Parameter(Mandatory)][uri]$ZitadelURL,
  [Parameter(Mandatory)][string]$ProviderID,
  # Expected Casdoor provider issuer for the target environment; defaults to
  # the production Casdoor issuer. Pass the staging issuer when preflighting
  # the staging ZITADEL instance.
  [Parameter()][uri]$ExpectedIssuer = "https://id.shuomiai.com"
)
$token = [string]$env:ZITADEL_ADMIN_TOKEN
if ([string]::IsNullOrWhiteSpace($token)) { throw "ZITADEL_ADMIN_TOKEN is required" }
$headers = @{ Authorization = "Bearer $token"; "Content-Type" = "application/json" }
$apiBase = $ZitadelURL.ToString().TrimEnd('/')
$expectedIssuer = $ExpectedIssuer.ToString().TrimEnd('/')
$providers = Invoke-RestMethod -Method Post -Uri ($apiBase + "/admin/v1/idps/_search") -Headers $headers -Body "{}"
$provider = @($providers.result | Where-Object id -eq $ProviderID)[0]
$policy = Invoke-RestMethod -Uri ($apiBase + "/admin/v1/policies/login") -Headers $headers
if ($null -eq $provider -or $provider.config.issuer -ne $expectedIssuer -or $provider.config.scopes -notcontains "openid" -or -not $provider.config.isCreationAllowed -or $provider.config.isLinkingAllowed -or $provider.config.isAutoUpdate -or -not $policy.externalLogin) { throw "unsafe Casdoor IdP policy" }
[pscustomobject]@{ providerID=$provider.id; issuer=$provider.config.issuer; scopes=@($provider.config.scopes); creationAllowed=[bool]$provider.config.isCreationAllowed; linkingAllowed=[bool]$provider.config.isLinkingAllowed; automaticUpdate=[bool]$provider.config.isAutoUpdate; externalLogin=[bool]$policy.externalLogin } | ConvertTo-Json -Compress
