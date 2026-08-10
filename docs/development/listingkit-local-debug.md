# ListingKit Local Debug

ListingKit local scripts require the same ZITADEL authentication flow as every
other environment. There is no local identity, trusted identity header, or
authentication-disable mode.

## Start the local stack

Run the local stack normally:

```powershell
.\scripts\start-listingkit-local-dev.ps1
```

Before starting, provide a valid local ZITADEL issuer/client configuration in
`.env` (or inherited process environment). The API returns `503` for protected
routes until `ZITADEL_ISSUER_URL` and `ZITADEL_CLIENT_ID` are configured, and
then requires a verified bearer token. The UI redirects unauthenticated users
through the configured ZITADEL login flow.

Start each side separately when needed:

```powershell
.\scripts\start-listingkit-local-api.ps1
.\scripts\start-listingkit-local-ui.ps1
```

## Direct API Calls

For authenticated API calls, prefer the local machine-token helpers instead of
repeatedly copying browser tokens:

```powershell
.\scripts\listingkit-fetch-machine-token.ps1 -ApiBaseUrl http://localhost:8085
.\scripts\listingkit-auth-check.ps1
```

The helper writes the token under `.local\listingkit-api-token.txt` and can export `LISTINGKIT_API_TOKEN` for scripted checks.

## Identity preflight before a coordinated release

Before deploying the canonical-subject API or its matching UI, run the
read-only identity preflight against the target Kubernetes namespace with the
exact immutable API image tag:

```bash
bash scripts/listingkit-identity-preflight-job.sh \
  --manifest deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml \
  --namespace task-processor \
  --image-tag "<immutable-release-tag>"
```

The shared tenant-directory credential must be able to read `POST /v2/users`
for every ZITADEL organization represented in the database. It does not need
user or membership write access, and the preflight Job does not receive the
dedicated member-invitation write token. The preflight only reads persisted
owner mappings and the ZITADEL directory; it never changes data.

In Kubernetes, only the API and this preflight import the shared Secret. The
UI, SHEIN login worker, imgproxy, and both schema migration Jobs use explicit
per-key `secretKeyRef` allowlists and receive neither the directory token nor
the member-invitation token. Database settings are the five
`TASK_PROCESSOR_DATABASE_{HOST,PORT,USER,PASSWORD,NAME}` keys;
`TASK_PROCESSOR_DATABASE_DSN` is not a supported core-config binding.

Success ends with `status=ok identity_preflight=passed`. A blocker is reported
as `status=blocked`, a table name, 12-hex SHA-256 fingerprints for tenant and
owner, an aggregate row count, and `reason=unknown_subject`. This means the
persisted owner is not a current ZITADEL `sub` in the same organization. Stop
and investigate; do not add a compatibility fallback or rewrite data from the
report. Never paste a complete tenant ID, subject, personal identifier, token,
or other secret into an issue or release log.

The controlled order is: confirm legacy `user_id` is absent or equals `sub`,
pass preflight, deploy and verify the API, deploy the matching UI, then run the
real-token role/owner-scope checks. A partial API/UI rollout is not acceptance.
Rollback is allowed only when the same `user_id` prerequisite has been
confirmed; the read-only preflight itself needs no data rollback.

## Safety

Do not use caller-supplied `X-User-*` or tenant headers as identity. ListingKit
derives identity from the verified ZITADEL bearer token in every environment.

Owner filtering is a fixed ListingKit startup invariant. Tests may use their
package-local state helpers to model legacy data, but local configuration and
environment variables cannot disable owner filtering.
