# Local account center Compose

## Enterprise verification (optional, #510)

The account page `/workbench/account/profile/verification` supports the first
enterprise authentication request through Tencent eSign's enterprise self-built
application. Only a current organization administrator can create an application
or read its result; the hosted link is returned only to its original applicant.
The applicant must have a verified `+86` phone in ZITADEL. Personal verification,
repeat enterprise applications and changing the applicant remain unavailable.
Authentication never grants platform permissions.

This feature is **disabled by default**. To prepare an authorized isolated trial:

1. Run the existing schema-init step with the `source_account_owner` role. It
   installs `subject_verification_applications` and `subject_verification_messages`
   in `source_accounts`; `init.sh` grants only SELECT/INSERT/UPDATE to the existing
   runtime role. Runtime startup does no DDL. Existing business data must not be
   deleted to enable this feature.
2. Create a private, absolute-path JSON file outside the checkout with string
   fields `Scope`, `SecretID`, `SecretKey`, `OperatorID`, `CallbackSigningKey`,
   `CallbackEncryptionKey`, `DataEncryptionKey`. `Scope` is an immutable bounded
   identifier for this Tencent account/application/environment. `OperatorID` is
   the configured Tencent operator, never the logged-in platform user. Signing
   key is at least 16 bytes; callback encryption key is Tencent's raw 32-byte AES
   key; data key is a separate random 32-byte key encoded in standard Base64.
   Use distinct keys, private file permissions (0600 on Linux; restricted ACL
   on Windows), and back up the data key with the database. Never commit or log
   the file. Changing scope or data key requires explicit operational review;
   this version does not rotate, recover or rebind existing applications.
3. For a directly launched API, set `TENCENT_ESIGN_VERIFICATION_CONFIG_FILE` to
   that file, then use the normal `go run ./cmd/current-application -config
   <private-runtime-manifest>` command. For Compose, set
   `TENCENT_ESIGN_VERIFICATION_CONFIG_HOST_FILE` and add the optional
   `docker-compose.verification.yml` overlay to the existing Compose command.
   No credentials belong in the UI or its environment.
4. Configure Tencent's encrypted, signed callback to the API's HTTPS endpoint
   `/api/v1/callbacks/tencent-esign/verification`. The current local Compose
   stack's UI proxy does not publish this API endpoint: the authorized trial
   must separately provide a reachable HTTPS callback ingress. Do not point it
   at Next.js `/api/account/verification`, which requires a browser session.
5. Log in as the current enterprise administrator, open the page above, enter
   the registered name and 18-character credit code, review consent, and submit.
   Continue in Tencent, then return and use **刷新认证状态**. Refresh only reads
   the platform's saved result; a browser return never marks authentication as
   successful. After completion, check the result after restarting the API.

State and message receipts are stored together in PostgreSQL. Links are encrypted
and removed on verification; raw callback bodies, full phone numbers, identity
documents, authorization letters and face images are not retained locally. The
minimal company and provider reference facts remain stored. Expired links are
hidden. A create timeout or lost response remains **申请结果待核实** and never
automatically creates another vendor request; an authentic late callback can
still complete the original request. This first version has no manual-success
override or resubmission/recovery button.

