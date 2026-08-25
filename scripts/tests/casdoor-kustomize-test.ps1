$ErrorActionPreference = 'Stop'

$rendered = kubectl kustomize deployments/kubernetes/casdoor/overlays/staging 2>&1
if ($LASTEXITCODE -ne 0) {
    throw "kubectl kustomize failed: $($rendered -join "`n")"
}

$required = @(
    'namespace: casdoor',
    'image: casbin/casdoor:3.143.0@sha256:1284af680ddf10aa80569f1f4a46210dd9875ce70845e67047053363d0c0ba58',
    'host: id.staging.shuomiai.com',
    'driverName = postgres',
    'shared-postgresql.platform-data.svc.cluster.local'
)
foreach ($needle in $required) {
    if (-not ($rendered -join "`n").Contains($needle)) {
        throw "rendered output is missing: $needle"
    }
}

$renderedText = $rendered -join "`n"
$forbidden = @(
    'image: .*:latest',
    'listingkit-tencent-sms-secret',
    'TASK_PROCESSOR_LISTINGKIT_ZITADEL_SMS_SIGNING_KEY',
    'kind: StatefulSet',
    'kind: PersistentVolumeClaim',
    'id.shuomiai.com'
)
foreach ($pattern in $forbidden) {
    if ($renderedText -match $pattern) {
        throw "rendered output contains forbidden pattern: $pattern"
    }
}

if ($renderedText -match 'host: casdoor\.invalid') {
    throw 'staging overlay still contains the non-routable base host'
}
if (([regex]::Matches($renderedText, '(?m)^kind: Ingress$')).Count -ne 2) {
    throw 'staging overlay must render the web and OTP ingresses'
}
if ($renderedText -notmatch 'traefik\.ingress\.kubernetes\.io/router\.middlewares: casdoor-auth-rate-limit@kubernetescrd') {
    throw 'staging OTP ingress is missing the auth rate limit middleware'
}
if ($renderedText -notmatch "printf '%s\\n' '\{\}' > /conf/init_data\.json") {
    throw 'Casdoor init_data.json must be a valid empty JSON object, not an empty file'
}

Write-Output 'casdoor-kustomize-test: pass'
