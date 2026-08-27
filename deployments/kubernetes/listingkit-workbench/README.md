# ListingKit Workbench K3S Deployment

This deploys the customer SHEIN Studio workbench as two services:

- `product-listing-api`: Go API on port `8085`.
- `listingkit-ui`: Next.js UI on port `3000`.

The browser talks to the Next.js UI. The UI proxies normal
`/api/listing-kits/*` and `/api/sds/*` requests to `product-listing-api`
through the cluster Service. The sole backend Ingress exception is the signed
ZITADEL SMS webhook at
`/api/v1/listing-kits/integrations/zitadel/sms`; it is routed directly to the
API before the UI catch-all.

## Prerequisites

- K3S cluster with Traefik Ingress enabled.
- Docker login to the target registry.
- `kubectl` context pointing at the target K3S cluster.
- Existing external services configured for DB, RabbitMQ, S3-compatible storage, SDS, SHEIN, and image generation.
- Reachable Temporal frontend for ListingKit workflows.

## Configure secrets

Create a real Secret from your secret manager or copy the example and fill it outside Git:

```powershell
Copy-Item deployments/kubernetes/listingkit-workbench/base/secret.example.yaml tmp/listingkit-workbench-secret.yaml
Copy-Item deployments/kubernetes/listingkit-workbench/base/member-invitation-secret.example.yaml tmp/listingkit-member-invitation-secret.yaml
kubectl apply -n task-processor -f tmp/listingkit-workbench-secret.yaml
kubectl apply -n task-processor -f tmp/listingkit-member-invitation-secret.yaml
```

Do not commit the filled secret file.

The application does not consume `TASK_PROCESSOR_DATABASE_DSN`. Before
applying this least-privilege manifest revision, make sure the existing
`listingkit-workbench-secret` contains the five bound database keys from the
example: `TASK_PROCESSOR_DATABASE_HOST`, `TASK_PROCESSOR_DATABASE_PORT`,
`TASK_PROCESSOR_DATABASE_USER`, `TASK_PROCESSOR_DATABASE_PASSWORD`, and
`TASK_PROCESSOR_DATABASE_NAME`. Copy the existing connection values in the
secret manager without printing or changing them; do not rely on the baked
example database settings.

The shared Secret name is reused, but its values are not shared with every
Pod. `product-listing-api` and the release-scoped identity preflight import the
whole Secret because they need the database plus API/read-only-directory
configuration. Every other manifest has an explicit key allowlist:

- `listingkit-ui`: Auth.js secret, ZITADEL issuer/client credentials, role
  allowlist, and demo webhook only. Public origins and redirect URIs come from
  `listingkit-workbench-config`.
- `shein-login-worker`: the five database keys and four SHEIN cookie Redis
  keys only. Each queued account already supplies its tenant and store id.
- `imgproxy`: signing key/salt and the two ProductImage S3 credential keys.
- both schema migration Jobs: the five database keys only.

None of those five workloads receives the read-only directory token or the
member-invitation write token. The worker and migration binaries use scoped
configuration loading so they do not require unrelated OpenAI, RabbitMQ,
Amazon, or ZITADEL credentials merely to pass startup validation.

When migrating an existing deployment, create
`listingkit-member-invitation-secret` with both the token and project id.
Then have the approved secret manager remove both invitation keys from the
already deployed shared Secret. Run **ListingKit API Deploy** with the current
approved immutable candidate and allow its exact-attempt **ListingKit UI
Deploy** gate to complete; those workflows own the API and UI restarts. The API
Secret reference is required before that release, because a missing token must
fail Pod startup instead of falling back to a legacy shared Secret value.

After both gated workflows succeed, restart the remaining non-release
consumers so no older imgproxy or SHEIN login Pod retains the removed write
token in its process environment:

```powershell
kubectl -n task-processor rollout restart deployment/imgproxy deployment/shein-login-worker
kubectl -n task-processor rollout status deployment/imgproxy --timeout=5m
kubectl -n task-processor rollout status deployment/shein-login-worker --timeout=5m
```

Do not inspect, decode, or print Secret values during this migration.

Required ZITADEL values:

```text
ZITADEL_ISSUER_URL=https://auth.example.com
ZITADEL_CLIENT_ID=<oidc-web-client-id>
ZITADEL_CLIENT_SECRET=<oidc-web-client-secret>
TASK_PROCESSOR_LISTINGKIT_ZITADEL_TENANT_DIRECTORY_TOKEN=<read-only-tenant-directory-token>
# API-only listingkit-member-invitation-secret:
TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN=<dedicated-member-invitation-token>
# API-only listingkit-member-invitation-secret:
TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=<existing-listingkit-project-id>
# Auth.js callback URI:
# https://<workbench-host>/api/auth/callback/zitadel
ZITADEL_POST_LOGOUT_REDIRECT_URI=https://<workbench-host>
NEXT_PUBLIC_ZITADEL_CONSOLE_URL=https://auth.example.com/ui/console
```

Use three separate credentials. `ZITADEL_CLIENT_SECRET` is only the Auth.js
OIDC web-client secret. The tenant-directory token is read-only and is only for
listing and validating tenants. The member-invitation token is a dedicated
write-capable service-account token; never copy the OIDC client secret or the
tenant-directory token into
`TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN`.

