# 1688 Runtime Acceptance Design

## Goal

Provide a safe operator-run PowerShell acceptance tool for the maintained 1688 crawler and ListingKit handoff runtime. The tool must make read-only infrastructure and authentication checks easy to repeat, while requiring an explicit confirmation before it creates a real crawler task or ListingKit task.

## Context

The repository now contains the maintained `crawler-1688` module, tenant-scoped crawler task storage, account-profile binding, and the authenticated 1688-to-ListingKit handoff route. Deterministic replay tests prove data propagation, but live acceptance still needs to verify a tenant-owned 1688 login account, a manually logged-in browser profile, asynchronous crawler execution, and optional ListingKit task persistence.

The current runtime uses `source_account_id` to select a tenant-owned 1688 login account. `source_store_id` is intentionally unsupported and must not be added back to the acceptance payload.

## Chosen approach

Add one operator-facing script, `scripts/1688-runtime-acceptance.ps1`, with explicit modes:

- `Preflight` (default): GET-only checks for `/health`, `/readyz`, and authenticated `/api/v1/listing-kits/settings-health`.
- `Crawl`: after explicit confirmation, POST `/api/v1/crawl` with a URL and `source_account_id`, then poll `/api/v1/tasks/{task_id}` until success or failure. It proves account-profile binding and durable crawler-result visibility without creating a ListingKit task.
- `EndToEnd`: performs the confirmed crawler run, then submits the returned product to `/api/v1/product-sourcing/1688/listingkit/tasks` with the authenticated source account and explicit SHEIN target store. It prints only task IDs, statuses, source identity, and redacted error codes.

The script uses the existing `LISTINGKIT_API_BASE_URL`, `LISTINGKIT_API_TOKEN`, and `.local/listingkit-api-token.txt` conventions. Token values are never printed, persisted by the script, or included in error output. The default mode never sends a POST request. `Crawl` and `EndToEnd` require both `-ConfirmCreateTask CREATE-1688-TASK` and the required IDs/URL, preventing accidental task creation from a health check.

## Data flow

```text
Preflight
  GET /health
  GET /readyz
  GET /api/v1/listing-kits/settings-health

Crawl
  POST /api/v1/crawl { url, source_account_id }
      |
      v
  GET /api/v1/tasks/{crawler_task_id} until terminal
      |
      v
  ProductData from successful crawler result

EndToEnd
  ProductData + source_account_id + shein_store_id
      |
      v
  POST /api/v1/product-sourcing/1688/listingkit/tasks
      |
      v
  persisted ListingKit task response
```

The script will not call marketplace submission, preview, readiness, deletion, or administrative store-provisioning endpoints. It will not expose browser profile paths, cookies, passwords, proxy credentials, or raw bearer tokens.

## Error handling and evidence

- Transport failures, non-success HTTP statuses, missing task IDs, malformed JSON, crawler terminal failures, and handoff failures produce non-zero exit codes.
- Polling is bounded by `-TimeoutSec` and `-PollIntervalSec`; timeout output includes the crawler task ID but no credentials or product payload.
- Successful output includes the mode, endpoint status, crawler task ID, crawler terminal status, ListingKit task ID when created, and normalized source identity when returned.
- The script writes no evidence file automatically. The operator can redirect the redacted console output into the existing validation record.
- The validation document must distinguish deterministic tests, read-only preflight, crawler acceptance, and end-to-end ListingKit acceptance. A successful script run must not be described as SHEIN submission acceptance.

## Testing strategy

Use Pester text/parser tests and a local fake HTTP server or request seam so tests never contact a cluster and never create a task. The tests must cover:

1. default `Preflight` performs only GET requests;
2. POST modes reject missing or incorrect confirmation before any request;
3. token resolution prefers `LISTINGKIT_API_TOKEN` and falls back to the standard token file without printing the token;
4. crawler polling handles processing, success, failure, and timeout responses;
5. `EndToEnd` maps crawler `product_data` into the handoff request while adding only `source_account_id` and `shein_store_id` as runtime selectors;
6. output redaction never emits `Authorization`, token contents, cookie values, proxy credentials, `user_data_dir`, or profile paths.

No production Go endpoint or database schema change is part of this design. If the acceptance run exposes a real runtime defect, that defect gets a separate regression change with its own test-first cycle.

