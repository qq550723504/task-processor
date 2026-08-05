# Studio Background Removal Retry UI Design

## Problem

The Studio preview can render a failed background-removal status from the flat/local design state while `itemizedBatchDetail` is not yet hydrated. The retry button is rendered, but `handleRetryBackgroundRemoval` returns immediately when the detail is missing, so no retry request is sent and no error is shown.

## Design

Keep the existing retry endpoint and permission checks. When the retry action is invoked, require only the active batch ID up front. If the itemized detail is missing, fetch the hydrated batch detail using the existing batch hydration path and apply it to the workbench state. Then call the existing `retrySheinStudioBatchBackgroundRemoval` API with the requested design ID and the batch tenant ID. If hydration or retry fails, surface the existing formatted workbench error and always clear the retrying design state.

The button remains disabled only while its own retry request is in flight. No backend contract, image-processing behavior, or authorization rule changes.

## Verification

Add a regression test around the retry controller behavior proving that a missing detail is hydrated before the retry API is called, and that the retrying state is cleared on failure. Run the focused frontend test, the related Studio test suite, and the frontend build/type-check command used by the repository.
