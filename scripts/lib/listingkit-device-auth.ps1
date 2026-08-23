$ErrorActionPreference = "Stop"

function Test-ListingKitDeviceLoopbackHost {
    param([string]$HostName)

    return $HostName -ieq "localhost" -or $HostName -eq "127.0.0.1" -or $HostName -eq "::1"
}

function ConvertTo-ListingKitDeviceUri {
    param(
        [string]$Value,
        [string]$Name
    )

    if ([string]::IsNullOrWhiteSpace($Value)) {
        throw "$Name is required"
    }
    try {
        $uri = [Uri]$Value
    } catch {
        throw "$Name must be an absolute HTTPS URI"
    }
    if (-not $uri.IsAbsoluteUri -or -not [string]::IsNullOrWhiteSpace($uri.UserInfo) -or
        -not [string]::IsNullOrWhiteSpace($uri.Query) -or -not [string]::IsNullOrWhiteSpace($uri.Fragment)) {
        throw "$Name must be an absolute HTTPS URI"
    }
    if ($uri.Scheme -eq "https") {
        return $uri
    }
    if ($uri.Scheme -eq "http" -and (Test-ListingKitDeviceLoopbackHost -HostName $uri.Host)) {
        return $uri
    }
    throw "$Name must use HTTPS unless it is a literal loopback test endpoint"
}

function Assert-ListingKitDeviceURI {
    param(
        [string]$Uri,
        [Uri]$Issuer,
        [string]$Name = "OIDC endpoint"
    )

    $candidate = ConvertTo-ListingKitDeviceUri -Value $Uri -Name $Name
    if ($null -eq $Issuer -or $candidate.Scheme -cne $Issuer.Scheme -or
        $candidate.Host -cne $Issuer.Host -or $candidate.Port -ne $Issuer.Port) {
        throw "$Name must use the same-origin as the issuer"
    }
    return $candidate
}

function Assert-ListingKitDeviceAPIBaseUrl {
    param([string]$ApiBaseUrl)

    [void](ConvertTo-ListingKitDeviceUri -Value $ApiBaseUrl -Name "-ApiBaseUrl")
}

function Get-ListingKitDeviceDiscoveryUri {
    param([Uri]$Issuer)

    $builder = [UriBuilder]::new($Issuer)
    $builder.Path = ($Issuer.AbsolutePath.TrimEnd("/") + "/.well-known/openid-configuration")
    return $builder.Uri.AbsoluteUri
}

function Invoke-ListingKitDeviceOAuthRequest {
    param(
        [string]$Uri,
        [ValidateSet("Get", "Post")]
        [string]$Method,
        [hashtable]$Form = $null,
        [ValidateRange(1, 3600)]
        [int]$TimeoutSec = 300
    )

    $params = @{
        Uri                 = $Uri
        Method              = $Method
        MaximumRedirection  = 0
        ErrorAction         = "Stop"
        SkipHttpErrorCheck  = $true
    }
    if ((Get-Command Invoke-RestMethod).Parameters.ContainsKey("OperationTimeoutSeconds")) {
        $params.OperationTimeoutSeconds = $TimeoutSec
    } else {
        $params.TimeoutSec = $TimeoutSec
    }
    if ($null -ne $Form) {
        $params.ContentType = "application/x-www-form-urlencoded"
        $params.Body = $Form
    }
    try {
        return Invoke-RestMethod @params
    } catch {
        throw "Device authorization request failed"
    }
}

function Get-ListingKitDeviceRemainingTimeoutSec {
    param([DateTime]$Deadline)

    $remaining = ($Deadline - [DateTime]::UtcNow).TotalSeconds
    if ($remaining -le 0) {
        throw "Device authorization timed out"
    }
    return [Math]::Max(1, [int][Math]::Ceiling($remaining))
}

