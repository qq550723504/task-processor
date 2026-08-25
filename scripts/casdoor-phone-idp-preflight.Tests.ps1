Describe "casdoor-phone-idp-preflight" {
  It "reads discovery without a credential" {
    $c = Get-Content -Raw scripts/casdoor-phone-idp-preflight.ps1
    $c | Should Match '\.well-known/openid-configuration'
    $c | Should Not Match 'SecretKey|Authorization:|Bearer |kubectl.+get secret'
  }
  It "validates that endpoints are HTTPS on the expected issuer host" {
    $c = Get-Content -Raw scripts/casdoor-phone-idp-preflight.ps1
    $c | Should Match '\$auth\.Scheme -ne ''https'''
    $c | Should Match '\$jwks\.Scheme -ne ''https'''
    $c | Should Match '\$auth\.Host -ne \$IssuerURL\.Host'
    $c | Should Match '\$jwks\.Host -ne \$IssuerURL\.Host'
  }
}
