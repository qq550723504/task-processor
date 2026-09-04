$scriptPath = Join-Path $PSScriptRoot "store-service-history-migrate.ps1"

Describe "Store service history migration wrapper" {
    It "keeps verify as the default and requires an explicit Phase E action" {
        $content = Get-Content -Raw $scriptPath
        $content | Should Match '\[ValidateSet\("verify", "backfill", "constraints"\)\]\[string\]\$Action = "verify"'
        $content | Should Match 'constraint-lock-timeout'
        $content | Should Match 'constraint-statement-timeout'
        $content | Should Match 'cmd/store-service-history-migrate'
    }

    It "does not invoke deployment or schema automation" {
        $content = Get-Content -Raw $scriptPath
        $content | Should Not Match 'kubectl|helm|AutoMigrate|listingkit-schema-migrate'
    }
}