Keep `urn:zitadel:iam:user:resourceowner` in `ZITADEL_SCOPES`; ListingKit uses
that claim as the tenant id. The Go API reads ZITADEL settings from core
config `listingkit.zitadel.*`; in Kubernetes we currently populate those config
keys through env binding such as `ZITADEL_ISSUER_URL` and
`ZITADEL_CLIENT_ID`. Authentication is mandatory; missing issuer/client
configuration fails closed for protected ListingKit routes.
Owner filtering is a fixed ListingKit startup invariant; deployment
configuration cannot disable it. For the full migration checklist, including
allowlist checks, see
[listingkit-config-migration-checklist.md](/D:/code/task-processor/docs/development/listingkit-config-migration-checklist.md).

## Provision ZITADEL roles

Provision roles as a deployment/operator step, not from the normal UI or Go API
startup path. The provisioning command needs a ZITADEL Management API token and
therefore should run from CI, an admin workstation, or a short-lived Kubernetes
Job with tightly scoped secrets.

```powershell
$env:ZITADEL_ISSUER_URL = "https://auth.example.com"
$env:ZITADEL_MANAGEMENT_TOKEN = "<management-api-token>"
$env:ZITADEL_ORG_ID = "<org-id>"
go run ./cmd/listingkit-zitadel-provision -project-name ListingKit -create-project
```

For an existing project, pass the project id instead of allowing creation:

```powershell
go run ./cmd/listingkit-zitadel-provision `
  -issuer-url https://auth.example.com `
  -token "<management-api-token>" `
  -org-id "<org-id>" `
  -project-id "<project-id>"
```

The command ensures these project roles exist:

```text
listingkit_viewer
listingkit_operator
listingkit_admin
platform_admin
```

Copy the command output into the workbench shared Secret/config:

```text
TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=<project-id>
TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS=
TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS=
TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES=listingkit_admin,listingkit_operator,listingkit_viewer,platform_admin
ZITADEL_SCOPES=<printed scope string>
TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHZ_REQUIRED=1
```

`listingkit_viewer` users see the main workflow menu only.
`listingkit_operator` users also see operational data menus.
`listingkit_admin` users see all ListingKit menus.
`platform_admin` is kept for platform administration compatibility.

The Go API enforces the same role model as the sidebar. If a user can sign in
but receives `listingkit_role_denied`, confirm the OIDC runtime uses the printed
`ZITADEL_SCOPES` value so access tokens contain the ZITADEL project role claim.

## Configure member invitations

Create a dedicated ZITADEL service account for member invitations after the
ListingKit project and its roles exist. Limit it to the target pilot
organizations and the configured ListingKit project. It needs only these two
operations:

- create a human user in the selected organization (`POST /v2/users/human`);
- assign one existing ListingKit project role to that user
  (`AuthorizationService/CreateAuthorization`).

Do not grant tenant or project creation, role-definition changes, user deletion,
or unrelated administration permissions. The application accepts only
`listingkit_viewer`, `listingkit_operator`, or `listingkit_admin`; the invitation
flow cannot grant `platform_admin`.

Store the dedicated token and existing project id only in the API-only
`listingkit-member-invitation-secret`:

```text
TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN=<dedicated-service-account-token>
TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=<existing-listingkit-project-id>
```

Inject the token through the approved secret manager; never commit it, print it,
or paste it into the OIDC or tenant-directory fields. Do not add the dedicated
Secret to UI, worker, imgproxy, or migration Job `envFrom` lists. Apply the
dedicated Secret and verify its key names, then run **ListingKit API Deploy**
with the current approved immutable API candidate and its exact-attempt
**ListingKit UI Deploy** gate. Do not restart or apply the API from a
workstation: the base API image is a development default and is not a release
target.

```powershell
kubectl apply -n task-processor -f tmp/listingkit-member-invitation-secret.yaml

$requiredKeys = @(
  "TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN"
)
$secret = kubectl -n task-processor get secret listingkit-member-invitation-secret -o json |
  ConvertFrom-Json
