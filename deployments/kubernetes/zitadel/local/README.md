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
username: zitadel-admin
password: Password1!
```

## Create the ListingKit OIDC app

In ZITADEL, create an OIDC Web application for ListingKit:

```text
Redirect URI: https://pod.shuomiai.com/api/zitadel-auth/callback
Post logout redirect URI: https://pod.shuomiai.com
```

Copy the generated client id and client secret into a real Kubernetes Secret
based on `listingkit-workbench-zitadel-secret.example.yaml`.

## Optional local ListingKit ingress host

If you want the workbench reachable at `https://listingkit.localhost`, apply an
overlay or one-off patch based on:

```text
deployments/kubernetes/zitadel/local/listingkit-workbench-ingress-patch.example.yaml
```

Keep this setup local-only. The bundled PostgreSQL persistence is disabled, the
master key is a development placeholder, and Traefik will use its default TLS
certificate unless you add a real `tls.secretName`.
