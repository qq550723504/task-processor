# ListingKit Release-Gate Review Fixes Design

## Goal

Close the four unresolved PR #109 release-gate findings without weakening the
canonical ZITADEL-subject or least-privilege boundaries.

## Scope

This design changes only the identity-preflight owner inventory, release-image
addressing, preflight runner, Kubernetes secret projection, release tooling,
and their tests and operating documentation. It does not alter user data,
perform a deployment, or grant new platform permissions.

## Invariants

1. A persisted owner row with a non-empty tenant and a blank owner blocks the
   gate. It is reported as `missing_subject`, never silently excluded.
2. Every API image passed to either the preflight attestation or Deployment is
   a digest reference of the form `repository@sha256:<64 hex characters>`.
   Tags, including non-`latest` tags, are not release candidates.
3. The preflight executable runs in a distinct digest-pinned runner image built
   from the current gate tooling source. It never depends on the rollback API
   candidate containing that executable.
4. The preflight Job receives exactly five database keys, `ZITADEL_ISSUER_URL`,
   and `TASK_PROCESSOR_LISTINGKIT_ZITADEL_TENANT_DIRECTORY_TOKEN` from
   `listingkit-workbench-secret`. It does not import that Secret with `envFrom`
   and never receives invitation, AI, storage, queue, or browser credentials.

## Design

### Ownerless historical data

The owner aggregate query continues to reject blank tenants, but retains blank
user columns by selecting their normalized text and grouping it. The service
creates a deterministic blocking finding for an empty subject before any
directory lookup. Its output uses the existing redacted fingerprint format and
the explicit reason `missing_subject`; the return remains a typed blocking
error. Valid non-empty subjects preserve the existing one-directory-request-
per-tenant behavior.

### Digest-addressed release candidates

The shared image validator accepts only a fully-qualified digest reference.
The CI build job publishes the candidate tag for operator discoverability, but
exports the `docker/build-push-action` digest and constructs the canonical
`repository@digest` address for every gate and Deployment operation. The local
PowerShell release helper resolves the pushed image's repository digest and
passes that same value to both drivers. The UI remains outside the identity
preflight contract and keeps its existing rollout path.

### Independent preflight runner

A small dedicated Docker image contains only the statically linked
`listingkit-identity-preflight` binary, its required configuration, and system
CA certificates. The workflow builds and pushes it from the current workflow
tooling revision, exports its digest address, and the Job uses that runner
image. The Job is annotated with the candidate API digest so logs and cluster
objects attest which release it gated. Rollback tooling uses the current gate
runner, so it can verify a legacy API image that predates the preflight binary.

### Least-privilege Job environment

The Job keeps the shared non-secret ConfigMap import, then lists explicit
`secretKeyRef` entries for the five database keys, issuer, and read-only
directory token. All entries are required. This follows the repository's
existing migration and worker allowlist pattern.

## Error Handling

- A blank owner, malformed image, missing digest, missing runner image, missing
  Secret key, directory error, metadata bridge error, or Kubernetes Job failure
  fails closed before API mutation.
- Digest resolution never prints credentials or full Secret values.
- Existing report redaction remains in force for tenant and subject values.

## Verification

- Go SQL-mock and service tests cover blank owners as deterministic blockers.
- Shell driver tests reject tags and assert candidate/runner digest rendering.
- Parsed workflow and Kubernetes manifest tests assert digest data flow,
  runner/candidate separation, preflight ordering, and exact Secret allowlist.
- The PowerShell test suite asserts local digest resolution and fail-closed
  manual release/rollback ordering.
- Run targeted suites, `go test ./tests`, `go test ./... -count=1`, Docker
  builds for the API and preflight runner, `kubectl` client dry-runs, and the
  PR checks after push.
