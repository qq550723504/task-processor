# ListingKit canonical ZITADEL-subject verification

Date: 2026-08-10 (final local verification from base `877fcfee344c534f62336cf7226fc9df97121226` through the scoped working-tree closure)

## Scope under review

Reviewed all committed changes from base
`877fcfee344c534f62336cf7226fc9df97121226` through `f3af69aa0`, plus the
scoped manual-deploy closure in the working tree. The range canonicalizes
ListingKit identity to the verified ZITADEL subject, removes owner-scope
configuration, adds an identity-preflight release gate, scopes workload secret
references, and includes follow-up fixes for the two prior review findings.

The follow-up repairs include `3a84bad95` (owner scope enabled by default with
no production disable setter), `9ed8198d9` (legacy username allowlists are
validation traps and fail closed), `8d748f739` (strict non-creating read-only
preflight database connections), `35b3bb313` plus `0f8aba6dc` (legacy tenant
organization normalization through the distinct metadata database), `c0d75c62e`
(immutable CI API apply), and `f3af69aa0` (bounded directory client and
canonical-sub documentation). This final closure additionally unifies every
documented API deploy or rollback path on the same full immutable-image
preflight and apply drivers.

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

## Final local refresh after closing documented deployment and rollback bypasses

The documented PowerShell manual-deploy helper and workstation rollback now
reuse the same Bash identity-preflight and immutable-API-apply drivers as CI.
The preflight Job renders the full exact candidate image, so a custom registry
cannot cause it to verify one image and deploy another. This evidence was
collected from the final working tree immediately before its scoped commit.

| Command | Result |
| --- | --- |
| `Invoke-Pester .\\scripts\\build-push-deploy-listingkit-workbench.Tests.ps1 -EnableExit` | PASS, exit 0; 8 tests. The real helper ran against stubbed external commands: `-SkipApply` made no Kubernetes call or Bash-resolution attempt; normal order was preflight, immutable API apply, then UI update; preflight or API-apply failures stopped every later Deployment/Kubernetes mutation; explicit blank and `latest` candidates were rejected before any external command; custom registries passed the same full image to both drivers. |
| `C:\\Program Files\\Git\\bin\\bash.exe scripts/tests/listingkit-identity-preflight-job-test.sh` and `...listingkit-apply-api-deployment-test.sh` | PASS, exit 0. The preflight driver rendered a complete custom-registry image rather than inferring a fixed registry; both release-driver behavior suites passed. |
| `go test ./internal/infra/database ./internal/app/runtime/listingkitidentitypreflight ./internal/listingkit/identitypreflight ./internal/listingkit/userdirectory ./internal/tenantbridge ./internal/listingkit/httpapi ./internal/listingkit ./internal/listingadmin ./internal/core/config ./tests -count=1` | PASS, exit 0. |
| `go vet ./...` | PASS, exit 0. |
| `go test ./... -count=1 -timeout=30m` | FAIL, exit 1, 90.5 s. The only failure remains `internal/crawler/alibaba1688/TestVerifiedCrawlerTenantResolverUsesLegacyTenantBridge` when its metadata table is absent and the legacy bridge is disabled. No review-range path touches `internal/crawler/alibaba1688`; `internal/sheinlogin` passed. This is recorded as a failing full suite, not a pass. |
| UI `npm.cmd test -- --run`, `typecheck`, `lint`, `build` | PASS, exit 0. Vitest: 258 files / 1486 tests. Lint: 0 errors and 14 existing warnings. |
| API Docker build plus `docker run --rm --entrypoint /app/listingkit-identity-preflight ... -h` | PASS, exit 0, 56 s. One prior Docker Hub metadata TLS timeout occurred before compilation; a single rerun succeeded with the same current source and the freshly built runtime image includes the preflight executable. |
| Production Kustomize render plus client dry-runs of overlay, identity-preflight Job, and API Deployment | PASS, exit 0. The generated-name Job was validated with `kubectl create --dry-run=client`, matching the release driver's create semantics. |
| Static release-boundary search | PASS. The manual helper, rollback instructions, and `listingkit-deploy.yml` contain no raw API `apply -k`, `set image deployment/product-listing-api`, `--image-tag`, or API `latest` mutation. |

The remaining target-environment preflight, coordinated rollout, real-token
authorization matrix, and audited platform-mutation acceptance remain external
release gates. No external state was changed by this verification.
