# Temporal configuration recovery

The production Temporal resources were created from the Temporal Helm chart
(chart label `temporal-1.2.0`), but the cluster currently has no Helm release
secret or GitOps owner for that release. The complete original values file is
therefore required before a Helm upgrade can be performed safely.

`temporal-persistence-pool-values.yaml` is the tracked source for the verified
PostgreSQL pool hardening only:

- both `default` and `visibility` stores keep `maxConns: 20`;
- both stores use `maxIdleConns: 5`;
- credentials remain in the existing Kubernetes Secrets.

`temporal-recovered-server-values.yaml` is a larger recovery baseline rebuilt
from the live server ConfigMap, Deployment resources, and the official
`temporal-1.2.0` chart defaults. It is not treated as authoritative until a
rendered diff confirms the service, resource, image, persistence, and schema
settings against the live cluster.

This file must be merged after the recovered release values, for example:

```powershell
helm upgrade --install temporal temporal `
  --repo https://go.temporal.io/helm-charts `
  --namespace temporal `
  --version 1.2.0 `
  -f <recovered-original-values.yaml> `
  -f deployments/kubernetes/temporal/temporal-persistence-pool-values.yaml
```

If the original values cannot be recovered, use the recovery baseline as the
starting input instead, but review the chart's schema-management settings
before running any upgrade.

Before applying, render and compare the result with the live resources:

```powershell
helm template temporal temporal `
  --repo https://go.temporal.io/helm-charts `
  --namespace temporal `
  --version 1.2.0 `
  -f <recovered-original-values.yaml> `
  -f deployments/kubernetes/temporal/temporal-persistence-pool-values.yaml `
  | kubectl diff -f -
```

Do not run the upgrade until the original values, chart version, schema-job
ordering, and database credentials have been recovered and reviewed.

## Recovery baseline verification (2026-08-07)

- The official `temporal-1.2.0` chart was rendered locally with the recovery
  values file.
- The rendered `temporal-config` `config_template.yaml` matched the live
  ConfigMap exactly.
- Rendered Deployment images, replica counts, and container resources matched
  all six live Temporal Deployments.
- `kubectl apply --dry-run=server` accepted the complete rendered resource set;
  no resource was applied.