$presentKeys = @($secret.data.PSObject.Properties.Name)
$missingKeys = @($requiredKeys | Where-Object { $_ -notin $presentKeys })
if ($missingKeys.Count -ne 0) {
  throw "Missing ListingKit invitation Secret keys: $($missingKeys -join ', ')"
}
$requiredKeys = @(
  "TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
  "TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID"
)
if (@($requiredKeys | Where-Object { $_ -notin $presentKeys }).Count -ne 0) {
  throw "Missing ListingKit invitation credentials in API-only Secret"
}
```

This validation inspects key names only and does not decode or print values.
After rollout, sign in as a `platform_admin`, select an existing target tenant
on the platform subscription page, invite a test user with
`listingkit_viewer`, and verify the response contains the expected tenant, user,
role, and authorization IDs. Confirm a `succeeded` row exists in
`listingkit_member_invitation_audits`, then have the user complete the emailed
verification flow and confirm the issued access token contains only the
intended ListingKit role for that tenant. These runtime checks are required;
a successful manifest render alone does not prove the ZITADEL permissions.

### Repair an incomplete invitation

`zitadel_member_invitation_incomplete` means ZITADEL created the human user and
sent the verification code, but the project role assignment failed. Do not
submit the invitation again: first repair the existing identity recorded by the
audit row.

1. Find the latest `listingkit_member_invitation_audits` row whose
   `error_code` is `zitadel_member_invitation_incomplete`. Read its `user_id`,
   `tenant_id`, `role`, `email`, and `created_at`; do not use a user id copied
   from the request or UI.
2. Confirm the recorded tenant is the intended target and the recorded role is
   one of `listingkit_viewer`, `listingkit_operator`, or `listingkit_admin`.
3. In ZITADEL Console or approved Management API tooling, create an
   authorization for that exact `user_id`, the configured
   `TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID`, the audit-recorded
   `tenant_id` as organization, and a single `roleKeys` entry equal to the
   audit-recorded role. Do not create another user and do not substitute
   `platform_admin`.
4. Verify ZITADEL returns an authorization id and shows that exact
   user/project/organization/role tuple. Have the user finish email verification
   and sign in, then verify the tenant and role claims and the expected
   least-privilege UI/API access.
5. Preserve the immutable incomplete audit row and record the repair operator,
   time, authorization id, and verification evidence in the incident/change
   record. Do not rewrite the row to make the original attempt look successful.

## Configure Tencent SMS relay for ZITADEL

This integration is intentionally a signed ZITADEL webhook plus the official
Tencent Cloud SMS API. ZITADEL owns verification codes and message templates;
the workbench only verifies the webhook signature and delivers the approved
message. Do not build or operate a second OTP system.

1. In Tencent Cloud SMS, complete required sign and template review first. Use
   the approved sign name and template ID issued by Tencent; confirm the
   template supports the ZITADEL notification values without placing a code or
   phone number in source control, tickets, or logs.
2. Use the ZITADEL Admin API to create an HTTP SMS Provider with the public
   HTTPS endpoint
   `https://<workbench-host>/api/v1/listing-kits/integrations/zitadel/sms`.
   Creating the provider returns its signing key once and leaves it inactive;
   do not activate it yet. The key must be shared only with the API-only
   Kubernetes Secret.
3. Have the approved secret manager materialize a Kubernetes Secret named
   `listingkit-tencent-sms-secret` from
   `base/tencent-sms-secret.example.yaml`, including the signing key returned
   in step 2 and the five Tencent SMS values. Apply the Secret before applying
   the API Deployment. Never place these values in
   `listingkit-workbench-secret`, a ConfigMap, UI, worker, imgproxy, or a
   migration Job. Inspect only key names, never decode or print values:

   ```powershell
   kubectl apply -n task-processor -f <secret-manager-output>.yaml
   $requiredKeys = @(
     "TASK_PROCESSOR_LISTINGKIT_ZITADEL_SMS_SIGNING_KEY",
     "TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_SECRET_ID",
     "TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_SECRET_KEY",
     "TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_APP_ID",
     "TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_SIGN_NAME",
     "TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_TEMPLATE_ID"
   )
   $secret = kubectl -n task-processor get secret listingkit-tencent-sms-secret -o json | ConvertFrom-Json
   $presentKeys = @($secret.data.PSObject.Properties.Name)
   if (@($requiredKeys | Where-Object { $_ -notin $presentKeys }).Count -ne 0) {
     throw "Missing Tencent SMS credentials in API-only Secret"
   }
   ```

4. Publish the API-only SMS path only through `ListingKit API Deploy` after the
   Secret is populated. That workflow runs migrations, both image-agent worker
   rollouts, the finite canary, the API rollout, and the production Ingress in
   order, then emits the exact run-and-attempt attestation. Allow its automatic
   `ListingKit UI Deploy` gate to finish, or manually supply both
   `release_gate_run_id` and `release_gate_run_attempt` from that API execution.
   A workstation must not apply the API Deployment, UI Deployment, shared
   ConfigMap, or production Ingress.

5. After the API rollout and Ingress apply succeed, verify the provider endpoint
   and template mapping, then activate the provider. Do not activate it before
   the rollout: Secret updates do not refresh existing API Pod environment
   variables. Do not disable signature validation or add bearer authentication
   as a replacement; invalid, stale, or unsigned requests must fail.
6. Test once with a controlled disposable device and a non-production user.
   Confirm delivery, one-time-code verification in ZITADEL, and only masked
   audit/log data. Do not record the phone number or verification code. Then
   test a deliberately invalid signature and confirm no SMS is sent.

### Rotate Tencent SMS credentials

Rotate the Tencent credentials and ZITADEL signing key through the secret
manager. Update both the API Secret and ZITADEL Provider for the signing-key
change within the approved maintenance window, then restart only
`product-listing-api` and repeat the rendered-manifest and controlled-device
checks above. Do not roll the UI, workers, imgproxy, or migration Jobs: they
must never consume this Secret.

## CI/CD deploy

GitHub Actions is now the preferred release path for ListingKit Workbench.

For the Release Candidate gate, immutable image policy, post-deploy smoke
checks, and recovery decision, follow the
[ListingKit Release Candidate Runbook](../../../docs/operations/listingkit-release-candidate-runbook.md).

### First controlled deployment

Provision the namespace and real Secrets through the approved infrastructure
and secret-management paths. A workstation must not apply the production
ConfigMap, migration Jobs, API, UI, or Ingress. Start **ListingKit API Deploy**
with the exact `source_ref` and immutable API candidate; it owns the ordered
ConfigMap apply, both schema migrations, identity preflight, v2/v3 worker
rollouts, finite canary, API rollout, and Ingress. A failed migration or gate is
a No-Go: investigate and use an approved roll-forward or restore procedure
instead of deleting or editing the production schema manually.

