# PR #274 Development Admission Review Fixes

## Scope

This follow-up resolves the three review findings on PR #274. The first finding is already present in the current PR head (`ef63f92`): non-decision reviews do not replace a maintainer's current-head approval. This change preserves that implementation and adds regression coverage only where needed.

The remaining changes are:

1. Run `.github/scripts/pr-scope-guard.test.cjs` in CI whenever the evaluator or its workflow contract changes.
2. Make the required commit status explicitly identify an authorized architecture override.

## Invariants and ownership

- `.github/scripts/pr-scope-guard.cjs` remains the policy evaluator; `.github/workflows/development-admission.yml` remains the only caller that reads GitHub admission inputs and publishes the status.
- The evaluator must continue to fail closed for unstable or incomplete pull-request snapshots.
- An override still requires the exact `architecture-approved` label and an eligible maintainer/admin approval for the current head. A `COMMENTED` or `PENDING` review is non-decisive and must not revoke an existing approval; `CHANGES_REQUESTED` and `DISMISSED` remain decisive revocations.
- Status descriptions are audit output only. They must distinguish ordinary allowance from allowance caused by `result.overridden === true`; status state and target SHA do not change.
- CI test wiring is non-privileged and must not alter the trusted evaluator's authorization boundary.

## Data flow and failure behavior

The CI workflow filters both push and pull-request events for `.github/scripts/**`, then runs a dedicated Node test job. The existing notification job includes that result so a failed policy suite cannot be reported as an all-green CI run.

The evaluator maps results as follows:

- `allowed: false` → failure, `Exceeds admission limits`.
- `allowed: true, overridden: false` → success, `Within admission limits`.
- `allowed: true, overridden: true` → success, an explicit authorized-override description.

No new database, external service, cache, filesystem, or browser persistence boundary is introduced. The only existing external write is the commit status publication, still targeted at `merge_commit_sha`; if evaluation fails, the workflow publishes an error status when possible and fails the check. A retry re-reads the pull request and recomputes the result, so no new idempotency key or recovery protocol is required.

## Verification evidence

- A focused regression test must fail before the status implementation changes and pass afterward.
- The full guard test suite must pass.
- The CI workflow must contain both `.github/scripts/**` path filters and the dedicated `node --test .github/scripts/pr-scope-guard.test.cjs` step; the notification dependency must include the new job.
- `git diff --check` must pass.
