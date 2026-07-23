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
kubectl apply -n task-processor -f tmp/listingkit-workbench-secret.yaml
```

Do not commit the filled secret file.

Required ZITADEL values:

```text
ZITADEL_ISSUER_URL=https://auth.example.com
ZITADEL_CLIENT_ID=<oidc-web-client-id>
ZITADEL_CLIENT_SECRET=<oidc-web-client-secret>
# Auth.js callback URI:
# https://<workbench-host>/api/auth/callback/zitadel
ZITADEL_POST_LOGOUT_REDIRECT_URI=https://<workbench-host>
NEXT_PUBLIC_ZITADEL_CONSOLE_URL=https://auth.example.com/ui/console
```

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

Copy the command output into the workbench secret/config:

```text
LISTINGKIT_ZITADEL_PROJECT_ID=<project-id>
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

## CI/CD deploy

GitHub Actions is now the preferred release path for ListingKit Workbench.

Workflow file:

- [D:/code/task-processor/.github/workflows/listingkit-deploy.yml](D:/code/task-processor/.github/workflows/listingkit-deploy.yml)

Trigger rules:

- push tag `listingkit-v*`: run backend tests, frontend build, build/push both images, then deploy to K3S
- manual dispatch: optional custom image tag, optional `latest` publish, optional `skip_apply`

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
- overlay: `deployments/kubernetes/listingkit-workbench/overlays/prod`

## Rollback

Use the same GitHub Actions workflow for rollback. Do not bypass it unless the
workflow or GitHub itself is unavailable.

Standard rollback path:

1. Open the `ListingKit Deploy` workflow in GitHub Actions.
2. Choose `Run workflow`.
3. Set `image_tag` to a previously deployed commit tag, for example `496ca069`.
4. Keep `skip_apply=false`.
5. Run the workflow and wait for rollout to finish.

This reuses the same deployment logic as a normal release and keeps the
rollback auditable.

To find a rollback target:

- Check prior successful runs of `ListingKit Deploy`.
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
`processed=12500 duration=42s`. A failed Job may be safely rerun after fixing
the cause; upserts are idempotent. Verify a known full SKU through
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

The Go API still reads `config/config-prod.yaml` baked into the image, with
secret values expected to be supplied by runtime configuration. For ListingKit
auth, use `listingkit.zitadel.*` in YAML or the bound env vars above; the
middleware no longer reads process env directly.
