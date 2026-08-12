Describe "1688 runtime acceptance safety" {
    BeforeAll {
        $scriptPath = Join-Path $PSScriptRoot "1688-runtime-acceptance.ps1"
        . $scriptPath -TestOnly
    }

    It "requires an exact confirmation before task creation" {
        { Assert-TaskCreationConfirmation -Mode "Crawl" -Confirmation "" } | Should Throw
        { Assert-TaskCreationConfirmation -Mode "Crawl" -Confirmation "CREATE-1688-TASK" } | Should Not Throw
        { Assert-TaskCreationConfirmation -Mode "Preflight" -Confirmation "" } | Should Not Throw
    }

    It "maps crawler product data without adding the rejected source store field" {
        $payload = New-ListingKitHandoffPayload `
            -ProductData @{ id = "321"; title = "Test product"; url = "https://detail.1688.com/offer/321.html" } `
            -SourceAccountID 3001 -SheinStoreID 168811 -CrawlerTaskID "crawler-task-1"

        $payload.source_account_id | Should Be 3001
        $payload.shein_store_id | Should Be 168811
        $payload.product.id | Should Be "321"
        $payload.PSObject.Properties.Name | Should Not Contain "source_store_id"
    }

    It "does not expose token or profile values in redacted errors" {
        $message = Get-RedactedRuntimeError -StatusCode 401 -Endpoint "https://example.test" -RawBody "token=secret; user_data_dir=C:\profiles\tenant-1"
        $message | Should Match "HTTP 401"
        $message | Should Not Match "secret|profiles|tenant-1|user_data_dir"
    }
}
