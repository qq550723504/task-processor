# 1688 Runtime Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a safe PowerShell operator tool that runs read-only 1688 runtime preflight by default and can explicitly execute a crawler-only or crawler-to-ListingKit acceptance flow.

**Architecture:** Keep all behavior in an operator script with small testable functions. The script calls existing `/health`, `/readyz`, `/api/v1/listing-kits/settings-health`, `/api/v1/crawl`, `/api/v1/tasks/{id}`, and `/api/v1/product-sourcing/1688/listingkit/tasks` endpoints; it adds no production API or persistence model. Pester loads the script in test-only mode and mocks the request seam, so tests never contact a cluster or create tasks.

**Tech Stack:** PowerShell, Pester, existing ListingKit HTTP APIs.

## Global Constraints

- Default mode is `Preflight` and performs GET requests only.
- `Crawl` and `EndToEnd` require `-ConfirmCreateTask CREATE-1688-TASK` before any POST.
- Use `source_account_id`; never send or support `source_store_id`.
- Read bearer tokens only from `LISTINGKIT_API_TOKEN` or `.local/listingkit-api-token.txt`; never print or persist token contents.
- Do not print passwords, cookies, proxy credentials, browser profile paths, `user_data_dir`, or raw error bodies.
- Do not call marketplace submission, preview, readiness, deletion, or administrative provisioning endpoints.
- Keep unrelated working-tree changes untouched and stage only the files listed in each task.

---

### Task 1: Define the acceptance script contract with failing Pester tests

**Files:**
- Create: `scripts/1688-runtime-acceptance.Tests.ps1`
- Test target: `scripts/1688-runtime-acceptance.ps1`

**Interfaces:**
- Consumes: the future script functions `Resolve-ListingKitToken`, `Assert-TaskCreationConfirmation`, `New-ListingKitHandoffPayload`, and `Get-RedactedRuntimeError`.
- Produces: tests that fail until the safety contract and payload mapping exist.

- [ ] **Step 1: Write the failing Pester tests**

Create the test file with these behaviors:

```powershell
BeforeAll {
    $scriptPath = Join-Path $PSScriptRoot "1688-runtime-acceptance.ps1"
    . $scriptPath -TestOnly
}

Describe "1688 runtime acceptance safety" {
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
        $message = Get-RedactedRuntimeError -StatusCode 401 -Endpoint "https://example.test" -RawBody "token=secret; user_data_dir=C:\\profiles\\tenant-1"
        $message | Should Match "HTTP 401"
        $message | Should Not Match "secret|profiles|tenant-1|user_data_dir"
    }
}
```

- [ ] **Step 2: Run the focused tests and verify the expected red failure**

Run:

```powershell
Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -Output Detailed
```

Expected: the test setup or missing-function assertions fail because `scripts/1688-runtime-acceptance.ps1` does not yet provide the contract. Fix only test syntax/setup errors; do not add production script code before observing the feature-missing failure.

- [ ] **Step 3: Commit the red tests**

```powershell
git add scripts/1688-runtime-acceptance.Tests.ps1
git commit -m "test: define 1688 runtime acceptance safety contract"
```

### Task 2: Implement safe token resolution, request seam, and preflight mode

**Files:**
- Create: `scripts/1688-runtime-acceptance.ps1`
- Test: `scripts/1688-runtime-acceptance.Tests.ps1`

**Interfaces:**
- Consumes: Task 1 Pester contract.
- Produces: `Preflight` mode, `Resolve-ListingKitToken`, `Invoke-AcceptanceRequest`, `Assert-TaskCreationConfirmation`, and sanitized error output.

- [ ] **Step 1: Add the script parameter contract and test-only guard**

Use this parameter block:

```powershell
param(
    [ValidateSet("Preflight", "Crawl", "EndToEnd")]
    [string]$Mode = "Preflight",
    [string]$ApiBaseUrl = "",
    [string]$TokenFile = "",
    [string]$Url = "",
    [long]$SourceAccountID = 0,
    [long]$SheinStoreID = 0,
    [string]$ConfirmCreateTask = "",
    [int]$TimeoutSec = 300,
    [int]$PollIntervalSec = 5,
    [switch]$TestOnly
)
```

Set `$ErrorActionPreference = "Stop"`, resolve the repository root from `$PSScriptRoot`, default the API URL from `LISTINGKIT_API_BASE_URL` or `http://localhost:8085`, and default the token file to `.local\listingkit-api-token.txt`.

- [ ] **Step 2: Implement token resolution without secret output**

Implement `Resolve-ListingKitToken` so environment wins over the token file and `Bearer ` is stripped:

```powershell
function Resolve-ListingKitToken {
    param([string]$Path)

    $value = [string]$env:LISTINGKIT_API_TOKEN
    if ([string]::IsNullOrWhiteSpace($value) -and (Test-Path -LiteralPath $Path)) {
        $value = Get-Content -LiteralPath $Path -Raw
    }
    $value = $value.Trim()
    if ($value.StartsWith("Bearer ", [System.StringComparison]::OrdinalIgnoreCase)) {
        $value = $value.Substring(7).Trim()
    }
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "No ListingKit API token found; set LISTINGKIT_API_TOKEN or provide the standard token file."
    }
    return $value
}
```

The function may return the token to the caller for an HTTP header, but no log/error path may include the returned value.

- [ ] **Step 3: Implement the request seam and redacted errors**

Implement `Invoke-AcceptanceRequest` around `Invoke-RestMethod` with an `Authorization` header, JSON body only when supplied, and bounded timeout. Catch failures and throw `Get-RedactedRuntimeError` containing status code and endpoint path only. Do not include the raw response body.

Implement `Get-RedactedRuntimeError` to retain only a numeric HTTP status and the URI path; it must replace or omit `token`, `cookie`, `password`, `proxy`, `user_data_dir`, and profile path text.

- [ ] **Step 4: Implement preflight GET checks**

Implement `Invoke-Preflight` to call, in order, `GET /health`, `GET /readyz`, and `GET /api/v1/listing-kits/settings-health`. Print only endpoint path and status (`PASS`/`BLOCKED`), then return success only when all three calls succeed. Missing token must stop before the authenticated request and return a non-zero exit code.

- [ ] **Step 5: Run the focused tests and parser check**

Run:

```powershell
Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -Output Detailed
$tokens = $null
$errors = $null
[System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path scripts/1688-runtime-acceptance.ps1), [ref]$tokens, [ref]$errors) | Out-Null
if ($errors.Count -gt 0) { throw $errors[0].Message }
```

Expected: Task 1 tests pass, and the script has no parser errors.

- [ ] **Step 6: Commit the preflight implementation**

```powershell
git add scripts/1688-runtime-acceptance.ps1 scripts/1688-runtime-acceptance.Tests.ps1
git commit -m "feat: add safe 1688 runtime preflight"
```

### Task 3: Add confirmed crawler polling and end-to-end handoff

**Files:**
- Modify: `scripts/1688-runtime-acceptance.ps1`
- Modify: `scripts/1688-runtime-acceptance.Tests.ps1`

**Interfaces:**
- Consumes: Task 2 request seam and token handling.
- Produces: `Crawl` and `EndToEnd` modes, `New-ListingKitHandoffPayload`, bounded polling, and redacted acceptance output.

- [ ] **Step 1: Add tests for confirmation and polling behavior**

Extend Pester tests with mocked `Invoke-AcceptanceRequest` responses for:

```powershell
It "rejects Crawl without exact confirmation before making a request" {
    Mock Invoke-AcceptanceRequest { throw "request should not run" }
    { Invoke-Crawl -Url "https://detail.1688.com/offer/321.html" -SourceAccountID 3001 -Confirmation "" } | Should Throw
    Should -Invoke Invoke-AcceptanceRequest -Times 0
}

It "polls processing then returns a successful crawler product" {
    Mock Invoke-AcceptanceRequest {
        if ($Path -eq "/api/v1/crawl") { return @{ data = @{ task_id = "crawler-task-1" } } }
        return @{
            data = @{
                task_id = "crawler-task-1"
                status = if ($script:pollCount++ -eq 0) { "processing" } else { "success" }
                product_data = @{ id = "321"; title = "Test product"; url = "https://detail.1688.com/offer/321.html" }
            }
        }
    }
    $script:pollCount = 0
    $result = Invoke-Crawl -Url "https://detail.1688.com/offer/321.html" -SourceAccountID 3001 -Confirmation "CREATE-1688-TASK" -PollIntervalSec 0
    $result.TaskID | Should Be "crawler-task-1"
    $result.ProductData.id | Should Be "321"
}
```

Also add a test that `EndToEnd` sends `source_account_id`, `shein_store_id`, `product`, and `source_run_id`, and never sends `source_store_id`.

- [ ] **Step 2: Run the new tests and observe the expected red failure**

Run:

```powershell
Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -Output Detailed
```

Expected: the new tests fail because `Invoke-Crawl` and the end-to-end path are not implemented yet.

- [ ] **Step 3: Implement exact confirmation and required-input validation**

Implement `Assert-TaskCreationConfirmation` to allow `Preflight` without confirmation and require the exact string `CREATE-1688-TASK` for `Crawl` and `EndToEnd`. Validate positive `SourceAccountID`, non-empty 1688 URL, positive `SheinStoreID` for `EndToEnd`, positive timeout, and non-negative poll interval before any POST.

