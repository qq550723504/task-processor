# Unified native local program (#619)

This profile combines the completed native capabilities in one normal application,
using their existing installers, contracts and independent restricted pools.
It is for an authorized **new, empty local project**. Retained instances and all
their named volumes remain untouched. It does not migrate data or install during
serving. Use the same four overlays on every restart.

## Build and start

Choose a fresh project and free loopback ports using the base README. A private
`.env` contains the project, three ports and `ACCOUNT_TOOL_RELEASE_DIRECTORY`
(an absolute path to this installation's plugin release, with no credentials).
`ACCOUNT_COMMERCIAL_DATABASE` defaults to `commercial`; custom names must use
at most 63 lowercase letters, digits or underscores and begin with a letter.
The existing owner names and the native names `notification_center`,
`agent_customization`, `ai_projects`, `reports` and `tool_market` are reserved.
Bootstrap and PostgreSQL initialization share this check; invalid names are
rejected before credentials or business database facts are provisioned.
First build the existing capture plugin against this program's exact HTTPS origin:

```powershell
./scripts/build-tool-market-plugin.ps1 -CaptureAppUrl 'https://localhost:YOUR_APP_PORT/capture/1688' -OutputDirectory 'ABSOLUTE_PRIVATE_RELEASE_DIRECTORY'
```

Use the usual Node dependencies documented by the extension. This is the existing
non-fixture plugin builder; the record includes its exact receiver and SHA. An
old instance's package is not reused under a different receiver origin.

From this directory:

```powershell
docker compose --env-file .env -f docker-compose.yml -f docker-compose.knowledge.yml -f docker-compose.data-services.yml -f docker-compose.modules.yml up -d --build --wait
docker compose --env-file .env -f docker-compose.yml -f docker-compose.knowledge.yml -f docker-compose.data-services.yml -f docker-compose.modules.yml --profile acceptance run --rm acceptance-fixture
```

The base README covers normal local TLS and private login handoff. No certificate
validation bypass, paid provider call or real business data is part of startup.
Local IAM identities/roles are separate from business acceptance.

## Installation order and boundaries

1. Existing base identity, membership, Commercial/Money/Resource and Store owners.
2. Existing `product-acquisition-init` once, with Collections, Supply Market and
   Data Services flags: one empty physical Product database and existing shared
   publication transaction. No second initializer retries that Product install.
   Source and Data Services keep separate bounded roles and pools; Data Services
   does not gain Supply Market/Store/Source execution privileges.
3. `native-modules-init` invokes current Notification, Agent Configuration,
   Customization, Project, Report and Cockpit installers, and the current Tool
   schema under its owner. Each dedicated database/pool remains separate.
   This installer requires a new module state; an interrupted installation is
   retained for inspection and refuses automatic replay. A completed restart
   preserves facts/configuration and does not grant or install again.
4. Private Knowledge/Tika and qualification object storage, the matched Chromium
   driver, the original Data Services Temporal worker, and the exact-origin tool
   package. Only installer containers mount administrative/schema credentials;
   serving receives its current private manifest and bounded runtime roles.
   Qualification object storage mounts only its dedicated root-password volume,
   never the SQL/module installation state containing other owner credentials.
5. Native descriptors/current IAM and truthful UI capability flags. The admitted
   optional local suspension policy in Data Services is unchanged; exact original
   grants, active users, native permissions and deadlines remain mandatory.

The bootstrap records the chosen profile and refuses to add it to a retained
base/Data-only installation or to restart its state without the original overlays.
No new domain schema, IAM, state machine, retry owner or recovery platform is added.

## Entries and missing configuration

After normal login and enterprise selection, this combination provides:

| Completed capability | Entry |
| --- | --- |
| Cockpit goals, store matrix, manual financial facts, alerts/advice | `/workbench/overview/goals`, `/workbench/overview/stores`, `/workbench/overview/alerts`, `/workbench/overview/advice` |
| Accounts, members, roles, resources, audit and store management | `/workbench/account`, `/workbench/stores` |
| Knowledge, personal projects and reports | `/workbench/ai/knowledge`, `/workbench/ai/projects`, `/workbench/ai/reports` |
| AI Workbench entry | `/workbench/ai` opens the first connected AI child; this profile opens Project Center while Chat and business-task execution remain unconfigured |
| 1688 acquisition and Product Collection | `/workbench/supply/acquisition`, `/workbench/data/mine` |
| Supply Market | `/workbench/supply/official`, `/workbench/supply/selected`, `/workbench/supply/catalogs` |
| Official/my/custom tools and capture plugin package | `/workbench/tools/official`, `/workbench/tools/mine`, `/workbench/tools/custom` |
| Tool Market entry | `/workbench/tools` opens Official Tools when the current Tool Market module is mounted; unconfigured installations keep the unavailable page |
| Agent configuration/templates and custom requests | `/workbench/agents/mine`, `/workbench/agents/custom` |
| Notifications | `/workbench/notifications` |
| Data Market and API Management | `/workbench/data/market`, `/workbench/data/api` |

API/private/platform permissions still apply. Configuration and empty lists are
real owner reads; the installer creates no sample requests, products, orders,
profits, successful jobs or balances.

The `/workbench` aggregate dashboard remains the existing unimplemented overview;
its GMV/trend/AI summary cards do not consume the completed Cockpit subpages yet.
Report reads are mounted, but saving a title report still requires the current
Product Review source below. Project/report linking and automated AI advice are
not completed capabilities and are not introduced by this composition.

The user selected on 2026-10-10: finish the unified program first and list every
missing configuration. These completed code paths remain unavailable here:

| Capability | Required current configuration |
| --- | --- |
| Ecoservices/online payments and third-party settlement | Qualified WeChat platform merchant profile/keys, fixed HTTPS notify ingress, platform-fee product and private immutable storage |
| Enterprise wallet online top-up | Qualified Alipay Page Pay or WeChat APIv3 Native merchant keys/certificates and fixed HTTPS notification ingress; separate from Ecoservices merchant admission |
| Real SMS delivery | Applicable Tencent Cloud SMS application, approved sign/template, provider credentials and the existing ZITADEL SMS callback configuration in `SMS.md` |
| AI Chat/title execution | Admitted organization model credentials, title and separate planning policies, explicit prices/budgets, Review/Asset and permitted organization scope |
| Complete image Agent | Admitted model/organization credentials, confirmed point price/limits, canonical Asset/storage, same current Product review owner and worker configuration |
| Store product/order/logistics sync | Actual official Store application and authorized channel configuration |
| Supply-chain publication | Official Store application plus current Asset/Temporal configuration and permissions |
| POD execution | Qualified SDS merchant credential, OSS hosts and current canonical Asset/Temporal configuration |
| Private platform-draft inspection | Authoritative current draft facts; the separate isolated offline trial cannot replace them in this combination |
| Saving a new title review report | Same-instance current Product Review reader; existing saved report reads do not require enabling model execution |

No fake merchant, invented price, old-organization credential, legacy trial or
always-allow policy is used to open these paths. Amazon/1688/model/payment/store
requests, credentials created via UI, business mutation and user acceptance need
their actual applicable authorization/evidence and are not proved by startup.

## Retain and restart

```powershell
docker compose --env-file .env -f docker-compose.yml -f docker-compose.knowledge.yml -f docker-compose.data-services.yml -f docker-compose.modules.yml stop
docker compose --env-file .env -f docker-compose.yml -f docker-compose.knowledge.yml -f docker-compose.data-services.yml -f docker-compose.modules.yml up -d --no-build --wait
```

Keep project/env/source/images/plugin origin and all overlays consistent. Stop
retains all database, identity, object, private manifest and Temporal volumes.
Do not use `down -v` or reinitialize a retained database. Record actual normal
login/read-only entry checks, restart, exact candidate CI/review and unrun
business/provider acceptance separately in #619 / the primary PR.
