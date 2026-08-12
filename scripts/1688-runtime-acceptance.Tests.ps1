Describe "1688 runtime acceptance safety" {
    BeforeAll {
        $scriptPath = Join-Path $PSScriptRoot "1688-runtime-acceptance.ps1"
        . $scriptPath -TestOnly
    }

    It "requires an exact confirmation before task creation" {
        $threw = $false
        try { Assert-TaskCreationConfirmation -Mode "Crawl" -Confirmation "" } catch { $threw = $true }
        $threw | Should Be $true

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
        ($payload.Keys -contains "source_store_id") | Should Be $false
    }

    It "does not expose token or profile values in redacted errors" {
        $message = Get-RedactedRuntimeError -StatusCode 401 -Endpoint "https://example.test" -RawBody "token=secret; user_data_dir=C:\profiles\tenant-1"
        $message | Should Match "HTTP 401"
        $message | Should Not Match "secret|profiles|tenant-1|user_data_dir"
    }

    It "prefers the environment token over the token file" {
        $tokenPath = Join-Path $TestDrive "listingkit-token.txt"
        Set-Content -LiteralPath $tokenPath -Value "file-token"
        $previous = $env:LISTINGKIT_API_TOKEN
        try {
            $env:LISTINGKIT_API_TOKEN = "Bearer env-token"
            (Resolve-ListingKitToken -Path $tokenPath) | Should Be "env-token"
        } finally {
            $env:LISTINGKIT_API_TOKEN = $previous
        }
    }

    It "keeps the default preflight request sequence GET-only" {
        $script:requestMethods = @()
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path)
            $script:requestMethods += "$Method $Path"
            return @{ status = "ok" }
        }

        Invoke-Preflight -Token "test-token" -BaseUrl "https://example.test" | Out-Null

        ($script:requestMethods -join ",") | Should Be "Get /health,Get /readyz,Get /api/v1/listing-kits/settings-health"
    }

    It "rejects Crawl without exact confirmation before making a request" {
        $script:requestCalls = 0
        Mock Invoke-AcceptanceRequest {
            $script:requestCalls++
            throw "request should not run"
        }

        try { Invoke-Crawl -Url "https://detail.1688.com/offer/321.html" -SourceAccountID 3001 -Confirmation "" } catch { }

        $script:requestCalls | Should Be 0
    }

    It "polls processing then returns a successful crawler product" {
        $script:pollCount = 0
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path)
            if ($Path -eq "/api/v1/crawl") {
                return @{ data = @{ task_id = "crawler-task-1" } }
            }
            $status = if ($script:pollCount++ -eq 0) { "processing" } else { "success" }
            return @{ data = @{ task_id = "crawler-task-1"; status = $status; product_data = @{ id = "321"; title = "Test product"; url = "https://detail.1688.com/offer/321.html" } } }
        }

        $result = Invoke-Crawl -Url "https://detail.1688.com/offer/321.html" -SourceAccountID 3001 -Confirmation "CREATE-1688-TASK" -PollIntervalSec 0

        $result.TaskID | Should Be "crawler-task-1"
        $result.Status | Should Be "success"
        $result.ProductData.id | Should Be "321"
    }

    It "sends the crawler product to the ListingKit handoff without source_store_id" {
        $script:lastHandoffBody = $null
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path, $Body)
            if ($Path -eq "/api/v1/crawl") {
                return @{ data = @{ task_id = "crawler-task-2" } }
            }
            if ($Path -eq "/api/v1/tasks/crawler-task-2") {
                return @{ data = @{ task_id = "crawler-task-2"; status = "success"; product_data = @{ id = "322"; title = "End-to-end product"; url = "https://detail.1688.com/offer/322.html" } } }
            }
            $script:lastHandoffBody = $Body
            return @{ data = @{ task_id = "listing-task-2"; status = "pending" } }
        }

        $result = Invoke-EndToEnd -Url "https://detail.1688.com/offer/322.html" -SourceAccountID 3002 -SheinStoreID 168812 -Confirmation "CREATE-1688-TASK" -PollIntervalSec 0

        $result.CrawlerTaskID | Should Be "crawler-task-2"
        $script:lastHandoffBody.source_account_id | Should Be 3002
        $script:lastHandoffBody.shein_store_id | Should Be 168812
        ($script:lastHandoffBody.Keys -contains "source_store_id") | Should Be $false
    }
}