function Get-ListingKitDeviceOAuthScopes {
    param(
        [string]$Scopes = "",
        [string]$ProjectID = ""
    )

    $configured = ([string]$Scopes).Trim()
    if ([string]::IsNullOrWhiteSpace($configured)) {
        $configured = ([string]$env:ZITADEL_SCOPES).Trim()
    }
    if (-not [string]::IsNullOrWhiteSpace($configured)) {
        $scopeTokens = @($configured -split '\s+' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        if ($scopeTokens -contains "offline_access") {
            throw "offline_access is not allowed for ListingKit device authorization"
        }
        return $configured
    }

    $project = ([string]$ProjectID).Trim()
    if ([string]::IsNullOrWhiteSpace($project)) {
        $project = ([string]$env:TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID).Trim()
    }
    if ([string]::IsNullOrWhiteSpace($project)) {
        throw "-Scopes or -ProjectID is required for ListingKit device authorization"
    }

    return @(
        "openid",
        "profile",
        "email",
        "urn:zitadel:iam:user:resourceowner",
        "urn:zitadel:iam:org:project:id:${project}:aud",
        "urn:zitadel:iam:org:project:role:listingkit_viewer",
        "urn:zitadel:iam:org:project:role:listingkit_operator",
        "urn:zitadel:iam:org:project:role:listingkit_admin",
        "urn:zitadel:iam:org:project:role:platform_admin"
    ) -join " "
}

function Get-ListingKitDevicePollSleepSec {
    param(
        [int]$PollIntervalSec,
        [DateTime]$Deadline
    )

    $remaining = ($Deadline - [DateTime]::UtcNow).TotalSeconds
    if ($remaining -le 0) {
        return 0
    }
    return [Math]::Min([Math]::Max(0, [double]$PollIntervalSec), $remaining)
}

function Get-ListingKitDeviceString {
    param(
        [object]$Response,
        [string]$Name
    )

    if ($null -eq $Response) {
        return ""
    }
    return ([string]$Response.$Name).Trim()
}

function Get-ListingKitDeviceTokenExpiry {
    param([string]$Token)

    $parts = ([string]$Token).Trim().Split(".")
    if ($parts.Count -ne 3) {
        return $null
    }

    try {
        $payload = $parts[1].Replace("-", "+").Replace("_", "/")
        switch ($payload.Length % 4) {
            2 { $payload += "==" }
            3 { $payload += "=" }
            0 { }
            default { return $null }
        }
        $claims = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($payload)) | ConvertFrom-Json
        $unixSeconds = 0L
        if (-not [long]::TryParse([string]$claims.exp, [ref]$unixSeconds) -or $unixSeconds -le 0) {
            return $null
        }
        return [DateTimeOffset]::FromUnixTimeSeconds($unixSeconds)
    } catch {
        return $null
    }
}

