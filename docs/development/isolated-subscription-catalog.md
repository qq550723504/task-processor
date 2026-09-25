# Isolated subscription purchase catalog

This is an explicit, one-time catalog input for the isolated #478 browser trial.
It is **not** a production price, a default free plan, an application-startup
seed, or a tenant-facing plan editor. Run it only against a fresh isolated
commercial-owner PostgreSQL database after the existing schema migration and
before any application replica serves purchase traffic. The command rejects a
database containing plans, offers, quotes, orders, subscriptions or
entitlements; it never writes an entitlement directly.

The authorized trial offer is:

| Field | Isolated-trial value |
| --- | --- |
| Offer ID | `paid-pilot-isolated-trial-v1` |
| Plan | `paid_pilot` / 付费试点（隔离试用） |
| Subscription term | 1 month |
| Offer availability | 48 hours from provisioning |
| Settlement | `ZERO_PRICE`, `CNY`, `0` minor units |
| Pricing version | `isolated-trial-v1` |

The owner plan has this **exact** module set; all listed numeric limits are
trial choices, not approved production quotas:

| Module | Trial limits |
| --- | --- |
| `store_management` | `store_count=1` |
| `rules` | no numeric limit; capability only |
| `listingkit` | `listingkit_generations_succeeded=5`, `product_image_jobs_succeeded=5`, `shein_drafts_succeeded=5`, `ai_tokens=50000` |
| `oss_storage` | `storage_bytes_current=104857600` (100 MiB) |

There is no `task_import` or `shein_publish` module/entitlement in this plan;
the old import-task routes are not part of this self-service trial. The older
`paid_pilot` product document names `listing_tasks_created` and
`studio_design_jobs_succeeded`, but those are not the current Go subscription
owner's enforced metric names; this trial does **not** claim to enforce a task
creation quota or to implement that older metric contract. The current
`listingkit_generations_succeeded` metric is the one enforced on the active
ListingKit generation path. A limit value of `0` means unlimited in the
current owner, so this trial does not use zero to disable a capability.

Use the same private commercial schema-owner JSON config shape as
`commercial-owner-schema-migrate` (`host`, `port`, `user`, `password`,
`database`, `maxConnections`, `maxIdleConnections`). Do not commit the config
or put its password on the command line. Confirm the selected database is an
isolated, empty database before invoking; its name must contain `isolated` or
`trial` as an additional accidental-target guard:

```powershell
go run ./cmd/isolated-subscription-catalog -config <private-config-absolute-path> -expected-database <isolated-database-name> -confirm ISOLATED_TRIAL_ONLY
```

The catalog is read-only while the application serves traffic, as required by
the frozen #480 contract. A failed or repeated command does not repair an
existing environment; inspect its state rather than clearing business data.
An expired 48-hour offer is unavailable; this command intentionally cannot
overwrite it or create a second free offer in the same database. Retire the
isolated instance under the trial's own data-retention procedure, never by
running this command against another environment.

After provisioning, start the isolated current application and Console, log in
as `listingkit_admin` for a verified Effective Organization with no active
subscription, then use 套餐与权益 → 套餐方案. Confirm the server quote, submit the
canonical order, and read back 我的权益, 资源与额度, and 账单与订单. The code and
PostgreSQL owner tests do not substitute for that browser acceptance.