- [ ] **Step 4: Implement bounded crawler polling**

Implement `Invoke-Crawl`:

1. POST `/api/v1/crawl` with `{ url, source_account_id }`.
2. Extract `data.task_id`; fail if missing.
3. Poll GET `/api/v1/tasks/{task_id}` until `success` or `failed`.
4. Sleep `PollIntervalSec` between non-terminal responses.
5. Throw on timeout, terminal failure, malformed response, or missing `product_data`.

Return an object containing only `TaskID`, `Status`, and `ProductData` for the next step and output projection.

- [ ] **Step 5: Implement the end-to-end handoff payload and mode**

Implement `New-ListingKitHandoffPayload` with this shape:

```powershell
@{
    url              = $ProductData.url
    product          = $ProductData
    source_run_id    = $CrawlerTaskID
    request_id       = "1688-runtime-$CrawlerTaskID"
    source_account_id = $SourceAccountID
    platforms        = @("shein")
    shein_store_id   = $SheinStoreID
}
```

Preserve all product fields returned by the crawler, do not fabricate credentials or source identity, and do not add `source_store_id`. `Invoke-EndToEnd` calls `Invoke-Crawl`, POSTs the payload to `/api/v1/product-sourcing/1688/listingkit/tasks`, and prints only crawler task ID, ListingKit task ID, status, source ID/key if present, and normalized product URL.

- [ ] **Step 6: Run focused tests, parser, and diff checks**

Run:

```powershell
Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -Output Detailed
$tokens = $null
$errors = $null
[System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path scripts/1688-runtime-acceptance.ps1), [ref]$tokens, [ref]$errors) | Out-Null
if ($errors.Count -gt 0) { throw $errors[0].Message }
git diff --check
```

Expected: all Pester tests pass, parser reports zero errors, and diff check is clean.

- [ ] **Step 7: Commit crawler and handoff modes**

```powershell
git add scripts/1688-runtime-acceptance.ps1 scripts/1688-runtime-acceptance.Tests.ps1
git commit -m "feat: add confirmed 1688 runtime acceptance flow"
```

### Task 4: Record operator instructions and run repository gates

**Files:**
- Modify: `docs/product/validation/2026-08-08-1688-controlled-replay.md`

**Interfaces:**
- Consumes: the script modes and Pester evidence from Tasks 2-3.
- Produces: explicit operator commands and an evidence boundary separating preflight, crawler acceptance, ListingKit task creation, and SHEIN submission.

- [ ] **Step 1: Add documented commands**

Document these commands without real credentials or IDs:

```powershell
Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -Output Detailed
.\scripts\1688-runtime-acceptance.ps1 -Mode Preflight
.\scripts\1688-runtime-acceptance.ps1 -Mode Crawl -Url "https://detail.1688.com/offer/<offer-id>.html" -SourceAccountID <account-id> -ConfirmCreateTask CREATE-1688-TASK
.\scripts\1688-runtime-acceptance.ps1 -Mode EndToEnd -Url "https://detail.1688.com/offer/<offer-id>.html" -SourceAccountID <account-id> -SheinStoreID <shein-store-id> -ConfirmCreateTask CREATE-1688-TASK
```

State that the operator must manually complete the 1688 login in the tenant/account browser profile before `Crawl`, and that successful task creation does not prove preview/readiness or SHEIN submission.

- [ ] **Step 2: Run the final validation gates**

Run:

```powershell
Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -Output Detailed
git diff --check
$env:GOWORK='off'; go test ./... -count=1
$env:CGO_ENABLED='0'; $env:GOOS='linux'; go build ./cmd/listing-control-plane ./cmd/product-listing-api ./cmd/shein-listing ./cmd/temu-listing
```

The PowerShell tests must pass. Record the exact Go/build result; do not call a timeout a pass.

- [ ] **Step 3: Review scope and commit the documentation**

Run:

```powershell
git status --short
git diff --stat HEAD~1..HEAD
```

Stage only the validation document and commit:

```powershell
git add docs/product/validation/2026-08-08-1688-controlled-replay.md
git commit -m "docs: document 1688 runtime acceptance tool"
```

### Final verification checklist

- [ ] Latest `origin/master` is the worktree base.
- [ ] Default mode performs no POST request.
- [ ] Exact confirmation is required before `Crawl` and `EndToEnd`.
- [ ] `source_account_id` is used and `source_store_id` is absent.
- [ ] Polling handles processing, success, failure, and timeout.
- [ ] No credential, cookie, proxy, or profile path is printed.
- [ ] Pester and PowerShell parser checks pass.
- [ ] Full Go test and maintained command builds are recorded accurately.
- [ ] Live acceptance remains explicitly separate from SHEIN preview/readiness/submission.