After that API execution succeeds, let **ListingKit UI Deploy** start from its
automatic `workflow_run` gate. If a manual UI gate is required, supply both
`release_gate_run_id` and `release_gate_run_attempt` from that exact successful
API attempt. UI tag events build and push only; they never mutate production.

The ListingKit deployment workflow runs the ListingKit schema migration Job
before the identity preflight. For an environment carrying the reviewed
system-owned exception set, run the one-shot
`scripts/listingkit-owner-scope-exceptions.ps1` seeder after that migration and
before the preflight. The seeder validates the live report fingerprint and
counts, so an empty or newly changed exception set remains a release blocker;
the workflow never copies approved exceptions into an unrelated database.

### Identity preflight release gate

Every API release must pass the read-only identity preflight in the target
environment before either the API or its matching UI is updated. The directory
credential needs read access sufficient to query `POST /v2/users` for every
ZITADEL organization represented by persisted tenant-owned rows. It does not
need user-create, update, invitation, or membership-write permission. The Job
uses the shared ConfigMap and shared Secret for the database and read-only
directory credential; it never references the API-only member-invitation
Secret.

Legacy ListingAdmin owner rows use numeric tenant IDs. For those rows, the
preflight must find both `projections.org_metadata2` and
`projections.user_metadata5` in exactly one of the `zitadel_auth` or `zitadel`
PostgreSQL databases that share the configured database host and credentials.
Grant the Job account only `CONNECT` plus `SELECT` on both projection tables in
the selected database. Neither candidate, both candidates, or an unreadable
metadata table blocks the release without printing database connection details;
verify the one intended metadata database and both read-only grants before
retrying.

Run the tested driver with the exact full immutable API and preflight-runner
images. Copy each digest from the release build output; do not substitute a
mutable tag.

```bash
API_CANDIDATE_IMAGE="docker.io/xuwei190/task-processor-product-listing-api@sha256:<64-hex-api-digest>"
PREFLIGHT_RUNNER_IMAGE="docker.io/xuwei190/task-processor-listingkit-identity-preflight@sha256:<64-hex-runner-digest>"

bash scripts/listingkit-identity-preflight-job.sh \
  --manifest deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml \
  --namespace task-processor \
  --image "$API_CANDIDATE_IMAGE" \
  --runner-image "$PREFLIGHT_RUNNER_IMAGE"
```

The driver renders a temporary manifest, creates one generated Job, waits up to
15 minutes, and prints its logs. The Job's 900-second active deadline matches
that driver wait, so a timed-out release gate cannot leave the preflight
running indefinitely. A failure or timeout also prints `describe` output and
returns non-zero before any Deployment image update. The Job only reads owner
identifiers from the database and users from ZITADEL; it never mutates either
system.

Successful output ends with:

```text
status=ok identity_preflight=passed
```

A blocked finding has this safe shape:

```text
status=blocked table=listing_store tenant=sha256:<12-hex> owner=sha256:<12-hex> rows=3 reason=unknown_subject
```

`unknown_subject` means a persisted owner value is not a current ZITADEL `sub`
in the same organization. Stop the release and investigate the source mapping,
deleted account, or organization mismatch; do not add a fallback or rewrite
data automatically. Fingerprints, table names, and aggregate row counts are
safe correlation fields, but no complete tenant ID, subject, personal data, or
secret may be pasted into an issue, change record, or release log.

Treat the API and UI as one coordinated release:

1. Confirm the legacy `user_id` claim is absent or exactly equals `sub`.
2. Pass this preflight against the target environment.
3. Deploy and verify the canonical-subject API image.
4. Deploy the matching ListingKit UI/Auth.js image.
5. Complete the real-token role and owner-scope checks.

A partial API/UI rollout is not release acceptance. The preflight is
release-scoped and intentionally absent from the base Kustomization.

### One-time owner reconciliation

The release preflight is a blocker, not a migration command. For a database
that has already completed the legacy migration, run the reconciliation tool
in its default read-only mode and review the redacted report:

```powershell
pwsh -File scripts/listingkit-owner-scope-dry-run.ps1 -ConfigPath config/config-prod.yaml
```

The command uses the non-validating configuration loader and strict,
non-creating database connections because this job mounts only database and
ZITADEL directory settings. It writes only aggregate counts and short
fingerprints; it never writes raw tenant IDs, legacy IDs, subjects, tokens, or
SQL bodies. Unmapped and conflicting candidates remain unresolved and must be
handled explicitly; the tool never assigns an arbitrary current member.

Only after the report has been reviewed may an operator repeat the command with
`-Execute -ConfirmReport <exact-12-hex-report-fingerprint>`. The command
re-runs the read-only scan, compares the fingerprint before opening a write
transaction, and updates only blank owner fields for uniquely verified
candidates. Use a small `-BatchSize` first and preserve the report and command
output in the change record. A mismatch, missing confirmation, metadata error,
or unresolved candidate fails closed before any `UPDATE`.

The application migration installs a PostgreSQL `owner_user_id` check as
`NOT VALID`. That protects new inserts and updates without making the schema
migration fail on the historical ownerless set. Do not validate these
constraints until the reviewed reconciliation report shows both
`unresolved_rows=0` and `auto_rows=0`. After the controlled backfill, validate
each legacy owner-scoped table explicitly:

```sql
ALTER TABLE "listing_store"
  VALIDATE CONSTRAINT "ck_listing_store_owner_user_id_nonblank";
ALTER TABLE "listing_category"
  VALIDATE CONSTRAINT "ck_listing_category_owner_user_id_nonblank";
ALTER TABLE "listing_filter_rule"
  VALIDATE CONSTRAINT "ck_listing_filter_rule_owner_user_id_nonblank";
ALTER TABLE "listing_generation_topic_override"
  VALIDATE CONSTRAINT "ck_listing_generation_topic_override_owner_user_id_nonblank";
ALTER TABLE "listing_generation_topic_policy"
  VALIDATE CONSTRAINT "ck_listing_generation_topic_policy_owner_user_id_nonblank";
ALTER TABLE "listing_operation_strategy"
  VALIDATE CONSTRAINT "ck_listing_operation_strategy_owner_user_id_nonblank";
ALTER TABLE "listing_pricing_rule"
  VALIDATE CONSTRAINT "ck_listing_pricing_rule_owner_user_id_nonblank";
ALTER TABLE "listing_profit_rule"
  VALIDATE CONSTRAINT "ck_listing_profit_rule_owner_user_id_nonblank";
ALTER TABLE "listing_scheduled_task_config"
  VALIDATE CONSTRAINT "ck_listing_scheduled_task_config_owner_user_id_nonblank";
ALTER TABLE "listing_sensitive_word"
  VALIDATE CONSTRAINT "ck_listing_sensitive_word_owner_user_id_nonblank";
ALTER TABLE "listing_product_import_task"
  VALIDATE CONSTRAINT "ck_listing_product_import_task_owner_user_id_nonblank";
ALTER TABLE "listing_product_import_mapping"
  VALIDATE CONSTRAINT "ck_listing_product_import_mapping_owner_user_id_nonblank";
ALTER TABLE "listing_product_data"
  VALIDATE CONSTRAINT "ck_listing_product_data_owner_user_id_nonblank";
```

This validation is a separate operational change. It is not run by the
deployment workflow and must not be used to bypass the identity preflight.

The production overlay is steady-state desired state only. It contains the two
long-lived image-agent workers and contains no finite canary Job. A generic
`kubectl apply -k` is therefore not a release procedure: it cannot prove the
ordered migrations, worker rollouts, finite canary, API rollout, and UI
attestation gates. For a new cluster, render an immutable copy locally for
review and client-side validation, then run the API release workflow. That
workflow exclusively deletes, applies, and waits for the canary; the matching UI
release is admitted only by its uploaded exact-source attestation.

```powershell
$source = Resolve-Path deployments/kubernetes/listingkit-workbench
$stagingRoot = (Resolve-Path $env:TEMP).Path
$staging = Join-Path $stagingRoot ("listingkit-workbench-release-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $staging | Out-Null
Copy-Item -Recurse (Join-Path $source "*") $staging
Push-Location (Join-Path $staging "overlays/prod")
try {
  kustomize edit set image "xuwei190/task-processor-product-listing-api=docker.io/xuwei190/task-processor-product-listing-api:$tag"
  kustomize edit set image "xuwei190/task-processor-listingkit-ui=docker.io/xuwei190/task-processor-listingkit-ui:$tag"
  kustomize build . > rendered-listingkit-production.yaml
  kubectl create --dry-run=client --validate=false \
    -f rendered-listingkit-production.yaml -o name
} finally {
  Pop-Location
  if ((Resolve-Path (Split-Path $staging -Parent)).Path -ne $stagingRoot) {
    throw "Refusing to remove a staging directory outside the temporary root"
  }
  Remove-Item -Recurse -Force $staging
}
```

This command does not contact or mutate a cluster. Record the source SHA, image
digests, API workflow run ID, migration Job name, canary result, and rollout
output in the Release Candidate Runbook. All production mutations use the
GitHub Actions release workflows below rather than a generic overlay apply.

### Release workflows

- `ListingKit API Deploy` ([workflow](../../../.github/workflows/listingkit-deploy.yml))
- `ListingKit UI Deploy` ([workflow](../../../.github/workflows/listingkit-ui-deploy.yml))

Trigger rules:

- API tag `listingkit-api-v*` runs migrations, both worker rollouts, the finite
  canary, and API rollout, then uploads a 24-hour exact-source release
  attestation scoped to that API workflow run ID and run attempt.
- A successful `ListingKit API Deploy` run automatically starts the gated UI
  path for the exact attested source SHA.
- UI tag `listingkit-ui-v*` may build and push an image only; it never mutates
  production.
- Manual UI production release requires the explicit successful API
  `release_gate_run_id` and `release_gate_run_attempt`. It downloads only that
  attempt's artifact and fails closed for a missing/expired/malformed
  attestation, wrong workflow/conclusion/run/attempt or source, or non-digest
  API candidate.
- Branch, tag, image-tag convention, and "latest successful run" are not
  release evidence.
- `publish_latest` is not a release or rollback target.

Required GitHub repository secrets:

```text
DOCKERHUB_USERNAME
DOCKERHUB_TOKEN
KUBE_CONFIG
```

