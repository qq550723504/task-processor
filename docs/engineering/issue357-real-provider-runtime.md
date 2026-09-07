# Issue #357 isolated official identity runtime

Status: IMPLEMENTATION_READY; admission frozen after two independent rounds.
This is the narrow L1-A runtime
contract, not a new login design or production deployment. Implemented SHA and
execution evidence belong in the Issue/PR. #358 owns browser regression and Next
authentication fixes; #357 alone owns provision, Go composition and run resources.

## Authority and boundaries

Use Issue #357 L1-A, the approved 2026-09-05 native-login simplification,
account-readonly-contract.md, issue347-commercial-read-contract.md and the current
multi-organization design. Must: fresh official provider, genuine authorization
code login, self profile, verified Effective Organization, actual commercial
reads, no business writes during reads, repeatable isolated cleanup. The accepted
ordinary grant cache window remains <=60s bounded by token expiry; commercial
reads and organization switching use the existing live policy. HTTP loopback is
local evidence only. Phone, Passkey, external IDP and SMS MFA are NOT_RUN unless
separately configured; no registration, payment, production or customer data.

## Commands and resource contract

Windows + Docker Desktop is the first supported and tested platform. Node and Go
versions follow the repository; install the existing UI lockfile with
`pnpm.cmd --dir web/listingkit-ui install --frozen-lockfile`.

From the owner checkout:

```powershell
node scripts/issue357-runtime.mjs start
node scripts/issue357-runtime.mjs check --run <uuid>
node scripts/issue357-runtime.mjs restart --run <uuid>
node scripts/issue357-runtime.mjs revoke --run <uuid> --user admin --org B
node scripts/issue357-runtime.mjs restore --run <uuid> --user admin --org B
node scripts/issue357-runtime.mjs stop --run <uuid>
# Consumer checkout, when needed:
node scripts/issue357-runtime.mjs start --web-dir <absolute-checkout>/web/listingkit-ui
```

`start` creates a new UUID. `start --run <uuid>` may only reuse an already READY,
owned run with matching clean source/web HEADs; it never repeats partial
bootstrap. `restart` retains the same owned provider/DB volumes and identities,
restarts applications, checks health, and performs no reseed. `stop` destroys only
that run's containers/network/volumes and private credentials; repeating it is
safe. A partial or ambiguous bootstrap is stopped and replaced by a fresh run.
Dirty checkout starts are labeled `sourceDirty` / `webDirty` and are development
evidence only; they cannot be reused or restarted as SHA-verified acceptance.
No generic reconcile, migration, adoption-by-name or background retry service.
`revoke`/`restore` deactivate/reactivate only a recorded authorization ID and first
verify its user/project/organization tuple using the official API. User selector
is admin/viewer; organization selector is B/C/Empty. No arbitrary provider IDs.
Every run/browser actor uses a fresh isolated browser context/profile. Cookies
are host scoped, not port scoped: do not use a personal default browser profile or
share one context between two localhost runs.

Private root: OS temporary directory / `task-processor-issue357/<uuid>/`.
Private `manifest.json` contains run identity, source/web SHA, registered resources,
origins, provider-generated IDs and test credential file paths. Private generated
Compose/bootstrap/runtime files stay under this root. Restrict Windows ACL to the
current user and SYSTEM; generated secrets are in `owner-secrets.json`, never in
the consumer manifest. Pin Docker commands to Docker Desktop's local Linux named
pipe; inherited contexts, remote hosts and application configuration are ignored.
Restrict generated files to the
current user and SYSTEM (Unix mode 0700/0600). Public evidence contains allowlisted
status, version/digest, IDs, origins, checks and cleanup results, never secrets.

Consumer manifest schema v1 (no optional alias names): `schemaVersion:"issue357-v1"`,
`runId`, `status`, `sourceSha`, `webSha`, `origins:{web,go,issuer}`, `instanceId`,
`projectId`, `organizations:{A,B,C,Empty,D}` where each value is `{id,name}`,
`users:{admin,viewer,"no-org"}` where each value is
`{id,homeOrganizationId,credentialFile}`. Each absolute credentialFile is private
JSON `{username,password}`. None contains a pre-issued application session.
Other environment-owner metadata is not a browser contract. All consumers require
`status:"ready"`; historical/stopped manifests are not online environments.

Names: Compose project `issue357-<uuid>`; services `proxy`, `zitadel-api`,
`zitadel-login`, `identity-db`, `commercial-db`; network and volumes use the same
run prefix. Every Docker resource has matching run ownership labels; actual
resource IDs are recorded and checked before restart/stop. No external network,
existing volume, Docker socket mount or existing `local-zitadel` resource is used.

Reuse the official deployment topology and pinned repository images:
`ghcr.io/zitadel/zitadel:v4.17.1`, `ghcr.io/zitadel/zitadel-login:v4.17.1`,
`traefik:v3.6.8`, `postgres:17.2-alpine`. A dedicated generated Compose configuration
removes the base deployment's fixed names and uses Traefik's file provider, scoped
to this run, instead of discovering all host containers. Record image digests.

