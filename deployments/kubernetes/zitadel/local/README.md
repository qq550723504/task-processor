# Local ZITADEL

This directory contains a local-only ZITADEL Helm configuration for testing the
ListingKit OIDC integration.

## Install

```bash
helm repo add zitadel https://charts.zitadel.com
helm repo update
kubectl create namespace zitadel --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install zitadel zitadel/zitadel \
  --namespace zitadel \
  -f deployments/kubernetes/zitadel/local/values.yaml
```

Wait for the setup job and pods:

```bash
kubectl -n zitadel get pods,jobs
```

Open ZITADEL at:

```text
https://auth.shuomiai.com
```

Default local admin:

```text
username: supperadmin
password: Zone5571886$$$
```

## Create the ListingKit OIDC app

In ZITADEL, create an OIDC Web application for ListingKit:

```text
Redirect URI: https://pod.shuomiai.com/api/zitadel-auth/callback
Post logout redirect URI: https://pod.shuomiai.com
```

Copy the generated client id and client secret into a real Kubernetes Secret
based on `listingkit-workbench-zitadel-secret.example.yaml`.

The UI still reads `ZITADEL_*` directly, while the Go API binds the same
issuer/client credentials into `listingkit.zitadel.*`. Additional rollout
checks and allowlist guidance are documented in
[listingkit-config-migration-checklist.md](/D:/code/task-processor/docs/development/listingkit-config-migration-checklist.md).

## Provision ListingKit project roles

Run role provisioning as an operator step after ZITADEL is reachable and before
you rely on role-based menus or authorization. Do not run this from the normal
ListingKit UI/API startup path; the management token should stay outside
application runtime secrets.

```powershell
$env:ZITADEL_ISSUER_URL = "https://auth.shuomiai.com"
$env:ZITADEL_MANAGEMENT_TOKEN = "<management-api-token>"
$env:ZITADEL_ORG_ID = "<org-id>"
go run ./cmd/listingkit-zitadel-provision -project-name ListingKit -create-project
```

The command is idempotent. It finds or optionally creates the `ListingKit`
project, ensures these project roles exist, and prints the runtime environment
values to copy into the workbench secret:

```text
listingkit_viewer
listingkit_operator
listingkit_admin
platform_admin
```

Use the printed `ZITADEL_SCOPES` value for the OIDC Web application runtime
configuration so access tokens include the project role claim parsed by
ListingKit.

## Optional local ListingKit ingress host

If you want the workbench reachable at `https://listingkit.localhost`, apply an
overlay or one-off patch based on:

```text
deployments/kubernetes/zitadel/local/listingkit-workbench-ingress-patch.example.yaml
```

Keep this setup local-only. The bundled PostgreSQL persistence is disabled, the
master key is a development placeholder, and Traefik will use its default TLS
certificate unless you add a real `tls.secretName`.

## Production note

虽然这个目录名还是 `local`，但当前线上 `https://auth.shuomiai.com` 的运行结
构已经和最初的“内置 PostgreSQL 16”不同。

`2026-05-16` 起，线上 ZITADEL 使用的是 `yudao-cloud` 命名空间下已有的
PostgreSQL 18 实例，并单独占用一个数据库：

```text
host: postgresql-v18.yudao-cloud.svc.cluster.local:5432
database: zitadel_auth
username: zitadel
```

约束：

- 不要把 ZITADEL 直接指到业务库 `ruoyi-vue-pro`
- 可以复用同一 PostgreSQL 实例，但必须保持独立 database
- `deployments/kubernetes/zitadel/postgres16/*` 现在只保留为历史示例，不再
  代表线上现状

已完成的线上调整：

- 旧 `zitadel-postgres16` 数据已迁移到 `postgresql-v18/zitadel_auth`
- `auth.shuomiai.com` 已验证恢复正常
- 旧 `zitadel-postgres16` 的 `StatefulSet/Service/Secret/PVC` 已删除

回滚结论：

- 现在没有基于旧 PVC 的快速回滚
- 如需回滚，必须依赖额外备份或重新恢复一份旧库数据
