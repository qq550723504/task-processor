$scriptPath = Join-Path $PSScriptRoot "listingkit-owner-scope-dry-run.ps1"

Describe "listingkit owner reconciliation wrapper" {
    It "keeps dry-run as the default and exposes explicit execute confirmation" {
        $content = Get-Content -Raw $scriptPath
        $content | Should Match '\[switch\]\$Execute'
        $content | Should Match '\[string\]\$ConfirmReport'
        $content | Should Match 'if \(\$Execute\)'
        $content | Should Match 'ConfirmReport is required with -Execute'
        $content | Should Match 'cmd/listingkit-owner-scope-dry-run'
    }

    It "does not embed credentials or raw identity values" {
        $content = Get-Content -Raw $scriptPath
        $content | Should Not Match 'OPENAI_API_KEY|ZITADEL_CLIENT_SECRET|Authorization:|user_id|subject'
    }
}
