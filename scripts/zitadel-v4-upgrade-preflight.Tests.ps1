Describe "zitadel-v4-upgrade-preflight" {
  It "uses only deployment reads" {
    $content = Get-Content -Raw scripts/zitadel-v4-upgrade-preflight.ps1
    $content | Should Match 'kubectl.+get deployment zitadel'
    $content | Should Match 'kubectl.+get deployment zitadel-login'
    $content | Should Not Match 'get secret|\.data|Authorization:|Bearer '
  }
  It "pins the approved target" {
    (Get-Content -Raw scripts/zitadel-v4-upgrade-preflight.ps1) | Should Match 'v4\.17\.1'
  }
  It "compares the deployed image tag exactly" {
    $content = Get-Content -Raw scripts/zitadel-v4-upgrade-preflight.ps1
    $content | Should Match 'Get-ExactImageTag'
    $content | Should Not Match '-notmatch'
    $content | Should Match 'coreTag -ne \$TargetVersion'
  }
}