Development tests use synthetic callbacks and an isolated PostgreSQL container.
Real Tencent authorization, callback reachability, supplier sample validation
and user acceptance are **NOT_RUN**; these are trial/opening conditions, not
claims established by local tests. Keep the optional configuration unset until
the authorized trial is ready. Design and frozen boundary:
[subject-verification design §13](../../../docs/architecture/subject-verification-tencent-esign-design.md#13-本轮企业首次引导认证合同).

## PostgreSQL layout (new local projects)

The standard local stack has **one application PostgreSQL instance**
(`business-db`, private `127.0.0.1:5433`) and **one independent ZITADEL instance**
(`identity-db`, private `127.0.0.1:5432`). The six application databases remain
separate databases, not schemas in one database. The base stack initializes the
first four; the existing image overlay initializes Product acquisition and
ImageAgent in the other two. Creating those two empty databases does not enable
their application features.

| Module / fact owner | Tables (all in `public`) | Logical database | Runtime role | Separate application pool / maximum connections | Instance |
| --- | --- | --- | --- | --- | --- |
| Source Account Registry; Account business profile | `source_account_resources`, `source_account_operations`; `account_business_profiles`, `account_business_profile_audit_events` | `source_accounts` | `source_account_runtime` | `sourceAccountDatabase` / 4 | `business-db:5433` |
| Commercial reads, usage reservation/settlement, member allocation | `saas_plans`, `saas_tenant_subscriptions`, `saas_tenant_entitlements`; `saas_usage_buckets`, `saas_usage_events`, `saas_usage_event_outbox`, `saas_subscription_audit_logs`; `account_member_token_locks`, `account_member_token_allocations`, `account_member_token_operations`, `account_member_token_audit_events` | `commercial` (or `ACCOUNT_COMMERCIAL_DATABASE`) | `commercial_runtime` | `commercialDatabase` / 4; image worker has its own commercial pool / 4 | `business-db:5433` |
| Subscription/catalog owner; commercial orders; organization resources | `saas_modules`, `saas_plans`, `saas_plan_modules`, `saas_tenant_subscriptions`, `saas_tenant_entitlements`, `saas_usage_counters`, `saas_usage_counter_adjustments`, `saas_subscription_audit_logs`, `saas_subscription_activation_fences`, `saas_purchased_plan_activations`; `commercial_offers`, `commercial_quotes`, `commercial_orders`, `commercial_order_items`; `saas_organization_resource_buckets`, `saas_organization_resource_operations`, `saas_organization_resource_source_claims`, `saas_organization_resource_events`, `saas_organization_resource_reservations`, `saas_organization_resource_debts`, `saas_organization_resource_audit_logs` | same commercial database | `commercial_owner_runtime` (separate password) | `commercialOwnerDatabase` / 2 | `business-db:5433` |
| Referral registration/economics; canonical Money owner | `referral_codes`, `registration_intents`, `referral_relations`, `referral_receipts`, `registration_admission_buckets`, `referral_earning_claims`, `referral_earnings_ledger`, `referral_refund_operations`, `referral_chargeback_operations`, `referral_earnings_projection`, `referral_withdrawals`, `referral_withdrawal_operations`, `referral_earnings_audit_events`; `ledger_payment_settlements`, `ledger_refund_settlements`, `ledger_chargeback_settlements`, `ledger_payout_methods`, `ledger_payout_method_operations`, `ledger_organization_wallets`, `ledger_organization_wallet_entries`, `ledger_organization_wallet_reservations`, `ledger_organization_wallet_reserve_decisions`, `ledger_organization_topup_settlements`, `ledger_organization_wallet_reversals` | `referrals` | `referral_runtime` | `referrals.referralDatabase` / 4 | `business-db:5433` |
| Organization membership operation receipts/audit (identity remains ZITADEL-owned) | `organization_member_operations`, `organization_member_audit_events` | `membership` | `organization_membership_runtime` | `membership.database` / 4 | `business-db:5433` |
| Acquisition; SRC publication and Product Catalog | `product_acquisition_operations`, `product_source_publications`, `product_source_publication_receipts`, `product_snapshot_versions`, `product_snapshot_heads` | `product_acquisition` | `source_acquisition_runtime` | `productAcquisitionDatabase` / 4 | `business-db:5433` |
| Organization ImageAgent; approved Product assets; AI invocation records | `image_agent_v2_runs`, `image_agent_v2_plans`, `image_agent_v2_slots`, `image_agent_v2_attempts`, `image_agent_v2_events`, `image_agent_v2_asset_catalog`, `image_agent_v2_asset_catalog_manifests`, `image_agent_v2_projection_snapshots`, `image_agent_v2_projection_commits`, `image_agent_v2_slot_external_effects`, `image_agent_v3_slot_external_effects`; `product_approved_assets`, `product_approval_receipts`, `product_approved_inventory_heads`, `product_approved_inventory_version_heads`; `ai_client_credentials`, `ai_invocations` | `image_agent` | `image_agent_runtime` (API), `image_agent_worker_runtime` (worker) | `imageAgent.database` / 4; worker `database` / 4 | `business-db:5433` |
| ZITADEL / official Login V2 | ZITADEL-managed identity schema | `zitadel` | existing ZITADEL provisioning configuration, unchanged | ZITADEL-managed pools, separate from application | `identity-db:5432` |

The table lists describe ownership, **not blanket table permissions**. The
commercial read role cannot write plans/entitlements or read owner-only money
and order tables; the ImageAgent API and worker keep their different exact
allowlists, and the old v2 slot-effect table is not granted to either current
role. Existing startup permission checks remain authoritative. Installed metadata
tables are `goose_source_account_registry_version` and
`goose_account_profile_version` in source_accounts, and
`goose_account_member_token_allocation_version` in commercial; these are
schema-owner-only. The existing subscription installer also creates
`saas_store_quota_allocations` and `saas_store_quota_buckets` in commercial;
this local composition grants neither commercial runtime role access to them.
Their presence does not open an additional store-quota product flow.

The seven current-application pools total at most 26 connections when the image
overlay is enabled (18 without it); the worker adds two independent pools of 4.
The server retains PostgreSQL's existing 100-connection limit. No shared pool,
cross-database transaction, FDW, or dblink is introduced. In particular, Money
remains in `referrals` and orders/resources in commercial: existing owner calls,
idempotency receipts and recovery retain their original transaction boundaries.

`business-db-init.sh` uses the official PostgreSQL empty-PGDATA initialization
hook to create the six databases and roles and revoke PUBLIC CONNECT/TEMP.
Both PostgreSQL instances explicitly use SCRAM on TCP, including loopback;
the shared network namespace must never turn `127.0.0.1` into passwordless trust.
Only this cluster bootstrap uses `business_cluster_admin`; its credential is
not mounted into schema installers, application or worker. Schema installers
connect as `source_account_owner`, `commercial_schema_owner`, `referral_owner`,
`membership_owner`, `acquisition_owner`, and `image_agent_owner`, respectively.
These logins have no superuser, CREATEDB, CREATEROLE, replication, BYPASSRLS or
role-membership privileges; each owns only its database. Runtime roles receive
CONNECT only to their own initialized database and retain existing table grants.
An incomplete cluster init never passes its health check; incomplete schema
initialization retains its existing fail-closed marker behavior.
Application database credential files remain mode 0600, owned by UID/GID 70
used by the pinned `postgres:17.2-alpine` image's initialization hook. The
root schema-install containers can read them, but serving containers receive
only their existing private runtime manifests. An unreadable or malformed
credential aborts initialization before a passwordless role can be created.

Use a **new project name and empty volumes** for this layout. It is not an
upgrade/migration command for retained multi-instance projects. Their containers,
volumes and original checkout remain in place; operate them with that checkout.
The new bootstrap refuses their old state instead of rewriting connections.

All six application databases persist in `${COMPOSE_PROJECT_NAME}-business-db`;
ZITADEL persists in `${COMPOSE_PROJECT_NAME}-identity-db`. Stop/restart commands
below retain both plus all project-specific credentials. The application
databases now share PostgreSQL restart, resource, WAL and physical backup/recovery
scope; logical database separation does not provide independent instance failure
domains. Keep the entire project volume set for local retention; no automatic
backup/migration platform or recovery of an old project's data is added.
See PostgreSQL's [database hierarchy](https://www.postgresql.org/docs/17/manage-ag-overview.html)
and [database privileges](https://www.postgresql.org/docs/17/ddl-priv.html).

For a new project with all six schemas initialized by the overlay, the narrow
PostgreSQL permission regression uses actual TCP connections and rolled-back
DDL probes. It checks all 14 schema/runtime roles, wrong passwords, cross-database
CONNECT, role escalation, and commercial reader/owner separation:

```powershell
docker compose --env-file .env cp ../../../scripts/tests/account-compose-postgres-permissions.sh business-db:/tmp/permissions.sh
docker compose --env-file .env exec -T business-db sh /tmp/permissions.sh
```

The four-database base mode does not satisfy this six-schema test's prerequisites.
It is not a test of external identity, payment or image-generation acceptance.

## Opt-in #487 image trial overlay (infrastructure preparation only)

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

This command prepares isolated infrastructure, health checks and persistent
storage only; it does not enable a usable image-generation workflow. Current
main-image candidates and Start return `503 IMAGE_UNAVAILABLE` with the overlay's
absent image price. The approved implementation uses enterprise AI points and
an administrator-configured member UTC monthly limit, not token allocation.
The overlay therefore cannot validate new generation,
the complete generation-to-human-approval flow, or canonical generation accounting.
Existing saved-run reads and exact approval-receipt recovery remain available
under their original authorization checks; they do not reopen new generation.

The backend-only command does not start `listingkit-ui`, and the #487 main-image
UI remains pending its exact Figma node. Product/browser and image-quality
acceptance remain unavailable. The local Python OpenAI-protocol stub's
deterministic white 2×2 PNG is not image-quality evidence. Its seven-token Review
usage belongs only to historical Review tests, not the current one-edit,
zero-Extract, zero-model-Review flow or its generation metering. The `paid_pilot`
isolated catalog is only a purchasable local trial offer; subscription activation
or member token allocation does not supply an image price/member point limit or
unlock dispatch. No paid or external AI key is installed.

For a separately authorized configured runtime, the current manifest's
`imageAgent.generation.priceVersion` and `pointsPerImage` must match the worker's
`imageagent.generation` configuration. Neither has a default. An incomplete
price or a missing verified commercial owner pool fails closed. This admits
the HTTP operation, not proof of worker health or provider readiness: the worker
still resolves the exact scoped GRSAI `gpt-image-2.5` credential/route and atomically
reserves member-limit and enterprise points before its unique dispatch. Do not
enable this overlay by adding a sample price or treating its old stub as that route.

The current API connects to the dedicated ImageAgent owner database as
`image_agent_runtime`, with the existing Start/Get/Approve database privileges;
these privileges alone do not enable the unpriced HTTP Start route. The
organization-v1 worker connects to the same database as
`image_agent_worker_runtime`, with exactly its 16 current tables' required
read/insert/update privileges; startup verifies and refuses extra privileges.
Its existing token/review connection remains `commercial_runtime`. The explicit
`commercialOwnerDatabase` pool uses `commercial_owner_runtime` for image AI point
reservations, original-month counters and settlement; neither uses a schema-owner
role. Startup verifies the existing resource owner's exact required rights,
including its two member-limit tables, without granting or migrating them.
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
$fixedInternalPorts = @(1025, 3000, 3001, 5432, 5433, 7233, 8025, 8080, 8085, 9000, 18080)
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