Allocate free random ports for issuer proxy, Go, Next and commercial PostgreSQL;
bind only 127.0.0.1, reject conflicts, never kill a port occupant. Identity PG and
internal API/Login ports are not published. Issuer is `http://localhost:<idpPort>`;
browser and host Go/Next use that exact issuer. Login V2 calls
`http://zitadel-api:8080` with canonical Host/forwarded scheme; Traefik routes
`/ui/v2/login` to official Login and other provider paths to h2c API. Next origin
is `http://localhost:<webPort>`; Go origin is `http://127.0.0.1:<goPort>`.

Next uses a run-private copy of tracked UI source (excluding all .env files), with
installed dependency reuse and separate build output, so concurrent runs neither
read real configuration nor share Next locks. Child environment is an explicit
OS/tool allowlist plus generated variables, never inherited application settings.
No existing .env, kube config, customer DB or IAM is read. No mock flags are set.

## Configuration and identities

Only generated inputs are accepted; no caller DSN/issuer/token override.
Next variables: `AUTH_SECRET`, `AUTH_URL`, `LISTINGKIT_PUBLIC_BASE_URL`,
`ZITADEL_ISSUER_URL`, `ZITADEL_CLIENT_ID`, `ZITADEL_CLIENT_SECRET`,
`ZITADEL_REDIRECT_URI`, `ZITADEL_POST_LOGOUT_REDIRECT_URI`, `ZITADEL_SCOPES`,
`LISTINGKIT_SERVICE_API_BASE`, `COMMERCIAL_API_ORIGIN`. Secrets use cryptographic
random generation; client secrets/IDs come from this provider's APIs. API/basic
introspection credentials are separate from the OIDC application. Existing
`zitadelprovision` owns project/role/application provisioning; its fixed local
redirect guard needs bounded same-loopback-origin/random-port support, retaining
the exact callback path. No second OIDC authentication mechanism is added.

Callback: `<webOrigin>/api/auth/callback/zitadel`; post logout: `<webOrigin>`.
Scopes reuse `zitadelprovision.RecommendedScopes(projectID)` (openid/profile/email,
resourceowner, ZITADEL API audience, application project audience and project role
claims). Add offline_access only with provider REFRESH_TOKEN application grant for
#358's actual refresh verification; keep production auth defaults unchanged.
PKCE/state/nonce follow the installed Auth.js provider/core, with observed checks
recorded; no hand-built callback, token or cookie.

Provider setup uses official first-instance machine PAT initialization and
supported management/v2 APIs, only after local resource/issuer ownership checks.
Initial administrator and Login V2 service credentials are generated separately.
Bootstrap management credential is private setup/control-only; it is never
injected into Next/Go, used as end-user token or mounted into Login V2. Login V2
gets only IAM_LOGIN_CLIENT PAT. The environment owner's explicit revoke/restore
control may access the setup credential; no control endpoint in application APIs.

Create synthetic email/password users `admin`, `viewer`, `no-org` in Home A via
official human-user API; email marked verified locally, password change disabled,
no SMTP/SMS delivery. No real address or phone. A owns the project and user
identities but gives these users no project role in A. Grant project to B/C and
Empty; admin has listingkit_admin in B/C/Empty; viewer has listingkit_viewer;
no-org has none. D is an ungranted organization. Save actual returned opaque IDs,
never infer tenant ownership from numeric equality. B/C have distinct synthetic
usage (1 vs 2 current-month committed operations); Empty has no subscription.
Only current UpsertPlan/ApplyPlan and UsageLedger operations seed commercial data.
The project must use an explicit local opt-in `projectRoleCheck:false` as well as
`hasProjectCheck:false`, permitting no-org identity authentication. Existing
provision callers retain roleCheck=true; only this newly owned local project opts
out. This is not authority for organization/commerce access: Go still checks actual
grants and Casbin. First-run OIDC token lifetimes: access/ID 120s, refresh idle 20m,
refresh absolute 60m; #358 verifies actual session expiresAt after normal login.
`LISTINGKIT_ACCEPTANCE_TOKEN_FILE` stays unset; no bearer token export.
Architect task `01a0797d-3d60-7441-a3f5-f073fdd430b9` confirmed this narrow
constraint: checks apply to every application in this run's business Project,
never the built-in ZITADEL Project. Both checks are separate explicit opt-ins;
create/update/repeated ProvisionLocalApplications preserve them. Keep
projectRoleAssertion=true and actual audiences/scopes. LocalOrigin must be a
canonical loopback origin with allocated port; exactly one callback and logout
pair, no extra URI/userinfo/path/query/fragment, no localhost/127.0.0.1 substitution.
Unspecified options preserve all prior defaults. Source evidence:
https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/project.proto#L26-L31 .

## Owner, persistence and lifecycle

