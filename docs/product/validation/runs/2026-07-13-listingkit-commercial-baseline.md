# ListingKit Commercial Readiness Baseline

## Scope

- Date: 2026-07-13
- Immutable commit: `0f9e16c0fd28e7092ac7d6d68d16db6546a79140`
- Commit subject: `ci: add commercial readiness validation workflow (#21)`
- Workflow: [ListingKit Commercial Readiness run 29226386359](https://github.com/qq550723504/task-processor/actions/runs/29226386359)
- Result: **pass**

This is the first PAY-001 baseline run for the exact merged PAY-000 commit. The workflow checked out the supplied full SHA; it did not use a branch head or modify repository contents.

## Recorded environment and dependency inputs

| Input | Recorded value |
| --- | --- |
| Go | `go1.26.0 linux/amd64` |
| Node | `v22.23.1` |
| npm | `10.9.8` |
| `go.mod` SHA-256 | `f3aa4b1a875a5f4d73f08c75afa51527cde4e55c493cfbc0b9f92f6ffe98c730` |
| `go.sum` SHA-256 | `51eb3b0672e0b39bea5ed9b8b06004acd20df80dbadacbd72fded610bdd69b22` |
| `web/listingkit-ui/package-lock.json` SHA-256 | `7e0c674b737f9d929f033f6f71e9fc452ccc277f5f4e87429c633b5fd35bafee` |

## Validation results

| Area | Evidence collected | Outcome |
| --- | --- | --- |
| Target metadata | Full SHA validation, detached checkout, tool versions and lockfile checksums | Pass |
| Backend | `go test ./... -count=1`; Listing Control Plane and listingadmin race tests; `make build-all` | Pass |
| Frontend | Dependency install; lint; typecheck; test; production build | Pass |
| Containers | API and ListingKit UI Docker builds, both without image push | Pass |
| Kubernetes | Production ListingKit Workbench Kustomize render | Pass |
| Summary gate | Aggregate result of all prerequisite jobs | Pass |

No job failed, so this baseline requires no risk-domain remediation or closure rerun.

## Preserved evidence

All artifacts are attached to [run 29226386359](https://github.com/qq550723504/task-processor/actions/runs/29226386359#artifacts):

- `commercial-readiness-metadata-29226386359`
- `commercial-readiness-backend-29226386359`
- `commercial-readiness-frontend-29226386359`
- `commercial-readiness-images-29226386359`
- `commercial-readiness-manifests-29226386359`
- `commercial-readiness-summary-29226386359`

The metadata artifact contains the checked-out commit record and the exact tool and dependency values above. The remaining artifacts retain command logs, rendered manifest, per-domain failure classification, and the aggregate summary.

## Boundary and follow-up

This run establishes build and validation evidence only. It does not certify paid-pilot readiness: the identity, tenant isolation, submission-safety, entitlement, data-protection, operational, and real-integration gates in the execution plan remain open. The next queued item is PAY-040, which defines the paid-pilot product catalog and usage policy.
