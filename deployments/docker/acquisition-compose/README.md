# Local acquisition trial Compose

This is a fresh, isolated local **acquisition-only** composition for the
current application. It is not an upgrade path for another trial, a shared
environment, or production. It enables the existing authenticated public-1688
acquisition route and deliberately leaves membership and referrals disabled.

Only loopback HTTPS is published:

- `https://localhost:18443` — ZITADEL issuer and Login V2.
- `https://localhost:18444` — ListingKit Web/BFF.

The Go application is reachable only at `127.0.0.1:8085` inside the shared
network namespace. `LISTINGKIT_SERVICE_API_BASE` therefore remains the same
Go instance's `/api/v1` root.

The UI alone sets `LISTINGKIT_PRODUCT_ACQUISITION_ENABLED=true`; the default
web build remains unavailable unless its serving environment opts in, while the
current application independently verifies its product-acquisition database.

## Create a new owned instance

Copy the blank environment file, generate a unique project name, and prove it
does not already name containers or volumes before starting it. A missing name
fails closed; do not substitute a fixed name.

```powershell
Copy-Item .env.example .env
$project = "task-processor-acquisition-$([guid]::NewGuid().ToString('N'))"
$containers = @(docker ps -aq --filter "label=com.docker.compose.project=$project" | Where-Object { $_ })
$volumes = @(docker volume ls --format '{{.Name}}' | Where-Object { $_ -like "$project-*" })
if ($containers.Count -ne 0 -or $volumes.Count -ne 0) { throw "Compose project name is already occupied: $project" }
"COMPOSE_PROJECT_NAME=$project" | Set-Content -LiteralPath .env -NoNewline -Encoding ascii
docker compose --project-name $project --env-file .env up --build --wait
```

Bootstrap creates a project-owned CA, leaf certificate, random database
credentials, Auth.js secret and a private local bootstrap-operator password.
The CA root is not installed into Windows or a browser. Use the normal browser
trust procedure for the public root; do not ignore TLS errors. Read the local
operator password only from the project-owned `tofu-inputs` volume, not from
terminal history, Issues, PRs or logs.

The product store is the independently named `product_acquisition` database.
It is initialized once by the existing explicit `product-acquisition-init`
command with the pre-provisioned `source_acquisition_runtime` role. The
ordinary current-application process only verifies and opens that schema; it
does not create, migrate or repair it. Its incomplete marker fails closed, so
do not retry an incomplete project.

Serving containers receive no database owner password, ZITADEL master key,
IAM-owner PAT, OpenTofu state, local-operator password or CA private key. The
UI sees only Auth.js/OIDC/public-CA inputs; Go sees only its runtime manifest
and public CA. All serving mounts are read-only.

## Retain or explicitly destroy

Stop/restart retains the named volumes and facts:

```powershell
docker compose --project-name $project --env-file .env down
docker compose --project-name $project --env-file .env up --wait
```

Destruction is intentionally not part of normal use. Before any destructive
command, resolve and display the exact project from `.env`, verify it against
`docker compose config`, and obtain the instance owner's explicit approval.
