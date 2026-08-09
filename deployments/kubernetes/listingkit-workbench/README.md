# ListingKit Workbench K3S Deployment

This deploys the customer SHEIN Studio workbench as two services:

- `product-listing-api`: Go API on port `8085`.
- `listingkit-ui`: Next.js UI on port `3000`.

The browser talks to the Next.js UI. The UI proxies `/api/listing-kits/*` and `/api/sds/*` to `product-listing-api` through the cluster Service, so the backend does not need a public Ingress.

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

When migrating an existing deployment, first set the non-secret project id in
`listingkit-workbench-config` and create `listingkit-member-invitation-secret`.
Then remove both invitation keys from the already deployed shared Secret and
restart every long-lived consumer so no UI, worker, or imgproxy Pod retains the
write token in its process environment. The API Secret reference is required:
do not restart the API until the dedicated Secret has been created, because a
missing token must fail Pod startup instead of falling back to a legacy shared
Secret value.

```powershell
$legacy = kubectl -n task-processor get secret listingkit-workbench-secret -o json | ConvertFrom-Json
foreach ($key in @(
  "TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
  "TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID"
)) {
  if ($legacy.data.PSObject.Properties.Name -contains $key) {
    $memberInvitationPatch = '[{"op":"remove","path":"/data/' + $key + '"}]'
    kubectl -n task-processor patch secret listingkit-workbench-secret --type=json `
      -p $memberInvitationPatch
  }
}
kubectl -n task-processor rollout restart deployment/product-listing-api,listingkit-ui,imgproxy,shein-login-worker
kubectl -n task-processor rollout status deployment/product-listing-api --timeout=5m
kubectl -n task-processor rollout status deployment/listingkit-ui --timeout=5m
kubectl -n task-processor rollout status deployment/imgproxy --timeout=5m
kubectl -n task-processor rollout status deployment/shein-login-worker --timeout=5m
```

The commands inspect only key names and do not decode or print Secret values.

Required ZITADEL values:

```text
ZITADEL_ISSUER_URL=https://auth.example.com
ZITADEL_CLIENT_ID=<oidc-web-client-id>
ZITADEL_CLIENT_SECRET=<oidc-web-client-secret>
TASK_PROCESSOR_LISTINGKIT_ZITADEL_TENANT_DIRECTORY_TOKEN=<read-only-tenant-directory-token>
# API-only listingkit-member-invitation-secret:
TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN=<dedicated-member-invitation-token>
# listingkit-workbench-config ConfigMap (non-secret identifier):
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
keys through env binding such as `ZITADEL_ISSUER_URL`,
`ZITADEL_CLIENT_ID`, and `TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTH_REQUIRED`.
For the full migration checklist, including owner-scope and allowlist rollout
checks, see
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
LISTINGKIT_ZITADEL_ALLOWED_ROLES=listingkit_admin,listingkit_operator,listingkit_viewer,platform_admin
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

Store the dedicated token only in the API-only
`listingkit-member-invitation-secret`; store the existing project id as the
non-secret `TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID` value in
`listingkit-workbench-config`:

```text
TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN=<dedicated-service-account-token>
TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID=<existing-listingkit-project-id>
```

Inject the token through the approved secret manager; never commit it, print it,
or paste it into the OIDC or tenant-directory fields. Do not add the dedicated
Secret to UI, worker, imgproxy, or migration Job `envFrom` lists. Apply the
dedicated Secret and ConfigMap, then restart only the API deployment:

```powershell
kubectl apply -n task-processor -f tmp/listingkit-member-invitation-secret.yaml
kubectl apply -n task-processor -f deployments/kubernetes/listingkit-workbench/base/configmap.yaml

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
$configMap = kubectl -n task-processor get configmap listingkit-workbench-config -o json |
  ConvertFrom-Json
if ([string]::IsNullOrWhiteSpace($configMap.data.TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID)) {
  throw "Missing ListingKit invitation project id in ConfigMap"
}

kubectl -n task-processor rollout restart deployment/product-listing-api
kubectl -n task-processor rollout status deployment/product-listing-api --timeout=5m
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

## CI/CD deploy

GitHub Actions is now the preferred release path for ListingKit Workbench.

For the Release Candidate gate, immutable image policy, post-deploy smoke
checks, and recovery decision, follow the
[ListingKit Release Candidate Runbook](../../../docs/operations/listingkit-release-candidate-runbook.md).

### First controlled deployment

Use a release image built by an approved CI run and its immutable tag. Before
applying the API or UI deployment, create the namespace, ConfigMap, and real
Secret, then run the schema migration Jobs once. The API does not auto-migrate
at startup.

```powershell
$tag = "<immutable-release-tag>"

kubectl apply -f deployments/kubernetes/listingkit-workbench/base/namespace.yaml
kubectl apply -f deployments/kubernetes/listingkit-workbench/base/configmap.yaml
# Apply the real Secret created outside Git before continuing.

