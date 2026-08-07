# Temporal configuration recovery

The production Temporal resources are now managed by Helm release `temporal`
(chart `temporal-1.2.0`, revision 1). The original pre-adoption values file was
not recovered; the checked-in recovery baseline below is the reviewed source
for the adopted release and should be compared with `helm get values` before
future upgrades.

`temporal-persistence-pool-values.yaml` is a pool-only override for the
verified PostgreSQL connection hardening:

- both `default` and `visibility` stores keep `maxConns: 20`;
- both stores use `maxIdleConns: 5`;
- it intentionally does not set schema ownership, database creation, or
  credentials; those remain in the recovered release values and existing
  Kubernetes Secrets.

`temporal-recovered-server-values.yaml` is the larger recovery baseline rebuilt
from the live server ConfigMap, Deployment resources, and the official
`temporal-1.2.0` chart defaults. It contains the adopted persistence and
schema settings, including `createDatabase: false` and `manageSchema: true`.
The baseline was rendered and compared with the live release before adoption.

For a future upgrade, inspect the stored release values first, then render the
recovery baseline and apply the optional pool-only override last:

```powershell
helm status temporal --namespace temporal
helm get values temporal --namespace temporal --all
helm upgrade --install temporal temporal `
  --repo https://go.temporal.io/helm-charts `
  --namespace temporal `
  --version 1.2.0 `
  -f deployments/kubernetes/temporal/temporal-recovered-server-values.yaml `
  -f deployments/kubernetes/temporal/temporal-persistence-pool-values.yaml
```

Do not remove the schema settings from the recovery baseline. They keep the
existing PostgreSQL databases authoritative while allowing the normal Helm
schema hook to run before future server upgrades.

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

Do not run the upgrade until the stored values, chart version, schema-job
ordering, and database credentials have been reviewed.

## Recovery baseline verification (2026-08-07)

- The official `temporal-1.2.0` chart was rendered locally with the recovery
  values file.
- The rendered `temporal-config` `config_template.yaml` matched the live
  ConfigMap exactly.
- Rendered Deployment images, replica counts, and container resources matched
  all six live Temporal Deployments.
- `kubectl apply --dry-run=server` accepted the complete rendered resource set;
  no resource was applied.
- The rendered chart includes `temporal-schema-1-2-0-1`, while the live
  namespace currently has no Temporal Job. Schema-job ownership and ordering
  remain an explicit upgrade gate.
- The Temporal databases currently report `temporal=1.19` and
  `temporal_visibility=1.14`. The rendered chart's schema hook uses a
  `ttlSecondsAfterFinished` of 86400, so an absent historical Job is
  consistent with a completed-and-cleaned hook, but the next upgrade must
  still preserve schema-before-server ordering.

## Helm adoption result (2026-08-07)

- The first adoption attempt exposed that `temporal_app` cannot create
  databases. Both SQL stores are therefore explicitly configured with
  `createDatabase: false` and `manageSchema: true`; the existing schema hook
  completed successfully against the already-provisioned databases.
- Helm 4 server-side adoption was not compatible with the existing headless
  Service and Deployment managed fields. The final adoption used
  `--take-ownership --no-hooks --server-side=false --rollback-on-failure` and
  created release `temporal` revision 1 without changing the database schema a
  second time.
- After adoption, all six Deployments were Ready, the API health endpoint
  returned `{"status":"ok"}`, no Temporal startup errors appeared in the
  final 45-second check, and PostgreSQL reported 44 active connections.
- `--no-hooks` was used only for the initial adoption after the schema hook had
  completed. The stored values retain `schema.useHelmHooks: true`, so future
  upgrades must run the pre-upgrade schema hook normally.
