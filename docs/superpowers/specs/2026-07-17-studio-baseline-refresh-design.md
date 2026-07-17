# Studio batch baseline refresh

## Problem

SHEIN Studio task creation evaluates each approved design against its SDS baseline. The task gate currently reads the persisted baseline cache directly. A baseline that was blocked while SDS login was in progress remains blocked indefinitely, even after SDS authentication becomes usable. This prevents the user from advancing and causes every candidate in a batch to fail with `baseline_not_ready`.

## Design

The Studio task gate will receive a readiness checker that uses the existing SDS baseline readiness service rather than a raw cache repository. The checker will:

1. Load the cached baseline.
2. Reconcile recoverable login-related validation states against the current SDS login status.
3. Persist a ready validation state only when the baseline payload is already valid and current SDS authentication is usable and not in progress.
4. Leave genuine login-in-progress, missing-authentication, malformed-cache, and product/design-surface failures blocked.

The task gate will keep its per-baseline cache for one task-creation run, so a batch with many designs does not repeatedly perform the same readiness check.

## Recovery

After deployment, task creation for batch `cc7ca86a-8e24-4c4e-90a6-38bb699b1bda` will be retried normally. No batch, design, or task-link rows will be force-marked successful; the normal gate will re-evaluate every selection and create tasks only for baselines that are actually ready.

## Tests

- A cached `login_in_progress` baseline becomes eligible when SDS status has a usable access token and is not in progress.
- The same baseline remains rejected while SDS login is still in progress.
- Existing non-login baseline failures remain rejected.
- The batch task-creation integration path uses the refreshing checker rather than the raw repository reader.
