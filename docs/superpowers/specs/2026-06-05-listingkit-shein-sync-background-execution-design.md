# ListingKit SHEIN Sync Background Execution Design

## Goal

Make `POST /api/v1/listing-kits/shein-sync/stores/:store_id/sync` return immediately while the actual SHEIN product sync continues in the background.

## Scope

- Keep the existing sync algorithm, repository writes, and job status model.
- Change only the execution model used by the HTTP runtime path.
- Do not change the request/response schema for the existing sync endpoint.

## Recommended Approach

Wrap the current synchronous `SheinSyncService` with an asynchronous adapter used only by the HTTP runtime wiring:

- The adapter creates a `pending` job by delegating to a lightweight synchronous bootstrap path.
- It starts a background goroutine that runs the existing sync flow and updates the same job to `running`, `succeeded`, or `failed`.
- The HTTP handler continues to return `202 Accepted` with the created job payload immediately.

## Why This Approach

- It solves client timeout issues without rewriting the sync core.
- It preserves existing repository and dashboard job status behavior.
- It limits risk by keeping schedulers and pure service tests on the synchronous implementation.

## Tradeoffs

- In-flight work is not durable across process restarts because execution uses in-process goroutines.
- This is acceptable for the current local-debugging goal, but a future production-grade version should move execution to a persistent worker/queue.

## Success Criteria

- Triggering store sync returns quickly with a `pending` or `running` job.
- The background task still writes synced products and final job status.
- Existing synchronous service tests remain valid.