function Test-ListingKitDeviceTokenCacheSupported {
    return [Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([Runtime.InteropServices.OSPlatform]::Windows)
}

function Save-ListingKitDeviceTokenCache {
    param(
        [string]$Path,
        [string]$Token,
        [DateTimeOffset]$ExpiresAt = [DateTimeOffset]::MinValue
    )

    if (-not (Test-ListingKitDeviceTokenCacheSupported)) {
        return
    }
    $expiry = $ExpiresAt
    if ($expiry -eq [DateTimeOffset]::MinValue) {
        $expiry = Get-ListingKitDeviceTokenExpiry -Token $Token
    }
    if ($null -eq $expiry -or $expiry -le [DateTimeOffset]::UtcNow.AddMinutes(1)) {
        return
    }

    try {
        $secureToken = ConvertTo-SecureString -String $Token.Trim() -AsPlainText -Force
        $protectedToken = ConvertFrom-SecureString -SecureString $secureToken
        $cache = [ordered]@{
            version         = 1
            expires_at      = $expiry.ToUniversalTime().ToString("o")
            protected_token = $protectedToken
        }
        $cachePath = [IO.Path]::GetFullPath($Path)
        $cacheDirectory = Split-Path -Parent $cachePath
        if (-not (Test-Path -LiteralPath $cacheDirectory)) {
            New-Item -ItemType Directory -Path $cacheDirectory -Force | Out-Null
        }
        [IO.File]::WriteAllText($cachePath, ($cache | ConvertTo-Json -Compress), [Text.UTF8Encoding]::new($false))
    } catch {
        Write-Verbose "Device token cache could not be saved; continuing without a cache."
    }
}

function Get-ListingKitDeviceTokenCache {
    param([string]$Path)

    if (-not (Test-ListingKitDeviceTokenCacheSupported) -or [string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return ""
    }

    try {
        $cache = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
        $metadataExpiry = [DateTimeOffset]::MinValue
        if (-not [DateTimeOffset]::TryParse([string]$cache.expires_at, [ref]$metadataExpiry) -or $metadataExpiry -le [DateTimeOffset]::UtcNow.AddMinutes(1)) {
            return ""
        }
        if ([string]::IsNullOrWhiteSpace([string]$cache.protected_token)) {
            return ""
        }

        $secureToken = ConvertTo-SecureString -String ([string]$cache.protected_token)
        $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureToken)
        try {
            $token = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
        } finally {
            if ($bstr -ne [IntPtr]::Zero) {
                [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
            }
        }
        $tokenExpiry = Get-ListingKitDeviceTokenExpiry -Token $token
        if ($null -ne $tokenExpiry -and ($tokenExpiry -le [DateTimeOffset]::UtcNow.AddMinutes(1) -or $tokenExpiry -lt $metadataExpiry.AddMinutes(-1) -or $tokenExpiry -gt $metadataExpiry.AddMinutes(1))) {
            return ""
        }
        return $token
    } catch {
        return ""
    }
}

function Resolve-ListingKitDeviceToken {
    param(
        [string]$IssuerURL,
        [string]$ClientID,
        [string]$Scopes = "",
        [string]$ProjectID = "",
        [ValidateRange(1, 3600)]
        [int]$TimeoutSec = 300,
        [switch]$OpenBrowser,
        [ref]$ExpiresAt
    )

    if ([string]::IsNullOrWhiteSpace($ClientID)) {
        throw "-ClientID is required"
    }
    $issuer = ConvertTo-ListingKitDeviceUri -Value $IssuerURL -Name "-IssuerURL"
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSec)
    $discovery = Invoke-ListingKitDeviceOAuthRequest -Method Get -Uri (Get-ListingKitDeviceDiscoveryUri -Issuer $issuer) -TimeoutSec (Get-ListingKitDeviceRemainingTimeoutSec -Deadline $deadline)
    $deviceEndpoint = Assert-ListingKitDeviceURI -Uri (Get-ListingKitDeviceString -Response $discovery -Name "device_authorization_endpoint") -Issuer $issuer -Name "device authorization endpoint"
    $tokenEndpoint = Assert-ListingKitDeviceURI -Uri (Get-ListingKitDeviceString -Response $discovery -Name "token_endpoint") -Issuer $issuer -Name "token endpoint"

    $scopeValue = Get-ListingKitDeviceOAuthScopes -Scopes $Scopes -ProjectID $ProjectID
    $device = Invoke-ListingKitDeviceOAuthRequest -Method Post -Uri $deviceEndpoint.AbsoluteUri -Form @{
        client_id = $ClientID.Trim()
        scope     = $scopeValue
    } -TimeoutSec (Get-ListingKitDeviceRemainingTimeoutSec -Deadline $deadline)
    $deviceCode = Get-ListingKitDeviceString -Response $device -Name "device_code"
    $userCode = Get-ListingKitDeviceString -Response $device -Name "user_code"
    $verificationUri = Assert-ListingKitDeviceURI -Uri (Get-ListingKitDeviceString -Response $device -Name "verification_uri") -Issuer $issuer -Name "verification URI"
    $expiresIn = 0
    [void][int]::TryParse((Get-ListingKitDeviceString -Response $device -Name "expires_in"), [ref]$expiresIn)
    if ([string]::IsNullOrWhiteSpace($deviceCode) -or [string]::IsNullOrWhiteSpace($userCode) -or $expiresIn -le 0) {
        throw "Device authorization response is incomplete"
    }
    $pollIntervalSec = 5
    $providerInterval = 0
    if ([int]::TryParse((Get-ListingKitDeviceString -Response $device -Name "interval"), [ref]$providerInterval) -and $providerInterval -gt 0) {
        $pollIntervalSec = $providerInterval
    }

    Write-Host ("Open {0} and enter code {1}" -f $verificationUri.AbsoluteUri, $userCode)
    if ($OpenBrowser) {
        Start-Process -FilePath $verificationUri.AbsoluteUri
    }

    $deviceDeadline = [DateTime]::UtcNow.AddSeconds($expiresIn)
    if ($deviceDeadline -lt $deadline) {
        $deadline = $deviceDeadline
    }
    $initialPollSleepSec = Get-ListingKitDevicePollSleepSec -PollIntervalSec $pollIntervalSec -Deadline $deadline
    if ($initialPollSleepSec -gt 0) {
        Start-Sleep -Seconds $initialPollSleepSec
    }
    while ([DateTime]::UtcNow -lt $deadline) {
        $tokenResponse = Invoke-ListingKitDeviceOAuthRequest -Method Post -Uri $tokenEndpoint.AbsoluteUri -Form @{
            grant_type  = "urn:ietf:params:oauth:grant-type:device_code"
            device_code = $deviceCode
            client_id   = $ClientID.Trim()
        } -TimeoutSec (Get-ListingKitDeviceRemainingTimeoutSec -Deadline $deadline)
        $refreshToken = Get-ListingKitDeviceString -Response $tokenResponse -Name "refresh_token"
        if (-not [string]::IsNullOrWhiteSpace($refreshToken)) {
            throw "Device authorization returned a refresh token"
        }
        $accessToken = Get-ListingKitDeviceString -Response $tokenResponse -Name "access_token"
        if (-not [string]::IsNullOrWhiteSpace($accessToken)) {
            $expiresIn = 0
            if ($null -ne $ExpiresAt -and [int]::TryParse((Get-ListingKitDeviceString -Response $tokenResponse -Name "expires_in"), [ref]$expiresIn) -and $expiresIn -gt 0) {
                $ExpiresAt.Value = [DateTimeOffset]::UtcNow.AddSeconds($expiresIn)
            }
            return $accessToken
        }
        switch (Get-ListingKitDeviceString -Response $tokenResponse -Name "error") {
            "authorization_pending" {
                $sleepSec = Get-ListingKitDevicePollSleepSec -PollIntervalSec $pollIntervalSec -Deadline $deadline
                if ($sleepSec -gt 0) {
                    Start-Sleep -Seconds $sleepSec
                }
                continue
            }
            "slow_down" {
                $pollIntervalSec += 5
                $sleepSec = Get-ListingKitDevicePollSleepSec -PollIntervalSec $pollIntervalSec -Deadline $deadline
                if ($sleepSec -gt 0) {
                    Start-Sleep -Seconds $sleepSec
                }
                continue
            }
            "access_denied" { throw "Device authorization was denied" }
            "expired_token" { throw "Device authorization expired" }
            default { throw "Device authorization token exchange failed" }
        }
    }
    throw "Device authorization timed out"
}
