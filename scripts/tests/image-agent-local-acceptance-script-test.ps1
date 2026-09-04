$ErrorActionPreference = "Stop"
$scriptPath = Join-Path $PSScriptRoot "..\image-agent-local-acceptance.ps1"

$previousErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = "Continue"
& powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath seed -SourceUrl "http://localhost/nope.png" 2>$null
$seedExitCode = $LASTEXITCODE
$ErrorActionPreference = $previousErrorActionPreference
if ($seedExitCode -eq 0) { throw "seed accepted an unsafe URL" }

& powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath stop
if ($LASTEXITCODE -ne 0) { throw "non-destructive stop must not require Reset" }

& powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath stop -Reset -WhatIf
if ($LASTEXITCODE -ne 0) { throw "reset preview must identify only the acceptance project" }

$scriptText = Get-Content -LiteralPath $scriptPath -Raw
$zitadelExampleEnvPath = Join-Path $PSScriptRoot "..\..\deployments\docker\zitadel\.env.example"
$zitadelExampleEnvText = Get-Content -LiteralPath $zitadelExampleEnvPath -Raw
$pinnedZitadelVersion = "ZITADEL_VERSION=v4.17.1"
if ($scriptText -notmatch [regex]::Escape($pinnedZitadelVersion)) {
    throw "acceptance orchestrator must pin $pinnedZitadelVersion"
}
if ($zitadelExampleEnvText -notmatch [regex]::Escape($pinnedZitadelVersion)) {
    throw "ZITADEL example environment must pin $pinnedZitadelVersion"
}
if ($scriptText -notmatch [regex]::Escape("SET client_min_messages TO warning")) {
    throw "acceptance marker setup must suppress idempotent PostgreSQL notices"
}
$stopFunction = [regex]::Match($scriptText, '(?s)function Stop-Acceptance \{(?<body>.*?)\r?\n\}').Groups['body'].Value
$ownershipStopIndex = $stopFunction.IndexOf('Stop-LocalProcess')
$shouldProcessIndex = $stopFunction.IndexOf('$PSCmdlet.ShouldProcess')
if ($shouldProcessIndex -lt 0 -or $ownershipStopIndex -lt 0 -or $shouldProcessIndex -gt $ownershipStopIndex) {
    throw 'stop WhatIf must pass ShouldProcess before stopping any owned process'
}
foreach ($required in @(
    "task-processor-image-agent-acceptance",
    "image_agent_acceptance",
    "--wait",
    "-runtime-file",
    "public HTTPS",
    "acceptance-postgres-data",
    "acceptance-s3-data",
    "acceptance-redis",
    "acceptance-minio-init",
    "TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE",
    "cmd/product-listing-api-schema-migrate",
    '"ZITADEL_SCOPES"',
    '${databaseName}?sslmode=disable',
    '$env:KUBECONFIG = ""',
    'TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_ID',
    'TASK_PROCESSOR_LISTINGKIT_ZITADEL_API_CLIENT_SECRET',
    'Import-ProvisionedApiRuntime'
)) {
    if ($scriptText -notmatch [regex]::Escape($required)) { throw "orchestrator is missing required boundary: $required" }
}
if ($scriptText -match "Write-Host.*Token|Write-Host.*Secret|Write-Output.*Token|Write-Output.*Secret") {
    throw "orchestrator must not print credentials"
}

Write-Output "PASS image-agent-local-acceptance script contract"
