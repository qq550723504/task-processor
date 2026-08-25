Describe "casdoor-phone-idp-preflight" {
  It "reads discovery without a credential" {
    $c = Get-Content -Raw scripts/casdoor-phone-idp-preflight.ps1
    $c | Should Match '\.well-known/openid-configuration'
    $c | Should Not Match 'SecretKey|Authorization:|Bearer |kubectl.+get secret'
  }
}