`KUBE_CONFIG` should be the full kubeconfig content for the target cluster, not a path.

Recommended GitHub environment setup:

```text
Environment name: production
Environment URL:  https://pod.shuomiai.com
```

If you want manual approval before production deployment, add protection rules to the `production` environment in GitHub.

The workflow uses:

- image tag: current commit short SHA by default
- Docker Hub namespace: `xuwei190`
- Kubernetes namespace: `task-processor`
- release overlays must be rendered with immutable image tags as shown above.

## Rollback

Use the same GitHub Actions workflows for rollback. There is no supported
workstation production fallback.

Standard rollback path:

1. Confirm the legacy ZITADEL `user_id` claim is absent or exactly equals
   `sub`; otherwise rollback can restore split ownership semantics and is not
   allowed.
2. Identify the prior API and UI image digests from a successful release record.
   Also identify the current `task-processor-listingkit-identity-preflight`
   runner digest; the gate runner is deliberately separate from the rollback
   candidate and must be available even when that candidate predates preflight.
3. Run `ListingKit API Deploy` with its prior immutable digest, then wait for the
   API rollout and readiness probe. The workflow then reapplies the standalone
   production SMS webhook Ingress only after that rollout succeeds.
4. Run `ListingKit UI Deploy` with the API rollback execution's explicit
   `release_gate_run_id` and `release_gate_run_attempt`; it downloads only that
   attempt's attestation, builds the exact attested source, and waits for the
   digest-pinned UI rollout.
5. Record the rollback decision, deployed tags, probe results, and any data
   recovery action in the validation run.

This reuses the same deployment logic as a normal release and keeps the
rollback auditable.

To find a rollback target:

- Check prior successful runs of `ListingKit API Deploy` and `ListingKit UI Deploy`.
- Or inspect the currently deployed / previously deployed image tags in Docker
  Hub or Kubernetes rollout history.

If GitHub Actions is unavailable, pause production mutation and restore the
gated release service. Internal rendering/apply helpers remain available to CI
and non-production validation, but are not supported production entry points.

## SHEIN POD image lookup index backfill

The `product-listing-api` image also contains the controlled
`/app/listingkit-shein-pod-image-index-backfill` maintenance binary. It requires
an explicit `-config` argument, migrates only the POD image lookup table, and
then rereads and locks each task before synchronizing its index row. It does not
run the unrelated ListingKit runtime migrations.

After deploying an image that contains the POD image lookup index:

1. Copy
   `deployments/kubernetes/listingkit-workbench/jobs/pod-image-index-backfill-job.yaml`
   to a temporary file outside the repository.
2. Replace `REPLACE_WITH_DEPLOYED_TAG` with the exact immutable tag currently
   deployed for `product-listing-api`. Do not use `latest`.
3. Create and follow the one-shot Job:

```powershell
$jobFile = Join-Path $env:TEMP "listingkit-pod-image-index-backfill-job.yaml"
Copy-Item deployments/kubernetes/listingkit-workbench/jobs/pod-image-index-backfill-job.yaml $jobFile
(Get-Content -Raw $jobFile).Replace("REPLACE_WITH_DEPLOYED_TAG", "<deployed-commit-tag>") |
  Set-Content -NoNewline $jobFile
$jobName = kubectl create -f $jobFile -o jsonpath='{.metadata.name}'
kubectl -n task-processor wait --for=condition=complete "job/$jobName" --timeout=30m
kubectl -n task-processor logs "job/$jobName"
```

Successful stdout is a single machine-readable line such as
`processed=12500 duration=42s`. The skipped-malformed count and safe per-row
diagnostics (`task_id`, JSON field name, and reason only) are written to stderr;
persisted request/result JSON is never logged. A failed Job may be safely rerun
after fixing the cause; upserts are idempotent. Verify a known full SKU through
`GET /api/v1/listing-kits/shein-pod-image-lookup/stores/<store_id>?q=<sku>`,
then inspect API/proxy logs for errors. Owner filtering remains enabled while
you complete backfill and tenant/user sampling.

## Image-agent Temporal v2/v3 recovery rollout

Manual image generation has two long-lived, capability-isolated worker
Deployments. `image-agent-temporal-worker` runs the frozen v2 registrations on
`image-agent-manual`; `image-agent-temporal-worker-v3` runs only v3
registrations on `image-agent-manual-v3`. Both use the same immutable API
candidate image, but each process receives an explicit `-wire-mode` and
`-task-queue`. Keep both Deployments present. The v2 worker is removed only
after the live drain evidence below reaches zero.

The deploy workflow preserves the identity preflight, ingress, and
digest-pinned image gates, then performs this order:

1. Run the additive Product Listing and ListingKit schema migrations.
2. Apply, restart, and wait for the v2 compatibility worker.
3. Apply, restart, and wait for the v3 recovery worker.
4. Delete and recreate the finite `image-agent-temporal-v3-canary` Job with the
   same candidate image, then require `Complete` within its deadline.
5. Only after those gates apply/restart the API so new starts route to v3.

The canary runs `/app/image-agent-temporal-worker -canary` on the v3 queue. It
registers and executes only the side-effect-free compatibility workflow, has
`restartPolicy: Never`, bounded deadline/backoff, receives only Temporal
address/namespace config, and receives no Secret.

### Live preflight and v2 drain evidence

