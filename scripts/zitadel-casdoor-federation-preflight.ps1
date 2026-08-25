param([Parameter(Mandatory)][uri]$IssuerURL, [Parameter(Mandatory)][string]$ProviderID)
$token = [string]$env:ZITADEL_ADMIN_TOKEN
if ([string]::IsNullOrWhiteSpace($token)) { throw "ZITADEL_ADMIN_TOKEN is required" }
$headers = @{ Authorization = "Bearer $token"; "Content-Type" = "application/json" }
$providers = Invoke-RestMethod -Method Post -Uri ($IssuerURL.ToString().TrimEnd('/') + "/admin/v1/idps/_search") -Headers $headers -Body "{}"
$provider = @($providers.result | Where-Object id -eq $ProviderID)[0]
$policy = Invoke-RestMethod -Uri ($IssuerURL.ToString().TrimEnd('/') + "/admin/v1/policies/login") -Headers $headers
if ($null -eq $provider -or $provider.config.issuer -ne "https://id.shuomiai.com" -or $provider.config.scopes -notcontains "openid" -or -not $provider.config.isCreationAllowed -or $provider.config.isLinkingAllowed -or $provider.config.isAutoUpdate -or -not $policy.externalLogin) { throw "unsafe Casdoor IdP policy" }
[pscustomobject]@{ providerID=$provider.id; issuer=$provider.config.issuer; scopes=@($provider.config.scopes); creationAllowed=[bool]$provider.config.isCreationAllowed; linkingAllowed=[bool]$provider.config.isLinkingAllowed; automaticUpdate=[bool]$provider.config.isAutoUpdate; externalLogin=[bool]$policy.externalLogin } | ConvertTo-Json -Compress
