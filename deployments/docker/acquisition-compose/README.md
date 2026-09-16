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

This supporting composition is not a user workflow on its own. It must land
with PR #430's acquisition page/BFF slice before a fresh instance can present
the operator workflow; until both changes are on the same source baseline, do
not describe this Compose project as a usable acquisition delivery.

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

## Trust and sign in

After `up --wait` succeeds, the instance owner can make a private, owner-only
handoff directory. This copies only the public root CA and the operator's
password from the two named volumes. It never mounts a serving container or
exports a CA private key, PAT, database credential, or OpenTofu state. Run this
in PowerShell from this directory; it deliberately fails rather than reuse an
unexpected project, volume, helper, or output directory.

```powershell
$projectLine = Get-Content -LiteralPath .env | Where-Object { $_ -match '^COMPOSE_PROJECT_NAME=task-processor-acquisition-[0-9a-f]{32}$' }
if (@($projectLine).Count -ne 1) { throw 'Expected one generated acquisition COMPOSE_PROJECT_NAME in .env' }
$project = $projectLine.Substring('COMPOSE_PROJECT_NAME='.Length)
$caVolume = "$project-trusted-ca"
$passwordVolume = "$project-tofu-inputs"
foreach ($volume in @($caVolume, $passwordVolume)) {
  $volumeProject = docker volume inspect $volume --format '{{ index .Labels "com.docker.compose.project" }}'
  if ($LASTEXITCODE -ne 0 -or $volumeProject -ne $project) { throw "Not this project-owned volume: $volume" }
}

$deliveryDir = Join-Path $env:LOCALAPPDATA "ListingKit\acquisition\$project"
if (Test-Path -LiteralPath $deliveryDir) { throw "Private handoff directory already exists: $deliveryDir" }
New-Item -ItemType Directory -LiteralPath $deliveryDir | Out-Null
$createdDeliveryDir = $true
$currentUser = [Security.Principal.WindowsIdentity]::GetCurrent().Name
icacls $deliveryDir /inheritance:r /grant:r "${currentUser}:(OI)(CI)F" | Out-Null
if ($LASTEXITCODE -ne 0) {
  if ($createdDeliveryDir -and (Test-Path -LiteralPath $deliveryDir) -and @((Get-ChildItem -LiteralPath $deliveryDir -Force)).Count -eq 0) {
    Remove-Item -LiteralPath $deliveryDir -Force
  }
  throw "Could not restrict the private handoff directory: $deliveryDir"
}

$helper = "$project-handoff-$([guid]::NewGuid().ToString('N'))"
if (docker container inspect $helper 2>$null) { throw "Unexpected existing helper: $helper" }
try {
  docker run --name $helper --rm --network none --read-only `
    --mount "type=volume,src=$caVolume,dst=/source-ca,readonly" `
    --mount "type=volume,src=$passwordVolume,dst=/source-password,readonly" `
    --mount "type=bind,src=$deliveryDir,dst=/delivery" `
    alpine:3.22 /bin/sh -ec 'umask 077; test -s /source-ca/root-ca.pem; test -s /source-password/operator-password; cp /source-ca/root-ca.pem /delivery/root-ca.pem; cp /source-password/operator-password /delivery/operator-password.txt; chmod 600 /delivery/root-ca.pem /delivery/operator-password.txt'
  if ($LASTEXITCODE -ne 0) { throw 'Private local-login handoff failed' }
} finally {
  if (docker container inspect $helper 2>$null) { docker rm -f $helper | Out-Null }
}
```

The helper has no network, has a read-only root filesystem, mounts both source
volumes read-only, and is removed by `--rm` (with cleanup limited to that
generated helper name). The password is copied into the owner-only directory;
the commands never print it. Do not move that file into the repository, an
environment file, a terminal command, an Issue, a PR, or a log.

Verify the public CA before trusting it. The hash is safe to display because it
is a public certificate fingerprint:

```powershell
$fingerprint = (Get-FileHash -LiteralPath (Join-Path $deliveryDir 'root-ca.pem') -Algorithm SHA256).Hash
Write-Host "Local acquisition root CA SHA-256: $fingerprint"
```

Henry must manually compare that fingerprint, then decide whether to trust this
specific local CA for the current Windows user. The following is intentionally
not run by Compose or an agent; it does not modify machine-wide trust and does
not bypass TLS validation:

```powershell
Import-Certificate -FilePath (Join-Path $deliveryDir 'root-ca.pem') -CertStoreLocation Cert:\CurrentUser\Root
```

Sign in at `https://localhost:18444` as
`local-bootstrap-operator@localhost`. Open the private password file locally
with `notepad (Join-Path $deliveryDir 'operator-password.txt')`, type it into
the browser, and close the editor afterwards; do not read it into PowerShell or
paste it into a command. Browser login and real 1688 acquisition remain manual
trial steps, not evidence from this Compose setup.

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