Use Temporal CLI v1.8.1 for this evidence procedure. Set address and namespace
without printing credentials, verify the pinned CLI version, and list both the
parent and child workflow types. `ImageAgentWorkflow` parents own run-state,
plan, pending-command, and approval Activities. Each `ImageSlotWorkflow` child
owns the slot execution and slot-result persistence Activities, so parent-only
describes are not drain evidence.

```bash
set -euo pipefail
: "${TEMPORAL_ADDRESS:?set the target Temporal address}"
: "${TEMPORAL_NAMESPACE:?set the target Temporal namespace}"
bash scripts/listingkit-image-agent-v2-drain-check.sh
```

The checked-in script pins Temporal CLI 1.8.1, validates every list/describe
command and JSON shape, enumerates `ImageAgentWorkflow` and
`ImageSlotWorkflow`, describes their exact workflow/run IDs, and reads each
execution's queue, `pendingChildren`, and `pendingActivities`. It counts only
the exact `image-agent-manual` queue and exact frozen legacy/`.v2` Activity
allowlist, including `imageagent.execute_slot` and
`imageagent.execute_slot.v2`.

Its stdout is exactly these deterministic safe counters:

```text
open_v2_parent_count=0
open_v2_child_count=0
pending_v2_child_count=0
pending_v2_activity_count=0
pending_v2_activity_attempt_sum=0
```

The script exits zero only when the first four counters are explicitly zero.
Any nonzero inventory, malformed/missing identity or queue, unexpected v2-queue
Activity, Temporal/jq failure, or unpinned CLI version exits nonzero. Retain the
v2 Deployment unless this executable gate exits zero; empty or partial output
is never drain evidence. Output contains counts only, never credentials,
object metadata, or presigned URLs.

Identify the `702d76631` rebound window from deployment records, not from Git
commit time: record the instant that image first became active and the instant
the correcting image became active. Use parameterized UTC timestamps and this
half-open query; it selects the complete persisted identity and constructs the
same full workflow ID used by production code:

```sql
\set rebound_start '2026-08-27T00:00:00Z'
\set rebound_end   '2026-08-28T00:00:00Z'

SELECT tenant_id, owner_user_id, id, created_at,
       format('image-agent:%s:%s:%s', tenant_id, owner_user_id, id) AS workflow_id
FROM image_agent_v2_runs
WHERE created_at >= :'rebound_start'::timestamptz
  AND created_at <  :'rebound_end'::timestamptz
ORDER BY created_at, tenant_id, owner_user_id, id;
```

For each row, the equivalent shell construction is
`workflow_id="image-agent:${tenant_id}:${owner_user_id}:${run_id}"`. Reconcile
that full ID, never bare `run_id`, against both list/describe inventories. Keep
ambiguous rows on the v2 inventory for manual reconciliation; never silently
move an in-flight history to v3.

### Rollback and durable staging retention

A rollback stops new v3 starts by rolling the API routing image/configuration
back through the gated workflow. It does **not** delete, scale down, or retarget
`image-agent-temporal-worker-v3`; that worker must finish histories already
started on `image-agent-manual-v3`. Keep the v2 worker too. The additive schema
migration is not rolled back while either worker can reference v3 records.

Only newly started v3 manual workflows receive the production-owned Temporal
`WorkflowExecutionTimeout` of 30 days. Timeout ends that workflow execution; it
does not authorize regeneration or deletion of staged bytes. Operators then
have a 7-day reconciliation allowance. The authoritative maximum durable
recovery window is therefore `30 days + 7 days = 37 days`. Existing/live
histories retain the options recorded when they were started, and committed old
histories remain replayed without a new workflow-history branch.

Configure the object store's lifecycle policy on `image-agent/staging/` outside
the application with a minimum retention of 45 days, which is strictly greater
than 37 days. Application request paths do not synchronously delete staging
objects. Run this redacted check with the same S3-compatible endpoint settings
used by operations; it prints only policy identity, status, prefix, and days:

```bash
set -euo pipefail
: "${STAGING_BUCKET:?set the staging bucket name without printing credentials}"
lifecycle_args=(s3api get-bucket-lifecycle-configuration --bucket "$STAGING_BUCKET")
if [[ -n "${S3_ENDPOINT:-}" ]]; then
  lifecycle_args+=(--endpoint-url "$S3_ENDPOINT")
fi
lifecycle_json="$(AWS_PAGER='' aws "${lifecycle_args[@]}")"
jq -e '
  [.Rules[] |
    select(.Status == "Enabled") |
    select((.Filter.Prefix // .Prefix // "") == "image-agent/staging/") |
    select((.Expiration.Days // 0) >= 45)] |
  length == 1
' <<<"$lifecycle_json" >/dev/null
jq -c '
  .Rules[] |
  select((.Filter.Prefix // .Prefix // "") == "image-agent/staging/") |
  {policy_id: .ID, status: .Status,
   prefix: (.Filter.Prefix // .Prefix), expiration_days: .Expiration.Days}
' <<<"$lifecycle_json"
```

Record the redacted output and infrastructure-policy revision as live evidence.

Operational evidence must never print Secret values, S3/COS credentials,
presigned URLs, provider headers, or full object metadata. Record only safe
counts, workflow/run IDs, activity names/attempts, phases, image digests,
rollout conditions, canary status, and redacted lifecycle-policy identity.

