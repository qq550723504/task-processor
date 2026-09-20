# Local referrals Compose

This is a fresh, isolated local runtime for PR #422. It is not a deployment
path for an existing trial, shared environment, or production database.

The only host listeners are loopback HTTPS:

- `https://localhost:18443`: ZITADEL issuer and Login V2.
- `https://localhost:18444`: ListingKit UI and referral pages.
- `http://localhost:18025`: Mailpit's local-only verification-mail inbox.

Everything behind Traefik shares one Docker network namespace. Next reaches the
current application only at `127.0.0.1:8085`; the current application reaches
its identity, source-account, commercial, and referral dependencies only by
their explicit loopback ports. No database port is published to the host.

## Start an empty local runtime

From this directory, generate a fresh high-entropy project name. Check both
Compose container labels and the project-prefixed named volumes before writing
it to `.env`; a collision must stop startup. There is no default project name.

```powershell
Copy-Item .env.example .env
$project = "task-processor-referrals-$([guid]::NewGuid().ToString('N'))"
$containers = @(docker ps -aq --filter "label=com.docker.compose.project=$project" | Where-Object { $_ })
$volumes = @(docker volume ls --format '{{.Name}}' | Where-Object { $_ -like "$project-*" })
if ($containers.Count -ne 0 -or $volumes.Count -ne 0) { throw "Compose project name is already occupied: $project" }
"COMPOSE_PROJECT_NAME=$project" | Set-Content -LiteralPath .env -NoNewline -Encoding ascii
docker compose --project-name $project --env-file .env up --build --wait
```

Keep `$project` in the PowerShell session for the stop, restart, and destroy
commands below. If a new session is needed, parse it from `.env`, then verify
that Compose resolves the same name before any state-changing command.

The bootstrap container creates a local CA, TLS leaf certificate, database
passwords, an Auth.js secret, and a random password for the explicitly named
`local-bootstrap-operator@localhost` account. The root signing key is discarded
after the leaf certificate is issued. The account exists only to make an empty
local instance operable; it is not a business user or referral result.

The Compose file declares 23 project-prefixed named volumes. They are split by
consumer rather than exposed as a shared private directory: Login sees only its
login-client PAT; Traefik sees only the TLS leaf; the UI sees only its Auth.js,
OIDC, service-credential, and public-CA inputs; and the current application
sees only its manifest, provider/runtime keys, service credential, and public
CA. Database owner credentials, ZITADEL's master key, the IAM-owner PAT, the
operator password, OpenTofu state, and any private key are not mounted into
either serving frontend.

The local CA is intentionally not installed into Windows or any browser trust
store. Trust its public root through your normal browser/operator procedure
before using either HTTPS endpoint. Do not bypass certificate validation.

Read the local operator credential only from the project-owned `tofu-inputs`
volume; do not paste it into terminal history, Issues, PR comments, screenshots,
or logs. The first referral registration sends its real ZITADEL verification mail
to Mailpit. Open the Mailpit loopback inbox, use that verification link, then
return through the normal ZITADEL/Auth.js flow to complete the referral path.

Initialization uses OpenTofu 1.12.6 with the official
`zitadel/zitadel` 3.4.0 provider. The bootstrap IAM-owner PAT has its own
volume and is mounted read-only only into that one-shot initializer; Login has
a separate login-client-PAT volume. Serving receives only the generated provider
machine PAT, which has `ORG_USER_MANAGER`, and its own required files are
mounted read-only. The initializer's provider probe creates, reads, and deletes a
temporary human user through the v2 user API under that limited PAT; it stops
instead of escalating to an owner role if any of those operations is denied.

`referral-schema-init` remains the greenfield installer: it does not migrate or
repair a populated referral schema. The one-shot initializer records an
incomplete marker before its first mutation. If it fails, do not retry against
the partial local facts; use the explicit destroy command below and start a
fresh local instance.

## Stop, restart, and destroy

Stopping retains all project-owned volumes and facts. A later `up` reuses the
completed initializer marker and does not run schema or authorization setup in
the serving processes:

```powershell
docker compose --project-name $project --env-file .env down
docker compose --project-name $project --env-file .env up --wait
```

Destroying the local instance is explicit and removes only volumes named by the
project in `.env`:

```powershell
$project = (Get-Content -LiteralPath .env | Where-Object { $_ -match '^COMPOSE_PROJECT_NAME=' } | Select-Object -First 1).Split('=', 2)[1]
if ([string]::IsNullOrWhiteSpace($project)) { throw 'COMPOSE_PROJECT_NAME is required' }
$resolved = (docker compose --project-name $project --env-file .env config --format json | ConvertFrom-Json).name
if ($resolved -ne $project) { throw "Compose project resolution mismatch: $resolved" }
Write-Host "Destroying only Compose project: $resolved"
docker compose --project-name $resolved --env-file .env down -v
```

`down -v` is required after an incomplete one-shot initialization. It is
destructive for this local project's identity, Mailpit, application databases,
and its project-owned credentials; it does not operate on any other Compose
project.
