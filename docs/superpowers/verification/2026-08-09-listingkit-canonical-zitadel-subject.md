# ListingKit canonical ZITADEL-subject verification

Date: 2026-08-10 (fresh local verification at `9ed8198d9`)

## Scope under review

Reviewed commits `c71ce2e1e` through `9ed8198d9` against base
`877fcfee344c534f62336cf7226fc9df97121226`. The range canonicalizes
ListingKit identity to the verified ZITADEL subject, removes owner-scope
configuration, adds an identity-preflight release gate, scopes workload secret
references, and includes follow-up fixes for the two prior review findings.

The follow-up repairs are `3a84bad95` (owner scope enabled by default with no
production disable setter) and `9ed8198d9` (legacy username allowlists are
validation traps and fail closed). They were independently reviewed before
this fresh verification run.

## Fresh local verification

| Command | Result |
| --- | --- |
| `go test ./internal/listingkit ./internal/listingadmin ./internal/core/config ./internal/listingkit/httpapi -count=1` | PASS, exit 0, 5.35 s. Covers owner-scope default-invariant, config, and HTTP boundary behavior. |
| `go test ./internal/core/config -run 'Test(ValidateConfigRejectsLegacyListingKitUsernameAllowlistWithoutExposingValues|ListingKitLegacyUsernameAllowlistInputsFailClosed)$' -count=1` | PASS, exit 0, 1.52 s. Covers legacy YAML, both environment variables, and blank-primary shadowing. |
| `go test ./internal/listingkit/httpapi -run 'TestListingKitZitadelAuthFailsClosedWhenOnlyLegacyUsernameAllowlistMatchesSubject$' -count=1` | PASS, exit 0, 3.69 s. Covers the direct API boundary. |
| `go vet ./...` | PASS, exit 0, 10.91 s. |
| `go test ./... -count=1 -timeout=30m` | FAIL, exit 1, 102.38 s. The only failing package was `internal/crawler/alibaba1688`: its legacy tenant-bridge test observed the known missing metadata-table condition and disabled the bridge. `git diff --name-only 877fcfee344c534f62336cf7226fc9df97121226..HEAD -- internal/crawler/alibaba1688` had no output, so this failure does not overlap the review range. `internal/sheinlogin` passed in this run and also has no paths in that range. |
| `npm.cmd test -- --run` from `web/listingkit-ui` | PASS, exit 0, 258 files and 1486 tests, 92.60 s. |
| `npm.cmd run typecheck` from `web/listingkit-ui` | PASS, exit 0, 10.40 s. |
| `npm.cmd run lint` from `web/listingkit-ui` | PASS, exit 0, 16.67 s; 0 errors and 14 warnings. |
| `npm.cmd run build` from `web/listingkit-ui` | PASS, exit 0, 8.73 s. |
| `docker build -f deployments/docker/Dockerfile.product-listing-api -t task-processor/listingkit-api:canonical-subject-verify .` | PASS, exit 0, 48.97 s. |
| `docker run --rm --entrypoint /app/listingkit-identity-preflight task-processor/listingkit-api:canonical-subject-verify -help` | PASS, exit 0, 0.45 s; the packaged preflight executable exposed its config and log-level options. |
| `kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod` | PASS, exit 0, 0.23 s. The in-memory render had no owner-scope configuration match. |
| Client dry-run of the API, UI, image-proxy, worker, two schema-migration, and identity-preflight manifests | PASS, exit 0 for all seven files (1.30--1.35 s each). |
| `C:\Program Files\Git\bin\bash.exe scripts/tests/listingkit-identity-preflight-job-test.sh` | PASS, exit 0, 0.75 s. |

The generic `bash` command did not execute the script because this Windows
environment resolves it to an unavailable WSL launcher (`/bin/bash` absent).
The Git Bash rerun above executed the same repository script successfully.

The all-package Go result is not a passing repository-wide suite. No product
code was changed to hide the unrelated crawler baseline failure.

## Static identity, configuration, and release-boundary checks

- No production `ConfigureOwnerScopeRequired` references remain in ListingKit
  or ListingAdmin. Both packages initialize owner scoping to enabled; the only
  disabling facility is named and documented for tests.
- No production owner-scope configuration name or environment name was found
  in config, deployments, development documentation, or HTTP bootstrap code.
- No production expression reinterprets `AllowedUsernames` as canonical user
  IDs. The legacy YAML key and environment names remain only to detect and
  reject obsolete configuration without logging its value.
- `rg -n 'user_id.*\?\?.*sub|UserID.*firstNonEmpty|businessUserId.*subject|GetHeader\("X-User-ID"\)' internal/listingkit web/listingkit-ui/src` found only test fixtures/assertions. The production compatibility header inventory remains limited to verified downstream transport and the legacy no-auth-context fallback; protected actor and authorization paths use authenticated request context.
- The image build contains the preflight executable. The deployment workflow
  runs the one-shot preflight driver before its API image-update step.
- The production overlay render contains no owner-scope setting. UI, worker,
  image proxy, and migrations use explicit secret-key references; API and the
  read-only preflight use their documented shared configuration source. No
  secret values were printed or recorded.

## Limits and remaining external gates

This is local test/build/manifest evidence, not deployment, runtime, or
business acceptance. Because the fresh repository-wide Go command failed,
this note does not claim a fully passing repository suite. Remaining gates are:

1. Run the target-environment identity preflight against real persisted owner
   data and the authorized directory endpoint.
2. Perform the coordinated API and matching UI rollout only after preflight
   passes.
3. Execute the real-token role and tenant/owner matrix.
4. Perform the audited platform-mutation acceptance flow.

No push, pull request, deployment, merge, live token use, or platform mutation
was performed.
