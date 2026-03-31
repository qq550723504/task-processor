# Redis for Kubernetes Deployment

`amazon-crawler-api` should use one shared Redis instance. Do not run a separate Redis inside each application Pod.

## Option 1: In-cluster Redis via Helm

If you use Helm, a practical starting point is Bitnami Redis:

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm upgrade --install redis bitnami/redis \
  --namespace infra \
  --create-namespace \
  --set architecture=standalone \
  --set auth.enabled=true \
  --set master.persistence.enabled=true \
  --set master.persistence.size=20Gi
```

Typical service DNS after install:

```text
redis-master.infra.svc.cluster.local
```

Then update these values in your overlay secret:

- `TASK_PROCESSOR_REDIS_HOST`
- `TASK_PROCESSOR_REDIS_PORT`
- `TASK_PROCESSOR_REDIS_PASSWORD`
- `TASK_PROCESSOR_REDIS_DB`

## Option 2: Managed Redis outside the cluster

If you already have Redis Cloud, AWS ElastiCache, Alibaba Cloud Redis, or another managed service, point the same environment variables to that external address.

This is often the better production choice if you want:

- simpler backup and failover
- less operational work inside Kubernetes
- easier scaling and monitoring

## Verification

After deployment, verify the app can reach Redis:

```bash
kubectl get pods -n task-processor
kubectl logs -n task-processor deploy/amazon-crawler-api
kubectl describe pod -n task-processor -l app=amazon-crawler-api
```

If Redis auth is enabled, make sure the password in the Kubernetes `Secret` exactly matches the server-side password.
