#!/usr/bin/env bash
set -euo pipefail

if command -v kustomize >/dev/null 2>&1; then
  KUSTOMIZE=(kustomize build)
else
  KUSTOMIZE=(kubectl kustomize)
fi

rendered="$("${KUSTOMIZE[@]}" deployments/kubernetes/casdoor/overlays/staging)"
grep -F 'namespace: casdoor' <<<"$rendered"
grep -F 'image: casbin/casdoor:v3.143.0' <<<"$rendered"
grep -F 'host: id.staging.shuomiai.com' <<<"$rendered"
grep -F 'driverName = postgres' <<<"$rendered"
! grep -Eqi 'image: .*:latest|listingkit-tencent-sms-secret|TASK_PROCESSOR_LISTINGKIT_ZITADEL_SMS_SIGNING_KEY' <<<"$rendered"

"${KUSTOMIZE[@]}" deployments/kubernetes/casdoor/overlays/prod >/dev/null
