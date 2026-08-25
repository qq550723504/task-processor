$scriptPath = Join-Path $PSScriptRoot 'casdoor-phone-idp-preflight.ps1'

Describe 'casdoor-phone-idp-preflight' {
    It 'is a read-only discovery and JWKS check' {
        Test-Path -LiteralPath $scriptPath | Should Be $true
        $content = Get-Content -Raw -LiteralPath $scriptPath
        $content | Should Match '\.well-known/openid-configuration'
        $content | Should Not Match 'SecretKey|Authorization:|Bearer |kubectl.+get secret'
    }
}
