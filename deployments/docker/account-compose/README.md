# Local account center Compose

This is a fresh, isolated local instance for the confirmed “我的账户” scope.
It reuses the existing current-application, ZITADEL Login V2, Auth.js/BFF,
source-account, commercial, referral and membership modules. It does not reuse
any other Compose project's database or identity data.

The instance provides:

- `https://localhost:19444` — normal ListingKit login and account center.
- `https://localhost:19443` — ZITADEL issuer and official Login V2.
- `http://localhost:19425` — this instance's Mailpit inbox for controlled local email.

The three public ports and the project name are configurable in `.env`; choose
values not used by either retained trial. The application and identity services
remain private to the Compose network except for those loopback endpoints.

## Start

From this directory, copy the example environment, choose a new lowercase
project name, and verify the ports are free before starting. The bootstrap
container keeps the operator/viewer/insufficient-role passwords and public CA
in project-named volumes; the extraction commands below create the private
handoff files for that exact project without printing any value:

```powershell
Copy-Item .env.example .env
$project = "task-processor-account-center-$([guid]::NewGuid().ToString('N'))"
$identityPort = 19443
$applicationPort = 19444
$mailPort = 19425
"COMPOSE_PROJECT_NAME=$project`nACCOUNT_IDENTITY_PORT=$identityPort`nACCOUNT_APPLICATION_PORT=$applicationPort`nACCOUNT_MAIL_PORT=$mailPort" | Set-Content -LiteralPath .env -Encoding ascii
$publicPorts = @($identityPort, $applicationPort, $mailPort)
if (($publicPorts | Sort-Object -Unique).Count -ne $publicPorts.Count) { throw "Public ports must be distinct" }
$fixedInternalPorts = @(1025, 3000, 3001, 5432, 5433, 5434, 5435, 5436, 8025, 8080, 8085)
$internalConflicts = @($identityPort, $applicationPort) | Where-Object { $fixedInternalPorts -contains $_ }
if ($internalConflicts.Count -gt 0) { throw "Identity/application port collides with a fixed Compose-internal port" }
foreach ($port in $publicPorts) { if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) { throw "Host port is already in use: $port" } }
docker compose --env-file .env up --build --wait
docker compose --env-file .env --profile acceptance run --rm acceptance-fixture
$handoff = Join-Path $env:LOCALAPPDATA "ListingKit\account-center\$project"
New-Item -ItemType Directory -Force -Path $handoff | Out-Null
docker run --rm -v "${project}-trusted-ca:/source:ro" -v "${handoff}:/out" alpine:3.22 sh -c 'cp /source/root-ca.pem /out/root-ca.pem'
docker run --rm --mount "type=volume,src=${project}-tofu-inputs,dst=/source,readonly" --mount "type=volume,src=${project}-acceptance-state,dst=/state,readonly" --mount "type=bind,src=$handoff,dst=/out" alpine:3.22 sh -ec 'cp /source/operator-password /out/operator-password.txt; cp /source/viewer-password /out/viewer-password.txt; cp /source/insufficient-password /out/insufficient-password.txt; cp /state/manifest.json /out/manifest.json'
icacls $handoff /inheritance:r /grant:r "$($env:USERNAME):(OI)(CI)(F)" "SYSTEM:(OI)(CI)(F)" "Administrators:(OI)(CI)(F)" | Out-Null
Write-Output "Private handoff directory: $handoff"
```

If a retained trial already uses one of the example ports, select another
three-port set and put the same values in `.env`. Do not remove or reuse an
existing project's volumes.

For a newly created instance, the private handoff files are at
`%LOCALAPPDATA%\ListingKit\account-center\<project-name>\`:
`operator-password.txt`, `viewer-password.txt`, and
`insufficient-password.txt` are the three local login passwords;
`manifest.json` contains only sanitized fixture IDs, organization names, role
keys, and URLs; `root-ca.pem` is the public CA. Existing instances created
before this fixture was added do not contain these files; create a new
isolated project instead of reusing their volumes. Do not put credentials in
the repository, `.env`, terminal history, issue, PR or logs. Henry must decide
whether to trust this CA for the current Windows user; the application never
disables TLS verification.

Sign in at the application URL as `local-bootstrap-operator@localhost`,
`local-acceptance-viewer@localhost`, or
`local-acceptance-insufficient@localhost` using the matching private password
file. The first user receives `listingkit_admin` in Organization A and
`listingkit_viewer` in Organization B; the other two receive viewer and
operator-only grants in Organization A for negative authorization checks. Use
only local addresses such as
`member@example.test` for invitations. New referral registrations use the
official Login V2 verification email delivered to this instance's Mailpit;
the Mailpit UI is not a substitute for the normal login or verification flow.

## Operate and retain

```powershell
docker compose --env-file .env stop
docker compose --env-file .env up --no-build --wait
docker compose --env-file .env ps
docker compose --env-file .env logs --tail=100 current-application listingkit-ui
```

Stopping is separate from destruction. Keep the project, named volumes and
controlled mailbox after handoff. No shared or production IAM, database or
funding service is used. Destruction is intentionally not part of delivery.

The account profile, membership and referral schemas are initialized once by
`schema-init` with dedicated runtime roles. `current-application` then verifies
the existing source-account boundary, provider credentials and independent
database pools before serving; a
misconfigured module fails startup instead of appearing as an unavailable page.

The separate membership directory PAT has the read-only ZITADEL instance role
`IAM_OWNER_VIEWER`: the directory contains role assignments from multiple
organizations, which an organization-scoped viewer cannot read. The application
still authorizes each signed-in caller against the live grant for the selected
organization and filters the provider query to that organization.
Schema initialization also places this existing read-only PAT in the private
current-application identity manifest as `tenantDirectoryToken`, including on
retained-state starts, so subscription purchase recovery can check the exact
actor grant before admitting each new effect.

The one-shot `acceptance-fixture` service is part of this isolated Compose
project only. Run it explicitly with the `acceptance` profile after the normal
stack is healthy. It uses the existing ZITADEL provisioning owner to
create/read back Organization A/B and the three user authorizations, then
writes the sanitized manifest to the project-owned `acceptance-state` volume.
It does not add a production route or insert business facts into PostgreSQL.

## Scope limits

Provider-owned identity fields remain read-only; the account-center-owned
business profile is persisted through the account profile API.
Resource balances without an authoritative source remain unavailable. Audit
history covers committed source-account receipts and member Token allocation
changes. Referral earnings and withdrawals use the immutable ledger projection
and manual-review state machine; commission entries are created only from
trusted settled-payment inputs from the commercial/payment owner. Product
acquisition remains in its retained existing instance; this account-center
instance does not make source-account login or acquisition a prerequisite.
