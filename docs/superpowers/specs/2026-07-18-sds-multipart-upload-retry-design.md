# SDS multipart upload retry design

## Goal

Retry transient failures while uploading a ListingKit design image to SDS, so a
temporary network or SDS object-storage interruption does not immediately leave
the task in `needs_review`.

## Scope

The retry belongs in `internal/sds/client.Client.UploadFile`, which is the
single client boundary used for SDS multipart uploads. Existing callers keep
their current API and their existing context deadline remains the total budget.

## Policy

- Make at most three total upload attempts.
- Wait one second before the second attempt and two seconds before the third
  attempt.
- Retry transport errors, including context deadline errors, and HTTP `408`,
  `429`, and `5xx` responses.
- Do not retry other `4xx` responses: these represent invalid form data,
  invalid or expired signatures, authorization failures, or another request
  that cannot succeed unchanged.
- Before every wait and attempt, honor context cancellation and deadline. A
  retry must never extend the parent SDS design-sync deadline.
- Preserve the final response and error type so existing task failure handling
  still records an actionable cause if every attempt fails.

## Failure handling

The ListingKit workflow continues to mark an exhausted SDS design-sync failure
as a failed child task and a `needs_review` task. The existing explicit
`sds_design_sync` retry remains available after automatic retries are
exhausted.

## Tests

- A transport failure followed by success performs a second upload and returns
  success.
- A non-retryable `4xx` response performs one upload only.
- Repeated retryable failures stop after three total attempts and return the
  final error.
- A canceled context stops without an additional retry.
