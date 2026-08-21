Describe "1688 local-agent acceptance guardrails" {
    BeforeAll {
        $scriptPath = Join-Path $PSScriptRoot "1688-local-agent-acceptance.ps1"
        $scriptText = Get-Content -LiteralPath $scriptPath -Raw
    }

    It "requires explicit job creation confirmation" {
        $scriptText | Should Match "CREATE-LOCAL-AGENT-JOB"
    }

    It "supports claiming an existing pending job without a URL" {
        $scriptText | Should Match "-Confirm is only valid when -Url creates"
        $scriptText | Should Match "-url"
    }

    It "does not expose account or target-store inputs" {
        $scriptText | Should Not Match "source_account_id|listing_store|source-account-id|listing-store"
    }

    It "validates the public offer host and numeric path" {
        $scriptText | Should Match "detail\.1688\.com"
        $scriptText | Should Match "offer/\[0-9\]"
    }

    It "asserts the bounded reconstructed envelope summary" {
        $scriptText | Should Match "envelope_summary"
        $scriptText | Should Match "source_key"
    }
}