$migrationJobs = @(
  "product-listing-api-schema-migrate-job.yaml",
  "listingkit-schema-migrate-job.yaml"
)
foreach ($jobFile in $migrationJobs) {
  $migrationFile = Join-Path $env:TEMP $jobFile
  Copy-Item (Join-Path "deployments/kubernetes/listingkit-workbench/jobs" $jobFile) $migrationFile
  (Get-Content -Raw $migrationFile).Replace("REPLACE_WITH_DEPLOYED_TAG", $tag) |
    Set-Content -NoNewline $migrationFile
  $jobName = kubectl create -n task-processor -f $migrationFile -o jsonpath='{.metadata.name}'
  kubectl -n task-processor wait --for=condition=complete "job/$jobName" --timeout=15m
  kubectl -n task-processor logs "job/$jobName"
}
```

The two Jobs share only the production ConfigMap and Secret references required
by the API. They run `/app/product-listing-api-schema-migrate` and
`/app/listingkit-schema-migrate -scope all` respectively, using the same
immutable API image that will be released. A failed Job is a No-Go: investigate
and use an approved roll-forward or restore procedure instead of deleting or
editing the production schema manually.

For a new cluster, render the existing production overlay from a temporary
copy, pin both image names to the same immutable tag, and apply that rendered
manifest only after the Job succeeds. This prevents the overlay's development
`latest` defaults from being used during bootstrap.

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
  kustomize build . | kubectl apply -f -
} finally {
  Pop-Location
  if ((Resolve-Path (Split-Path $staging -Parent)).Path -ne $stagingRoot) {
    throw "Refusing to remove a staging directory outside the temporary root"
  }
  Remove-Item -Recurse -Force $staging
}
```

Record the source SHA, image tags, migration Job name, and rollout output in
the Release Candidate Runbook. Subsequent releases must use the independent
GitHub Actions workflows below rather than reapplying an unpinned overlay.

### Release workflows

- `ListingKit API Deploy` ([workflow](../../../.github/workflows/listingkit-deploy.yml))
- `ListingKit UI Deploy` ([workflow](../../../.github/workflows/listingkit-ui-deploy.yml))

Trigger rules:

- API tag `listingkit-api-v*` deploys only `product-listing-api`.
- UI tag `listingkit-ui-v*` deploys only `listingkit-ui`.
- For an RC, use `workflow_dispatch` for both workflows and set the same
  immutable `source_ref` and `image_tag` explicitly.
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

Use the same GitHub Actions workflows for rollback. Do not bypass them unless
GitHub Actions itself is unavailable.

Standard rollback path:

1. Identify the prior API and UI image tags from a successful release record.
2. Run `ListingKit API Deploy` with its prior immutable tag, then wait for the
   API rollout and readiness probe.
3. Run `ListingKit UI Deploy` with its prior immutable tag, then wait for the
   UI rollout.
4. Record the rollback decision, deployed tags, probe results, and any data
   recovery action in the validation run.

This reuses the same deployment logic as a normal release and keeps the
rollback auditable.

To find a rollback target:

- Check prior successful runs of `ListingKit API Deploy` and `ListingKit UI Deploy`.
- Or inspect the currently deployed / previously deployed image tags in Docker
  Hub or Kubernetes rollout history.

Emergency fallback from a workstation:

```powershell
kubectl -n task-processor set image deployment/product-listing-api product-listing-api=docker.io/xuwei190/task-processor-product-listing-api:496ca069
kubectl -n task-processor set image deployment/listingkit-ui listingkit-ui=docker.io/xuwei190/task-processor-listingkit-ui:496ca069
kubectl -n task-processor rollout status deployment/product-listing-api --timeout=5m
kubectl -n task-processor rollout status deployment/listingkit-ui --timeout=5m
```

Use the emergency path only when GitHub Actions cannot be used. If you do use
it, follow up with a normal workflow-driven deploy so the release history stays
consistent.

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
then inspect API/proxy logs for errors. Keep owner-scope rollout disabled until
the backfill and tenant/user sampling are complete.

## Temporal rollout

ListingKit now runs three Temporal-backed workflow paths:

- `StandardProductWorkflow`
- `PlatformAdaptWorkflow`
- existing SHEIN publish workflow

Current production wiring keeps them on the same task queue and starts the
worker inside `product-listing-api`. This is the smallest rollout shape and
matches the current runtime implementation.

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

## Manual deploy fallback

```powershell
.\scripts\build-push-deploy-listingkit-workbench.ps1 -Tag v20260428-1 -PublishLatest
```

Useful switches:

- `-DockerHubUser xuwei190`: image namespace.
- `-Namespace task-processor`: Kubernetes namespace.
- `-OverlayPath deployments/kubernetes/listingkit-workbench/overlays/prod`: Kustomize overlay.
- `-SkipTests`: skip local test/build checks before Docker build.
- `-SkipApply`: update images without applying manifests.

## Change public host

Edit `deployments/kubernetes/listingkit-workbench/overlays/prod/patch-ingress.yaml` and set the desired host.

## Runtime env

The UI uses:

- `LISTINGKIT_API_BASE=http://product-listing-api:8085/api/v1/listing-kits`
- `LISTINGKIT_SERVICE_API_BASE=http://product-listing-api:8085/api/v1`
- `ZITADEL_ISSUER_URL`, `ZITADEL_CLIENT_ID`, `ZITADEL_CLIENT_SECRET`, and
  redirect URIs from `listingkit-workbench-secret`
- `TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN` only from
  `listingkit-member-invitation-secret` in `product-listing-api`; the project
  id is a non-secret ConfigMap value.

The Go API still reads `config/config-prod.yaml` baked into the image, with
secret values expected to be supplied by runtime configuration. For ListingKit
auth, use `listingkit.zitadel.*` in YAML or the bound env vars above; the
middleware no longer reads process env directly.
