# ListingKit Release Candidate Runbook

Use this runbook for a customer-trial release candidate (RC). It separates
evidence collection from the human Go/No-Go decision and keeps API and UI
rollouts independently reversible.

Complete the [Customer Trial Acceptance Checklist](./listingkit-customer-trial-acceptance-checklist.md)
alongside this runbook. The checklist records tenant access, the controlled
product lifecycle, recovery evidence, and the final human decision.

## Release identity

Record one immutable source SHA before starting. Both images must be tagged
with the first eight characters of that SHA (or another documented immutable
tag); never use `latest` as the release or rollback target.

| Field | Record |
| --- | --- |
| Source SHA |  |
| API image | `docker.io/xuwei190/task-processor-product-listing-api:<tag>` |
| UI image | `docker.io/xuwei190/task-processor-listingkit-ui:<tag>` |
| API workflow run |  |
| UI workflow run |  |
| Operator / approver |  |

## Pre-release gate

- [ ] The exact SHA has a successful Commercial Readiness workflow run.
- [ ] `go test ./... -count=1`, UI lint/typecheck/test/build, image builds,
  and production Kustomize rendering are retained as CI evidence.
- [ ] Required database/configuration migrations have been reviewed. Runtime
  auto-migrate remains disabled; a migration is executed only through its
  approved procedure.
- [ ] A restore point and the corresponding recovery owner are recorded before
  any data-changing migration.
- [ ] The intended API and UI image tags are immutable and match the recorded
  source SHA.
- [ ] A controlled tenant, store, source product, and `save_draft` validation
  case are available. Do not enable customer publishing merely because the
  deployment checks are green.

## Deployment

1. Require the `production` environment approval in GitHub Actions.
2. Run **ListingKit API Deploy** with `source_ref` set to the recorded SHA and
   `image_tag` set to the immutable tag. The tag trigger is
   `listingkit-api-v*`; `workflow_dispatch` is preferred for an RC because it
   makes the chosen SHA explicit.
3. Wait for the API rollout to complete, then record its deployed image:

   ```powershell
   kubectl -n task-processor get deployment product-listing-api -o wide
   ```

4. Run **ListingKit UI Deploy** against the same `source_ref` and immutable
   `image_tag`. Its tag trigger is `listingkit-ui-v*`.
5. Wait for the UI rollout and record its image:

   ```powershell
   kubectl -n task-processor get deployment listingkit-ui -o wide
   ```

For a release containing the dependency readiness endpoint, API source and the
Kubernetes manifest must be deployed together: readiness uses `/readyz`, while
liveness and startup continue to use `/health`.

## Post-deploy gate

- [ ] Both deployments show `AVAILABLE=1` and their expected immutable images.
- [ ] `/health` is reachable as a liveness check.
- [ ] `/readyz` returns HTTP 200 through the in-cluster service proxy.
- [ ] An authenticated operator confirms ListingKit settings health and the
  controlled-tenant preflight.
- [ ] Run one controlled `save_draft` smoke test and record task ID, attempt
  ID, remote draft ID, readiness result, and final status under
  `docs/product/validation/runs/`.
- [ ] Observe API logs and task recovery signals for the agreed window. Do not
  expand the customer allowlist while the smoke test or observation window is
  unresolved.

Use the following read-only probe for the API endpoints without exposing the
service publicly:

```powershell
kubectl get --raw /api/v1/namespaces/task-processor/services/http:product-listing-api:8085/proxy/health
kubectl get --raw /api/v1/namespaces/task-processor/services/http:product-listing-api:8085/proxy/readyz
```

`settings-health` is authenticated and must be checked with an operator token;
do not put that token in this document or in a command history.

## Rollback and recovery

If rollout or smoke validation fails, stop further customer enablement and
record the failure before acting.

1. Roll back API and/or UI using their respective GitHub Actions workflow with
   the previously recorded immutable image tag.
2. Wait for the affected rollout and rerun the liveness/readiness probes.
3. For a schema or data incompatibility, use the approved roll-forward or
   restore procedure. Do not attempt an unreviewed direct database rollback.
4. Attach the failed run, deployed tags, probe results, task/attempt IDs, and
   recovery decision to the validation record.

The workstation `kubectl set image` procedure remains an emergency-only path;
after using it, restore the workflow-driven release history with a normal
deployment run.
