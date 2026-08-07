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

This file must be merged after the recovered release values, for example:

```powershell
helm upgrade --install temporal temporal `
  --repo https://go.temporal.io/helm-charts `
  --namespace temporal `
  --version 1.2.0 `
  -f <recovered-original-values.yaml> `
  -f deployments/kubernetes/temporal/temporal-persistence-pool-values.yaml
```

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
