$ErrorActionPreference = 'Stop'

$compose = Join-Path $PSScriptRoot '../../deployments/docker/acquisition-compose/docker-compose.yml'
$envFile = Join-Path $PSScriptRoot '../../deployments/docker/acquisition-compose/.env.example'

# A copied configuration must fail closed without an explicit fresh project id.
$negativeFailed = $false
try { & docker compose --env-file $envFile -f $compose config 2>&1 | Out-Null; $negativeFailed = $LASTEXITCODE -ne 0 } catch { $negativeFailed = $true }
if (-not $negativeFailed) { throw 'acquisition compose accepted a missing project name' }

$project = "task-processor-acquisition-contract-$([guid]::NewGuid().ToString('N'))"
$previousProject = $env:COMPOSE_PROJECT_NAME
$env:COMPOSE_PROJECT_NAME = $project
$rendered = & docker compose --project-name $project --env-file $envFile -f $compose config 2>&1 | Out-String
if ($null -eq $previousProject) { Remove-Item Env:COMPOSE_PROJECT_NAME } else { $env:COMPOSE_PROJECT_NAME = $previousProject }
if ($LASTEXITCODE -ne 0) { throw "acquisition compose did not render: $rendered" }
foreach ($expected in @(
    "$project-product-db",
    "$project-product-db-owner-secret",
    "$project-product-runtime-secret",
    'https://localhost:18443',
    'https://localhost:18444'
)) {
    if ($rendered -notmatch [regex]::Escape($expected)) { throw "missing acquisition-compose contract: $expected" }
}
if ($rendered -match 'referral_runtime|provider-machine\.pat|referral-db') { throw 'acquisition compose must not configure referrals' }
$terraform = Get-Content -LiteralPath (Join-Path (Split-Path -Parent $compose) 'terraform/main.tf') -Raw
if ($terraform -notmatch 'resource\s+"zitadel_user_grant"\s+"operator"\s*\{[\s\S]*?role_keys\s*=\s*\["listingkit_operator"\]') {
    throw 'local bootstrap operator must receive exactly listingkit_operator'
}
if ($terraform -match 'role_keys\s*=\s*\[[^\]]*"(?:listingkit_admin|platform_admin)"') {
    throw 'local bootstrap operator must not receive an administrator role'
}
$init = Get-Content -LiteralPath (Join-Path (Split-Path -Parent $compose) 'init.sh') -Raw
foreach ($expected in @('source_acquisition_runtime', 'productAcquisitionDatabase', '"enabled": false', '-confirm-empty-database product_acquisition')) {
    if ($init -notmatch [regex]::Escape($expected)) { throw "missing explicit acquisition initializer contract: $expected" }
}

$env:COMPOSE_PROJECT_NAME = $project
$model = (& docker compose --project-name $project --env-file $envFile -f $compose config --format json | ConvertFrom-Json)
Remove-Item Env:COMPOSE_PROJECT_NAME
$forbiddenServingVolumes = @('identity-db-secret', 'source-db-owner-secret', 'commercial-db-owner-secret', 'product-db-owner-secret', 'zitadel-api-secrets', 'zitadel-iac-pat', 'tofu-state', 'tofu-inputs')
foreach ($service in @('listingkit-ui', 'current-application')) {
    $sources = @($model.services.$service.volumes | ForEach-Object source)
    if (@($sources | Where-Object { $_ -in $forbiddenServingVolumes }).Count -ne 0) { throw "$service mounts an owner-only volume" }
    if (@($model.services.$service.volumes | Where-Object { -not $_.read_only }).Count -ne 0) { throw "$service has a writable private mount" }
}
