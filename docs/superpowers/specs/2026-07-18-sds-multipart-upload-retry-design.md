# SDS multipart upload retry design

## Goal

Give the existing SDS multipart-upload retry loop enough end-to-end time to
recover from a temporary network or SDS object-storage interruption, without
turning invalid requests into long-running retries.

## Scope

The retry remains in the req/v3 client configured by
`internal/sds/client.Client`. It already resets multipart file readers between
attempts and retries transport errors, `429`, and `5xx` responses. This change
adjusts the shared retry interval and extends the ListingKit SDS design-sync
deadline, which is the total budget passed to all upload attempts.

## Policy

- Keep three total upload attempts (`RetryCount=2`).
- Replace the current deterministic wait sequence with capped exponential
  backoff plus jitter, based on the configured 1.5-second base delay.
- Preserve the existing retry classification: transport errors, `429`, and
  `5xx` responses retry; other `4xx` responses do not.
- Extend `SDSDesignSyncTimeout` from 130 seconds to 180 seconds. The context
  remains the hard end-to-end budget and cancels a wait or request when it
  expires.
- Preserve the final response and error type so existing task failure handling
  still records an actionable cause if every attempt fails.

## Failure handling

The ListingKit workflow continues to mark an exhausted SDS design-sync failure
as a failed child task and a `needs_review` task. The existing explicit
`sds_design_sync` retry remains available after automatic retries are
exhausted.

## Tests

- Retry delays increase exponentially and remain within the configured cap.
- Retry jitter stays bounded by its exponential delay window.
- The SDS design-sync timeout is 180 seconds for one variant and retains its
  bounded per-variant extension for larger batches.
- Existing request-level retry tests continue to prove that a multipart upload
  is replayable and a non-retryable `4xx` is not retried.
