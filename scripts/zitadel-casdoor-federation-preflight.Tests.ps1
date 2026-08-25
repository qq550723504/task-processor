$actionPath = Join-Path $PSScriptRoot '..\deployments\kubernetes\casdoor\zitadel-actions\map-casdoor-phone-identity.js'
$preflightPath = Join-Path $PSScriptRoot 'zitadel-casdoor-federation-preflight.ps1'

Describe 'ZITADEL Casdoor phone federation' {
    It 'maps only a verified stable external subject' {
        Test-Path -LiteralPath $actionPath | Should Be $true
        $content = Get-Content -Raw -LiteralPath $actionPath
        $content | Should Match 'claimsJSON\(\)'
        $content | Should Match 'phone_verified'
        $content | Should Match 'phone\.id\.shuo[m]?iai\.invalid'
        $content | Should Not Match 'phone_number|api\.userGrants|fetch\(|console\.log|console\.error'
    }

    It 'keeps the policy preflight read-only and credential-gated' {
        Test-Path -LiteralPath $preflightPath | Should Be $true
        $content = Get-Content -Raw -LiteralPath $preflightPath
        $content | Should Match 'ZITADEL_ADMIN_TOKEN'
        $content | Should Match 'idps/_search'
        $content | Should Not Match 'POST.*policies|PUT.*policies|PATCH.*policies|kubectl.+apply'
    }
}
