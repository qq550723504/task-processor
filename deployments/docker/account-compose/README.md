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
project name, and verify the ports are free before starting:

```powershell
Copy-Item .env.example .env
$project = "task-processor-account-center-$([guid]::NewGuid().ToString('N'))"
"COMPOSE_PROJECT_NAME=$project`nACCOUNT_IDENTITY_PORT=19443`nACCOUNT_APPLICATION_PORT=19444`nACCOUNT_MAIL_PORT=19425" | Set-Content -LiteralPath .env -Encoding ascii
foreach ($port in 19443,19444,19425) { if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) { throw "Port is already in use: $port" } }
docker compose --env-file .env up --build --wait
```

If a retained trial already uses one of the example ports, select another
three-port set and put the same values in `.env`. Do not remove or reuse an
existing project's volumes.

The delivered instance's private handoff files are at
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
docker compose --env-file .env start
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
