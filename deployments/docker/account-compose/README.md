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
container keeps the operator password and public CA in project-named volumes;
the extraction commands below create the private handoff files for that exact
project without printing either value:

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
$handoff = Join-Path $env:LOCALAPPDATA "ListingKit\account-center\$project"
New-Item -ItemType Directory -Force -Path $handoff | Out-Null
docker run --rm -v "${project}-trusted-ca:/source:ro" -v "${handoff}:/out" alpine:3.22 sh -c 'cp /source/root-ca.pem /out/root-ca.pem'
docker run --rm -v "${project}-tofu-inputs:/source:ro" -v "${handoff}:/out" alpine:3.22 sh -c 'cp /source/operator-password /out/operator-password.txt'
icacls $handoff /inheritance:r /grant:r "$($env:USERNAME):(OI)(CI)(F)" "SYSTEM:(OI)(CI)(F)" "Administrators:(OI)(CI)(F)" | Out-Null
Write-Output "Private handoff directory: $handoff"
```

If a retained trial already uses one of the example ports, select another
three-port set and put the same values in `.env`. Do not remove or reuse an
existing project's volumes.

For the retained delivered instance, the private handoff files are at
`%LOCALAPPDATA%\ListingKit\account-center\task-processor-account-center-20260919-a\`:
`operator-password.txt` is the operator password and `root-ca.pem` is the
public CA. Do not put credentials in the repository, `.env`, terminal history,
issue, PR or logs. Henry must decide whether to trust this CA for the current
Windows user; the application never disables TLS verification.

Sign in at the application URL as `local-bootstrap-operator@localhost` using
the private password file. Use only local addresses such as
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

The membership and referral schemas are initialized once by `schema-init` with
dedicated runtime roles. `current-application` then verifies both schemas,
provider credentials and the independent database pools before serving; a
misconfigured module fails startup instead of appearing as an unavailable page.

## Scope limits

Profile fields without an existing current owner remain read-only/unavailable.
Resource balances without an authoritative source remain unavailable. Audit
history currently covers committed source-account register/enable/disable
receipts only. Referral earnings, commission and withdrawal decisions are not
implemented or inferred. Product acquisition remains in its retained existing
instance; this account-center instance does not make source-account login or
acquisition a prerequisite.
