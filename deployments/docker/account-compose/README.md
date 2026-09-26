# Local account center Compose

## Opt-in #487 image trial overlay (backend validation only)

`docker-compose.image-agent.yml` is a separate, isolated local trial overlay;
the default account-center Compose does not enable ImageAgent, expose MinIO, or
change its database roles. Use a **new** Compose project and unused loopback
identity/application/mail ports. The commercial database name must contain
`trial` or `isolated`. On the first initialization only, set
`ACCOUNT_ISOLATED_TRIAL_CATALOG=ISOLATED_TRIAL_ONLY`; unset it on restarts. The
base schema initializer refuses to re-provision that catalog, while the image
initializer has its own one-shot marker and never re-runs migrations on restart.

From this directory, after setting `COMPOSE_PROJECT_NAME`,
`ACCOUNT_IDENTITY_PORT`, `ACCOUNT_APPLICATION_PORT`, `ACCOUNT_MAIL_PORT`, and
`ACCOUNT_COMMERCIAL_DATABASE` to unique trial values:

```powershell
$env:ACCOUNT_ISOLATED_TRIAL_CATALOG = 'ISOLATED_TRIAL_ONLY'
docker compose -f docker-compose.yml -f docker-compose.image-agent.yml up -d --build image-trial-worker current-application
Remove-Item Env:ACCOUNT_ISOLATED_TRIAL_CATALOG
docker compose -f docker-compose.yml -f docker-compose.image-agent.yml ps --all
```

The backend-only command does not start `listingkit-ui`, and the #487 main-image
UI remains pending its exact Figma node. Do not call this a product/browser or
image-quality acceptance. The local Python OpenAI-protocol stub returns a
deterministic white 2×2 PNG and observed Review usage solely to check workflow,
approval and canonical accounting; it is not a visual QA provider. The
`paid_pilot` isolated catalog is only a purchasable local trial offer; a real
subscription activation and canonical member token allocation are still
required before provider dispatch. No paid or external AI key is installed.

The current API connects to the dedicated ImageAgent owner database as
`image_agent_runtime`, with only Start/Get/Approve permissions. The
organization-v1 worker connects to the same database as
`image_agent_worker_runtime`, with exactly its 16 current tables' required
read/insert/update privileges; startup verifies and refuses extra privileges.
It separately uses the existing `commercial_runtime` role for canonical member
reservation/settlement, not `commercial_owner_runtime` or the schema owner.
Only the worker-secret volume carries MinIO credentials; the API cannot read
it. All new PostgreSQL, Temporal and MinIO volumes are project-named and
persist across stop/restart. `image-trial-init` grants roles only after owner
schema installation; ordinary app/worker start verifies permissions and never
grants or migrates.

The only anonymous storage route is HTTPS localhost GET/HEAD for the immutable
`image-agent/public/` prefix, using the existing local CA. MinIO grants only
GetObject on that prefix; staging/recovery objects, listing and writes are not
public. Source/1688 URLs and arbitrary provider downloads keep the original
public-URL SSRF checks. An exact generated-image URL is accepted only when
derived from a verified durable manifest/key. This trial does not make output
URLs generally public-network-safe or authorize production deployment.

To restart without deleting the volumes, leave the catalog flag unset and run
the same `docker compose -f ... -f ... up -d` command. The independent image
initializer verifies the stored manifest and MinIO policy; an interrupted first
initialization fails closed instead of auto-repairing a partially created DB.
Use `stop`, not `down -v`, to retain trial data. Destruction or cleanup of a
retained project is a separate decision.

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

## Referral completion evidence for a separately authorized acceptance

Issue #475's original completion remains `UNKNOWN / NOT_CLASSIFIED`: its
Auth.js subject and HTTP status/error were not retained. The candidate user ID
from the official verification URL, a `CREATED` Intent and no receipt do not
bind that candidate to the original POST. Do not replay it or start dependent
payment/refund/withdrawal checks to fill this gap. This section grants no new
runtime or identity operations.

For a **future, separately authorized** attempt, reuse the passive observer in
`web/listingkit-ui/scripts/referral-completion-evidence.mjs` on the existing
Playwright page, before the operator's approved action. It sends no requests,
does not log in, and does not retry completion. Do not start another runner or
enable HAR, tracing, raw headers, session dumps or provider payload logging.

```javascript
// From the existing UI acceptance script/session. Use a new private file.
const { captureReferralCompletion } = await import("./scripts/referral-completion-evidence.mjs");
const finishEvidence = await captureReferralCompletion(page, {
  origin: "https://localhost:<authorized-application-port>",
  file: "<absolute-private-output-directory>/completion.jsonl",
});
// Existing authorized browser steps run here; this helper performs none.
// In the existing session's finally block, before closing the browser:
await finishEvidence();
```

The existing `account-browser-verification.mjs` installs the same observer for
its account pages and lists the files in `report.json`. Its default scenarios
do not click complete; each file has an `attemptCount` and independent
`NOT_RUN` / `OBSERVED` / `CAPTURE_FAILED` status. The report's overall `PASS`
still refers only to the account checks. `OBSERVED` means a request was captured,
not that completion passed; empty files mean no completion observed, **NOT_RUN**.
The script finishes capture before closing each context.
Use a fresh output directory; existing evidence files are never overwritten.
Keep the output under the private handoff directory and its Windows ACL above.
Retain the existing report's source/runtime SHA and exact origin with the file;
a control fixture is not real Auth.js/provider acceptance.

Each matching POST gets a flushed `REQUEST` row before its result and a row
with the same attempt number when the response/failure is observed. Only the
exact loopback origin/path is observed; query-bearing URLs are ignored. Output
contains HTTP status, allowlisted error code, opaque expected/authenticated
subject and Intent when available, nullable verification/receipt facts and
their source. It excludes bodies, email, cookies, credentials and proof.

- `expectedSubject` is only the public `X-Expected-User-ID` assertion.
  `authenticatedSubject` is populated **by contract inference** only when a
  recognized owner reply proves the BFF passed its Auth.js equality check;
  its source is `BFF_VALIDATED_EXPECTATION`, not a directly read session.
  BFF rejection, timeout, malformed response or missing assertion cannot
  establish that binding.
- `referral_verification_pending` / 409 gives `verifiedReadback=false` with
  source `OWNER_PENDING_CONTRACT`. A valid completion receipt gives
  `receiptPresent=true` and its opaque Intent. Receipt replay does **not**
  imply a fresh provider read or `verifiedReadback=true`.
- All other unobserved verification/receipt/Intent fields stay `null` /
  `NOT_OBSERVED`, never false or a candidate ID. An UNKNOWN response, failed
  transport or missing response is not safe failure and never authorizes a
  retry. This helper does not classify missing/conflict/expired errors.

If independent provider verified/match booleans or receipt absence are still
required, obtain them only through the already authorized current owner /
repository diagnostic boundary for that exact bound subject. Record source
and observation time separately; do not dump SQL, decrypt payloads, invoke
Complete/Resume as a read, or add a production debug route. Missing evidence
remains a limitation. This future capture cannot reconstruct the old POST.

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
