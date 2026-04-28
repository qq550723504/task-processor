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

The Go API still reads `config/config-prod.yaml` baked into the image, with secret values expected to be supplied by environment variables or the mounted runtime configuration used in your cluster.
