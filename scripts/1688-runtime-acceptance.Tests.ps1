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
        $threw = $false
        try { Assert-TaskCreationConfirmation -Mode "Crawl" -Confirmation "create-1688-task" } catch { $threw = $true }
        $threw | Should Be $true
        { Assert-TaskCreationConfirmation -Mode "Preflight" -Confirmation "" } | Should Not Throw
    }

    It "maps crawler product data without adding the rejected source store field" {
        $payload = New-ListingKitHandoffPayload `
            -ProductData ([pscustomobject]@{ id = "321"; title = "Test product"; url = "https://detail.1688.com/offer/321.html" }) `
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
            if ($Path -eq "/api/v1/listing-kits/settings-health") {
                return @{ status = "ready" }
            }
            return @{ status = "ok" }
        }

        Invoke-Preflight -Token "test-token" -BaseUrl "https://example.test" | Out-Null

        ($script:requestMethods -join ",") | Should Be "Get /health,Get /readyz,Get /api/v1/listing-kits/settings-health"
    }

    It "runs public preflight checks before failing at the authenticated check when the token is missing" {
        $script:requestMethods = @()
        Mock Resolve-ListingKitToken { throw "token is missing" }
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path, $Token)
            $script:requestMethods += "$Method $Path"
            if ($Path -eq "/api/v1/listing-kits/settings-health") {
                $Token | Should Be ""
                return @{ status = "ready" }
            }
            return @{ status = "ok" }
        }

        Invoke-Main

        ($script:requestMethods -join ",") | Should Be "Get /health,Get /readyz,Get /api/v1/listing-kits/settings-health"
    }

    It "blocks preflight when settings health is not ready" {
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path)
            if ($Path -eq "/api/v1/listing-kits/settings-health") {
                return @{ status = "blocked" }
            }
            return @{ status = "ok" }
        }

        { Invoke-Preflight -Token "test-token" -BaseUrl "https://example.test" } | Should Throw "settings-health status is 'blocked'; task creation is not allowed"
    }

    It "accepts the ordered handoff payload at the request boundary" {
        Mock Invoke-RestMethod { return @{ status = "ok" } }
        $body = New-ListingKitHandoffPayload `
            -ProductData ([pscustomobject]@{ id = "325"; title = "Binding product"; url = "https://detail.1688.com/offer/325.html" }) `
            -SourceAccountID 3005 -SheinStoreID 168815 -CrawlerTaskID "crawler-task-5"

        { Invoke-AcceptanceRequest -Method Post -Path "/api/v1/test" -Token "test-token" -Body $body -BaseUrl "https://example.test" } | Should Not Throw
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

    It "rejects a non-1688 URL before making a crawler request" {
        $script:requestCalls = 0
        Mock Invoke-AcceptanceRequest {
            $script:requestCalls++
            throw "request should not run"
        }

        try { Invoke-Crawl -Url "https://example.com/product/321" -SourceAccountID 3001 -Confirmation "CREATE-1688-TASK" } catch { }

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
            return @{ data = @{ task_id = "crawler-task-1"; status = $status; product_data = [pscustomobject]@{ id = "321"; title = "Test product"; url = "https://detail.1688.com/offer/321.html" } } }
        }

        $result = Invoke-Crawl -Url "https://detail.1688.com/offer/321.html" -SourceAccountID 3001 -Confirmation "CREATE-1688-TASK" -PollIntervalSec 0

        $result.TaskID | Should Be "crawler-task-1"
        $result.Status | Should Be "success"
        $result.ProductData.id | Should Be "321"
    }

    It "accepts the live ProductData response casing from the crawler task" {
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path)
            if ($Path -eq "/api/v1/crawl") {
                return @{ data = @{ task_id = "crawler-task-live" } }
            }
            return @{ data = @{ task_id = "crawler-task-live"; status = "success"; ProductData = [pscustomobject]@{ id = "323"; title = "Live casing product"; url = "https://detail.1688.com/offer/323.html" } } }
        }

        $result = Invoke-Crawl -Url "https://detail.1688.com/offer/323.html" -SourceAccountID 3003 -Confirmation "CREATE-1688-TASK" -PollIntervalSec 0

        $result.ProductData.id | Should Be "323"
    }

    It "fails immediately when the crawler returns an unknown status" {
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path)
            if ($Path -eq "/api/v1/crawl") {
                return @{ data = @{ task_id = "crawler-task-invalid-status" } }
            }
            return @{ data = @{ task_id = "crawler-task-invalid-status"; status = "queued" } }
        }

        { Invoke-Crawl -Url "https://detail.1688.com/offer/326.html" -SourceAccountID 3006 -Confirmation "CREATE-1688-TASK" -PollIntervalSec 0 } | Should Throw "crawler task crawler-task-invalid-status returned unexpected status 'queued'"
    }

    It "rejects an expired crawler deadline before polling" {
        { Get-RemainingRequestTimeoutSec -Deadline ([DateTime]::UtcNow.AddSeconds(-1)) -TaskID "crawler-task-expired" } | Should Throw "crawler task crawler-task-expired timed out"

        $remaining = Get-RemainingRequestTimeoutSec -Deadline ([DateTime]::UtcNow.AddSeconds(10))
        ($remaining -ge 1 -and $remaining -le 11) | Should Be $true
    }

    It "caps polling sleep at the remaining deadline" {
        $sleep = Get-CappedPollSleepSec -PollIntervalSec 300 -Deadline ([DateTime]::UtcNow.AddSeconds(2))

        ($sleep -ge 0 -and $sleep -le 2) | Should Be $true
    }

    It "preserves a one-second request allowance while the deadline is still future" {
        $remaining = Get-RemainingRequestTimeoutSec -Deadline ([DateTime]::UtcNow.AddMilliseconds(500)) -TaskID "crawler-task-short"

        $remaining | Should Be 1
    }

    It "sends the crawler product to the ListingKit handoff without source_store_id" {
        $script:lastHandoffBody = $null
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path, $Body)
            if ($Path -eq "/api/v1/crawl") {
                return @{ data = @{ task_id = "crawler-task-2" } }
            }
            if ($Path -eq "/api/v1/tasks/crawler-task-2") {
                return @{ data = @{ task_id = "crawler-task-2"; status = "success"; product_data = [pscustomobject]@{ id = "322"; title = "End-to-end product"; url = "https://detail.1688.com/offer/322.html" } } }
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

    It "rejects an EndToEnd handoff response without a task id" {
        Mock Invoke-AcceptanceRequest {
            param($Method, $Path)
            if ($Path -eq "/api/v1/crawl") {
                return @{ data = @{ task_id = "crawler-task-3" } }
            }
            if ($Path -eq "/api/v1/tasks/crawler-task-3") {
                return @{ data = @{ task_id = "crawler-task-3"; status = "success"; ProductData = [pscustomobject]@{ id = "324"; title = "Missing handoff task"; url = "https://detail.1688.com/offer/324.html" } } }
            }
            return @{ data = @{ status = "pending" } }
        }

        { Invoke-EndToEnd -Url "https://detail.1688.com/offer/324.html" -SourceAccountID 3004 -SheinStoreID 168814 -Confirmation "CREATE-1688-TASK" -PollIntervalSec 0 } | Should Throw "handoff response did not contain a task id"
    }

    It "reports the live source identity fields and derived source key" {
        $evidence = Get-SourceIdentityEvidence -SourceIdentity ([pscustomobject]@{
            SourceType = "crawler"
            SourcePlatform = "1688"
            SourceID = "327"
            SourceVersion = "v1"
        })

        $evidence.SourceID | Should Be "327"
        $evidence.SourceKey | Should Be "crawler:1688:327:version:v1"
    }
}
