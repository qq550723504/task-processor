Describe "zitadel-casdoor-federation-preflight" {
  BeforeAll {
    $script:action = Get-Content -Raw deployments/kubernetes/casdoor/zitadel-actions/map-casdoor-phone-identity.js
    $script:preflight = Get-Content -Raw scripts/zitadel-casdoor-federation-preflight.ps1
  }

  It "pins the fixed Casdoor issuer in the action" {
    $action | Should Match 'https://id\.shuomiai\.com'
  }

  It "gates user creation on the verified phone claim" {
    $action | Should Match 'phone_verified'
    $action | Should Match 'verified Casdoor phone identity required'
  }

  It "uses only the technical .invalid email domain" {
    $action | Should Match 'phone\.id\.shuomiai\.invalid'
    $action | Should Not Match 'phone_number'
  }

  It "never grants roles, tenants, or fetches externally" {
    $action | Should Not Match 'api\.userGrants|fetch\(|console\.log'
  }

  It "preflight reads provider policy without printing secrets" {
    $preflight | Should Match 'idps/_search'
    $preflight | Should Match 'ZITADEL_ADMIN_TOKEN'
    $preflight | Should Not Match 'Write-Host|Write-Output.*token|ConvertTo-SecureString'
  }
}