Official ZITADEL owns users/organizations/projects/grants/credentials and its
session/audit PostgreSQL. Auth.js owns browser protocol cookies and application
session. Current authruntime verifier/UserInfo/AuthorizationClient and
workbenchcontext resolver/Casbin own request identity/scope. Current subscription
and ledger owners own commercial facts keyed by exact Effective Organization ID.
No tenantbridge, old Task/Workspace runtime, extra IAM, grant source or DTO.

The narrow opt-in Go harness delegates to existing workbench and commercial
module builders and route assembly; it adds no production default wiring or
business routes. A separate setup phase migrates only the new commercial DB and
seeds via domain operations. Runtime has SELECT-only DB credentials with
default_transaction_read_only=on, statement timeout and bounded pool. No schema
or default-plan constructor runs during serving. Snapshot all saas_* row values
and xmin after setup and before shutdown; equality proves zero business writes.

Persistent boundaries are Docker resources/volumes, private files, IAM DB,
commercial DB, and browser state. There is no shared transaction across them.
Within commercial seeding reuse existing domain transactions. Since all assets
are disposable and not externally observed business assets, ambiguous setup
mutation, response loss or cancellation aborts this run; do not retry non-idempotent
creates. Persist resource registration before each create; reconcile exact labels
only for cleanup when creation response is lost. No automatic recovery beyond
bounded official health polling. One exclusive run lock owns setup/revoke/stop.

READY is written atomically only after all bootstrap checks and applications are
healthy. Lifecycle: creating -> provider-ready -> seeded -> ready -> stopping ->
stopped; any partial phase -> failed -> stopping. Failure leaves a safe status and
owned cleanup manifest. A source/config mismatch, unowned resource, invalid path,
DSN outside assigned commercial DB, wrong discovery issuer or occupied port fails
closed. Stop never deletes by a broad prefix. Abrupt termination requires explicit
stop of the same manifest; it does not count as successful normal cleanup.
Source/config fingerprint restricts reuse/restart, never cleanup. Source changes,
already exited processes and stale locks must not block owned stop. Verify process
identity/start time before signaling (a reused PID is foreign). Snapshot mismatch
still triggers resource cleanup and retained FAIL evidence. Keep ownership metadata
until every cleanup step succeeds, so partial cleanup can be retried safely.
Control failures return nonzero and a safe local code (INVALID_RUN,
OWNERSHIP_MISMATCH, RUN_BUSY, DEPENDENCY_UNAVAILABLE, MUTATION_OUTCOME_UNKNOWN),
never raw provider content. An uncertain revoke/restore outcome requires a fresh
official read of the recorded tuple before any further mutation. The same lock
serializes these controls with restart/stop. No background replayer is created.
Go receives private generated JSON with loopback issuer/authorization URL,
project/API client ID and secret, assigned Go port, and commercial reader host,
port/database/username/password; its dedicated config validates those values
against manifest-derived assigned resources before owner construction.

Health: provider `/debug/ready`, `/.well-known/openid-configuration` with exact
issuer/endpoints; official Login `/ui/v2/login/healthy`; Go health plus unauthenticated
account/commercial requests denied; Next `/api/auth/providers` with configured
ZITADEL. Startup deadline 5 minutes, setup/API calls <=30s, body <=1MiB; existing
account/commercial request/body/deadline limits remain authoritative. Normal stop
first ends Next, then Go drains <=10s, checks DB snapshot, closes connections, then
stops exact Docker resources and removes exact owned volumes/network. Verify PID,
ports and Docker IDs gone before success; retain only sanitized audit evidence.

## Required evidence and #358 handoff

TDD RED before implementation: reject shared/nonloopback/mismatched manifests,
foreign resource labels, unsafe cleanup paths, inherited credentials, concurrent
owner and invalid callback; fail missing runtime/true reader assembly tests.
Then real runtime: official digest/discovery, supported API IDs/grants, a fresh
browser bare /login -> official password -> code callback -> Auth.js session,
actual profile/organization/commercial through current client/BFF/Go, viewer denial,
no-org self read, B/C distinction, Empty null and D denial. Verify session JSON
contains no access/refresh/ID token. No storageState/cookie injection or response mock.

Exercise provider removal/restoration of this run's project authorization: live
commercial denial and cached account convergence <=60s. Preserve official logout
semantics: application logout clears local session and redirects to end-session;
do not claim global access-token revocation from cookie removal. #358 owns detailed
logout/relogin, late responses, changing user/org, refresh and returnTo/browser
matrices; #357 supplies real controls and performs minimal genuine login/read/logout.
Test restart with same IDs and new normal login; second concurrent run must not
reuse resources; repeat stop and verify first/second run isolation and zero writes.

Planned handoff is this document plus an Issue #358 comment. Implemented handoff
must contain pushed SHA, actual commands/OS, private path (no values), public IDs,
origins, health/revoke/restore/stop details, explicit instance owner, actual
PASS/FAIL/NOT_RUN and limitations. Bootstrap/health never implies browser PASS.