Keep evidence categories separate:

- Local evidence: Go/frontend tests, structural manifest tests, Kustomize
  render, immutable-image driver tests, and diff/Secret allowlist inspection.
- Live evidence: schema Job completion, both worker rollouts, canary Job
  completion/log outcome, Temporal v2 drain and pending activities, rebound
  reconciliation, lifecycle policy, and one end-to-end business acceptance.

Local GREEN is not proof of live drain, canary completion, deployment, or
business acceptance.

## Other ListingKit Temporal workflows

ListingKit now runs three Temporal-backed workflow paths:

- `StandardProductWorkflow`
- `PlatformAdaptWorkflow`
- existing SHEIN publish workflow

These three non-image-agent workflows keep their existing production task queue
and API-process worker wiring; the dual image-agent queues above do not change
them.

Temporal env is prepared in:

- [D:/code/task-processor/deployments/kubernetes/listingkit-workbench/base/configmap.yaml](D:/code/task-processor/deployments/kubernetes/listingkit-workbench/base/configmap.yaml)

Prepared variables:

```text
LISTINGKIT_TEMPORAL_ENABLED=1
LISTINGKIT_TEMPORAL_ADDRESS=temporal-frontend.temporal.svc.cluster.local:7233
LISTINGKIT_TEMPORAL_NAMESPACE=default
LISTINGKIT_TEMPORAL_START_WORKER=1
```

Before rollout, confirm:

1. `LISTINGKIT_TEMPORAL_ADDRESS` points at the real Temporal frontend Service.
2. `LISTINGKIT_TEMPORAL_NAMESPACE` exists in Temporal.
3. `product-listing-api` pods can reach Temporal on port `7233`.

Recommended rollout order:

1. Apply the updated ConfigMap.
2. Roll `product-listing-api`.
3. Confirm API logs contain:
   - `connected listingkit shein publish temporal client`
   - `started listingkit shein publish temporal worker`
4. Create one new ListingKit task and confirm logs contain:
   - `WorkflowType StandardProductWorkflow`
   - `WorkflowType PlatformAdaptWorkflow`

Manual verification after rollout:

1. Create a new task from `/listing-kits/new`.
2. Open the task status or workspace page.
3. Use:
   - `运行标准商品层`
   - `运行平台适配层`
4. Confirm both actions return `200` and produce Temporal workflow log lines.

Important current behavior:

- New tasks automatically run `StandardProductWorkflow`, then auto-chain into `PlatformAdaptWorkflow(platform=all)`.
- Older tasks without `standard_product_snapshot` are still manually compatible; platform adaptation can rebuild a fallback snapshot from the persisted legacy result.
- Successful manual reruns are allowed; standard/platform workflows now permit duplicate workflow IDs after a successful prior run.

## Non-production workstation build/deploy

```powershell
.\scripts\build-push-deploy-listingkit-workbench.ps1 `
  -Tag v20260428-1 `
  -Namespace listingkit-nonprod
```

Direct workstation apply is supported only when an explicitly non-production
namespace is supplied. The default namespace is `task-processor`, so a default
invocation fails before `git`, Docker, Bash, or kubectl. `-SkipApply` keeps the
build/push-only path available without any Kubernetes mutation. Production
release and rollback always use `ListingKit API Deploy`, followed by the
automatic or manual exact-attempt `ListingKit UI Deploy` gate.

Useful switches:

- `-DockerHubUser xuwei190`: image namespace.
- `-Namespace listingkit-nonprod`: required explicit non-production namespace
  for direct apply.
- `-SkipTests`: skip local test/build checks before Docker build.
- `-SkipApply`: build and push images only; it performs no Kubernetes command.
- `-PublishLatest`: additionally refreshes floating tags for development only;
  the gated release still uses the versioned tag.

The workstation script never applies the production SMS webhook Ingress. Use an
environment-specific Ingress manifest for a staging namespace.

## Change public host

Edit `deployments/kubernetes/listingkit-workbench/overlays/prod/patch-ingress.yaml` and set the desired host.

## Runtime env

The UI uses:

- `LISTINGKIT_API_BASE=http://product-listing-api:8085/api/v1/listing-kits`
- `LISTINGKIT_SERVICE_API_BASE=http://product-listing-api:8085/api/v1`
- `AUTH_SECRET`, `ZITADEL_ISSUER_URL`, `ZITADEL_CLIENT_ID`,
  `ZITADEL_CLIENT_SECRET`, the canonical `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS`,
  `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS`, and
  `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES` keys, plus
  `LISTINGKIT_DEMO_WEBHOOK_URL`, from explicit keys in
  `listingkit-workbench-secret`
- public Auth.js origins and redirect URIs from
  `listingkit-workbench-config`, not the shared Secret
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN` only from
  `listingkit-member-invitation-secret` in `product-listing-api`; the project
  id is co-located there so the deployment cannot overwrite it with an empty
  shared ConfigMap value.
- Tencent SMS and ZITADEL webhook signing values only from
  `listingkit-tencent-sms-secret` in `product-listing-api`; no other workload
  may receive that Secret.

The Go API still reads `config/config-prod.yaml` baked into the image, with
secret values expected to be supplied by runtime configuration. For ListingKit
auth, use `listingkit.zitadel.*` in YAML or the bound env vars above; the
middleware no longer reads process env directly.

