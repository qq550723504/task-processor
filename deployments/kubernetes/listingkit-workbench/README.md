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

## Deploy

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
