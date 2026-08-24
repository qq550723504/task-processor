# ListingKit owner write invariant design

**Status:** Proposed for review  
**Date:** 2026-08-18

## Problem

The deployment run [32123406983](https://github.com/qq550723504/task-processor/actions/runs/32123406983) stopped at the identity preflight gate with:

```text
status=blocked owner_reconciliation=unresolved rows=0 auto_rows=2500 system_owned_rows=1150115
```

`auto_rows` is not the same as unresolved rows. It means the reconciliation scanner found rows whose canonical owner can be uniquely derived, but whose persisted `owner_user_id` is empty. The gate correctly blocks them, but the product currently allows new rows to be created without an owner, so the same class of data can reappear after a one-time cleanup.

The source-level cause is distributed owner assignment. Several ListingKit repositories assign `owner_user_id` only when an HTTP request identity is present and then still execute `Create` when it is absent. At least one local task-RPC path writes directly through GORM and bypasses the repository entirely. The database column is indexed but not protected by a non-empty constraint.

## Goal and invariant

For every new or updated row in the legacy owner-scoped inventory, the following must hold:

```text
owner_user_id is a non-empty canonical subject
```

If the write path cannot resolve a canonical subject, it must return an error before inserting or updating. It must never silently create an ownerless row.

The invariant applies to these owner-scoped legacy tables:

- `listing_store`
- `listing_category`
- `listing_filter_rule`
- `listing_generation_topic_override`
- `listing_generation_topic_policy`
- `listing_operation_strategy`
- `listing_pricing_rule`
- `listing_profit_rule`
- `listing_scheduled_task_config`
- `listing_sensitive_word`
- `listing_product_import_task`
- `listing_product_import_mapping`
- `listing_product_data`

Native system-owned tables remain governed by their existing system-owned policy. They must not be forced into a fabricated user identity merely to satisfy this legacy owner invariant.

## Ownership resolution contract

Owner resolution is ordered and explicit:

1. An authenticated HTTP request may supply the canonical subject already established by the identity middleware.
2. Internal/background code must pass an explicit canonical owner in its operation input or write context.
3. A child record may inherit the owner from a trusted persisted parent only when that relationship is unambiguous and checked in the same transaction.
4. A client-provided arbitrary owner value is not trusted as an override.
5. If none of the above produces a non-empty canonical subject, the operation fails.

The existing product-import-mapping validation is the reference behavior: it resolves the owner and rejects a blank result. The implementation should centralize this contract so individual repositories cannot accidentally keep the current “set if available, then create anyway” behavior.

## Write-path changes

All owner-scoped creation and update paths will use the shared owner-resolution/validation boundary, including:

- ListingKit admin CRUD repositories for stores, categories, filter rules, generation policies/overrides, operation strategies, pricing/profit rules, scheduled-task configuration, and sensitive words.
- Product-data create and upsert paths.
- Import-task batch creation and import-mapping writes.
- The local task-RPC import-task path, which currently constructs a model and calls GORM directly.
- Activity-strategy writes, after confirming whether their “shared by all operators” semantics are actually system-owned or should carry the initiating owner. The inventory policy and persisted semantics must agree; no implicit empty owner is acceptable.

Direct GORM writes to an owner-scoped table will be removed or routed through the same validated repository/service boundary. Internal callers that currently have no request context will be updated to pass the owner explicitly from the originating task or parent record.

## Database defense

Application validation is the primary source fix; the database is the final safety net.

Migration sequencing:

1. Keep the existing one-time reconciliation/backfill as a separately authorized operational action. It is not hidden inside application startup or a deployment rerun.
2. After the historical ownerless set has been reconciled and the report is zero, add a non-empty `owner_user_id` check to each owner-scoped table.
3. Use a PostgreSQL `CHECK` constraint that treats `NULL`, whitespace-only, and empty strings as invalid. During rollout, install it as `NOT VALID` so existing historical rows do not make the migration fail; new writes are still checked. Validate it only after the controlled backfill.
4. Keep SQLite/test migrations compatible, and test the constraint behavior at the repository and migration layers.

This two-stage rollout is intentional: adding a strict constraint before the known production rows are reconciled would turn the deployment failure into a migration failure, while leaving the schema unprotected would allow regression.

## Preflight and observability

The preflight gate remains fail-closed for both unresolved rows and auto-resolvable rows. Its log and report must name the two counts separately, for example:

```text
status=blocked unresolved_rows=0 auto_rows=2500 system_owned_rows=1150115
```

This removes the current misleading `owner_reconciliation=unresolved` label and makes the next failure actionable without guessing which category caused the block. Table-level detail remains in the redacted reconciliation report; secrets and raw identities must not be logged.

## Tests and acceptance criteria

Tests will be written before implementation and must cover:

- Every owner-scoped repository family rejects a missing owner before `Create`/`Save`/upsert.
- Explicit canonical owners succeed for HTTP-independent/internal callers.
- Batch writes are atomic with respect to owner validation: a batch containing an ownerless item does not partially create ownerless data.
- The local task-RPC path cannot bypass owner validation.
- Parent-derived ownership is accepted only for the intended trusted parent relationship.
- Database checks reject `NULL`, empty, and whitespace-only owners once enabled.
- Preflight output distinguishes `unresolved_rows` from `auto_rows`, and auto-resolvable rows still block the release.
- Existing owner reconciliation `ApplyUnique` behavior remains bounded, fingerprint-confirmed, and repeatable.

Acceptance means that a newly introduced ownerless row is rejected at the application boundary and, if a future path bypasses it, rejected by the database. The current 2,500 historical rows must be handled by the separate controlled backfill before the database constraints are validated; a green CI run alone does not prove that production data has been repaired.

## Out of scope

- No production database update, historical backfill, secret change, deployment, rerun, merge, or PR publication is part of this code change.
- No new synthetic “system user” is introduced.
- No blanket `NOT NULL` change is applied to native system-owned tables.
- No weakening or bypass of the identity preflight gate is allowed.

