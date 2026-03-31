# Amazon Crawler API on Kubernetes

This directory now includes Kubernetes manifests for the Amazon crawler API:

- `amazon-crawler-api.example.yaml`
- `amazon-crawler-api/base/`
- `amazon-crawler-api/overlays/staging/`
- `amazon-crawler-api/overlays/prod/`
- `REDIS.md`

## Recommended layout

- `base/`: shared resources
- `overlays/prod/`: production overrides

The overlay currently overrides:

- replica count
- resource requests and limits
- ingress host
- image pull policy
- runtime `Secret`

## Before you apply it

Replace the placeholder values in:

- `amazon-crawler-api/overlays/prod/secret.yaml`

The key fields are:

- `TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_ID`
- `TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_SECRET`
- `TASK_PROCESSOR_AMAZON_SPAPI_REFRESH_TOKEN`
- `TASK_PROCESSOR_OPENAI_API_KEY`
- `TASK_PROCESSOR_MANAGEMENT_*`

Also update:

- the image tag from `xuwei190/task-processor-amazon-crawler-api:latest`
- the ingress host `amazon-crawler-api.example.com`
- the Redis service DNS name if your Redis lives in another namespace or is managed outside the cluster

## Redis recommendation

Do not run one Redis per Pod for this service. Use one shared Redis instance or Redis cluster, then point:

- `TASK_PROCESSOR_REDIS_HOST`
- `TASK_PROCESSOR_REDIS_PORT`
- `TASK_PROCESSOR_REDIS_PASSWORD`
- `TASK_PROCESSOR_REDIS_DB`

The example defaults to `redis-master.default.svc.cluster.local:6379`. Change it to your real in-cluster DNS name or external address.

## Browser runtime note

The current example assumes the runtime image already contains a Linux browser binary and sets:

- `TASK_PROCESSOR_BROWSER_HEADLESS=true`
- `TASK_PROCESSOR_BROWSER_PATH=/usr/bin/chromium`

If your base image uses a different location, update both:

- `browser.browserPath` in the `ConfigMap`
- `TASK_PROCESSOR_BROWSER_PATH` in the `Deployment`

## Render

```bash
kubectl kustomize deployments/kubernetes/amazon-crawler-api/overlays/staging
kubectl kustomize deployments/kubernetes/amazon-crawler-api/overlays/prod
```

## Apply

```bash
kubectl apply -k deployments/kubernetes/amazon-crawler-api/overlays/staging
kubectl kustomize deployments/kubernetes/amazon-crawler-api/overlays/prod
kubectl apply -k deployments/kubernetes/amazon-crawler-api/overlays/prod
```

## Suggested next step

You can keep extending this with:

- `overlays/test/`
- external secret management such as External Secrets or Sealed Secrets
- CI/CD deployment automation

If you later need more reusable values or cross-service packaging, this structure can be converted into a Helm chart cleanly.
