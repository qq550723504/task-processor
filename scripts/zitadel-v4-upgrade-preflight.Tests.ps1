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

  It "documents setup before starting the v4 runtime" {
    $runbook = Get-Content -Raw docs/operations/zitadel-v4-security-upgrade-runbook.md
    $runbook | Should Match 'setup --masterkeyFromEnv --init-projections=true'
    $runbook | Should Match '"start","--masterkeyFromEnv","--tlsMode","external"'
    $runbook | Should Match 'zitadel_app'
    $runbook.IndexOf('kubectl -n zitadel set image deployment/zitadel') | Should BeLessThan $runbook.IndexOf('setup --masterkeyFromEnv --init-projections=true')
    $runbook.IndexOf('setup --masterkeyFromEnv --init-projections=true') | Should BeLessThan $runbook.IndexOf('kubectl -n zitadel patch deployment zitadel')
  }

  It "provides a setup-job generator that preserves secret references" {
    $path = 'scripts/zitadel-v4-upgrade-setup.ps1'
    (Test-Path $path) | Should Be $true
    if (Test-Path $path) {
      $content = Get-Content -Raw $path
      $content | Should Match 'setup.*masterkeyFromEnv.*init-projections=true'
      $content | Should Match 'replicas.*-ne 0'
      $content | Should Not Match 'get secret|\.data|Authorization:|Bearer '
    }
  }
}
