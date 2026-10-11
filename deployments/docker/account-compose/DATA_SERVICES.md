# Native Data Services profile (#619)

The integration candidate in #619 consumes
the frozen [Data Services architecture](../../../docs/architecture/data-services-v1.md)
and [feature handoff](../../../docs/engineering/data-services-runtime-handoff.md).
The user selected the current formal IAM boundary without a local enterprise
business-suspension feature. The independently admitted
[native authorization supplement](../../../docs/architecture/data-services-native-authorization.md)
consumes the formal Workbench resolver's optional deny-only port unchanged.
Exact original member grants, active users, current permissions and deadlines
remain required; explicitly supplied status checks still deny failures/suspension.
No always-allow adapter or new local status owner is supplied.

## Installation and configuration

This profile is for an authorized **new, empty, isolated project**. Preserve
existing projects, checkouts and named volumes. It does not install into retained
Product databases, migrate data, repair schemas during serving, or replace a
running instance. Do not combine it with the Image or Issue #36 isolated trial
profiles: those have their own Product installer.

The base schema owner runs the existing `product-acquisition-init` once with
`collections: true` and `dataServices: true`, before marking installation
complete. Current Source, Catalog, Collection, credentials, jobs and customization
tables share one Product database and the existing publication transaction.
`source_acquisition_runtime` retains its existing table permissions.
`data_services_runtime` receives a separate pool, at most four connections,
with only the admitted Data Services publication/Collection/service tables.
Neither pool owns the schema. Serving verifies schema and grants without DDL.
Resource stays in its existing independent database and recovery service.

The private `current-application.json` receives `productCollections`,
`productAcquisitionDatabase`, and an opt-in `dataServices` object containing:

| Field | Profile value |
| --- | --- |
| database | Same Product host/port/database, `data_services_runtime`, maxConnections 4 |
| temporalAddress / temporalNamespace | `127.0.0.1:7233` / `default` |
| browserExecutable | `/opt/data-services/chromium` |
| driverDirectory | `/opt/data-services/playwright` |
| enabledSites | us, uk, de, fr, it, es, ca, jp, au, mx, br, in, ae, sa |
| trustedProxyCIDRs | Explicit local single TLS hop: 127.0.0.1/32, ::1/128 |

Passwords stay in project-specific private named volumes. Do not print the
manifest, put credentials in `.env`, or include them in Issues/PRs.

The `data-services-runtime` image installs Chromium and the driver using the
repository's locked Playwright Go SDK during build. Serving checks the explicit
paths and never downloads browser dependencies. This does not prove that Amazon
pages can be collected. No paid call, login, CAPTCHA bypass or proxy rotation is
part of startup.

## Start and retain

Use the base README's new-project port checks, private login handoff and normal
TLS setup. Set a fresh project name and three free ports in a private `.env`.
Always include both files for this project:

```powershell
docker compose --env-file .env -f docker-compose.yml -f docker-compose.data-services.yml up -d --build --wait
docker compose --env-file .env -f docker-compose.yml -f docker-compose.data-services.yml --profile acceptance run --rm acceptance-fixture
docker compose --env-file .env -f docker-compose.yml -f docker-compose.data-services.yml ps --all
```

Report actual startup and normal-login evidence separately from developer checks
and business acceptance. Missing installed schema, grants, IAM, browser paths or
worker lifecycle stops the application; serving does not repair prerequisites.

After normal login and enterprise selection, intended entries are
`/workbench/data/market`, `/workbench/data/api`, and `/workbench/data/mine`.
The first two navigation entries require `LISTINGKIT_DATA_SERVICES_ENABLED=true`;
My Data requires its separate Collection capability. Platform specialists use
`/workbench/admin/data-customization` under the verified platform identity.
The native descriptor registrar mounts Console, Specialist and DataKey routes
with their original, distinct authentication policies. The TLS proxy forwards
only `/data-api/v1/amazon/jobs` and its descendants to the DataKey handler.

The runtime registers the existing `data-services-v1` worker, starts it before
HTTP serving and stops it before closing its owned Temporal client and Product
pool. Temporal uses this project's persistent volume. Original Product and Store
Resource consumers remain registered alongside Amazon; the existing recovery
loop owns settlement recovery.

```powershell
docker compose --env-file .env -f docker-compose.yml -f docker-compose.data-services.yml stop
docker compose --env-file .env -f docker-compose.yml -f docker-compose.data-services.yml up -d --no-build --wait
```

Keep the same project, environment, source and overlays on every restart. Stop
does not delete data. Destruction, real-provider calls, merge, production rollout
and user acceptance need their separate applicable authority/evidence.

Installer/PostgreSQL, native policy/descriptor and lifecycle checks, plus UI and
Linux image builds, are developer evidence. Normal IAM login, actual worker
recovery, Amazon requests and user business acceptance remain **NOT_RUN** for
each candidate until those specific operations are actually executed.
