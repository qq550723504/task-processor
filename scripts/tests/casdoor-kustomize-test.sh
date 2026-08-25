#!/usr/bin/env bash
set -euo pipefail

if command -v kustomize >/dev/null 2>&1; then
  KUSTOMIZE=(kustomize build)
else
  KUSTOMIZE=(kubectl kustomize)
fi

rendered="$("${KUSTOMIZE[@]}" deployments/kubernetes/casdoor/overlays/staging)"
grep -F 'namespace: casdoor' <<<"$rendered"
grep -F 'image: casbin/casdoor:3.143.0@sha256:1284af680ddf10aa80569f1f4a46210dd9875ce70845e67047053363d0c0ba58' <<<"$rendered"
grep -F 'host: id.staging.shuomiai.com' <<<"$rendered"
grep -F 'driverName = postgres' <<<"$rendered"
grep -F 'shared-postgresql.platform-data.svc.cluster.local' <<<"$rendered"
! grep -Eqi 'image: .*:latest|listingkit-tencent-sms-secret|TASK_PROCESSOR_LISTINGKIT_ZITADEL_SMS_SIGNING_KEY|kind: StatefulSet|kind: PersistentVolumeClaim' <<<"$rendered"

# The strict OTP limiter must be scoped to the casdoor-otp ingress; the
# catch-all "/" router keeps the looser web limiter.
grep -q 'name: casdoor-otp' <<<"$rendered"
grep -A2 'router.middlewares: casdoor-auth-rate-limit@kubernetescrd' <<<"$rendered" | grep -q 'name: casdoor-otp'
if grep -B6 'router.middlewares: casdoor-auth-rate-limit@kubernetescrd' <<<"$rendered" | grep -q 'path: /$'; then
  echo "strict OTP limiter attached to the catch-all route" >&2
  exit 1
fi
grep -F "printf '%s\\n' '{}' > /conf/init_data.json" <<<"$rendered"

"${KUSTOMIZE[@]}" deployments/kubernetes/casdoor/overlays/prod >/dev/null
