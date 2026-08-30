$ErrorActionPreference = 'Stop'

$base = docker compose --env-file deployments/docker/zitadel/.env.example -f deployments/docker/zitadel/docker-compose.yml config | Out-String
if ($base -match 'docker_yudao-network') { throw 'base Compose must not require docker_yudao-network' }
if ($base -notmatch 'postgres:') { throw 'base Compose must include local postgres' }

$overlay = docker compose --env-file deployments/docker/zitadel/.env.example -f deployments/docker/zitadel/docker-compose.yml -f deployments/docker/zitadel/docker-compose.yudao-db.yml config | Out-String
if ($overlay -notmatch 'docker_yudao-network') { throw 'Yudao overlay must opt in to docker_yudao-network' }
