# PAY-012 Tenant Store Ownership Design

**Status:** approved design for implementation planning

## Goal

Prevent a customer from selecting or executing against another tenant's SHEIN
or 1688 store. Every ListingKit task creation and external-store action must
validate the current authenticated tenant's access to the relevant store before
it persists work, reads credentials or cached configuration, or calls a remote
platform.

## Scope

- Target SHEIN stores selected during ListingKit task creation.
- Source 1688 and target SHEIN stores used by the 1688 source handoff.
- SHEIN draft, publish, sync, retry, and other runtime actions that resolve a
  task's request or store-resolution snapshot.
- Stable server error codes and focused regression tests for cross-tenant,
  disabled, wrong-platform, and stale-snapshot store references.

This task reuses the existing `listingadmin.Store` ownership data and ListingKit
store-profile/snapshot model. It does not introduce a second store model,
database migration, entitlement change, token-refresh redesign, or a
platform-admin cross-tenant workflow.

## Current-state findings

- The 1688 handoff already checks a source and target store through a narrow
  `GetStore(ctx, tenantID, storeID)` dependency. It must be brought under the
  shared failure contract rather than remain a one-off check.
- ListingKit submission resolves a task's explicit `SheinStoreID` or persisted
  snapshot before building a SHEIN client. A manually selected store can be
  represented as enabled even when no tenant-scoped profile matches it, and the
  snapshot path currently skips a fresh ownership check.
- The submission store catalog receives a tenant ID, but PAY-012 requires a
  defensive equality check on the returned record as well as the query scope.

## Decision

Use one tenant-scoped store-access validator at two mandatory boundaries:

1. **Creation boundary.** Before a task is persisted, validate every explicit
   SHEIN target store. The 1688 handoff validates both its 1688 source store and
   SHEIN target store through the same contract.
2. **Execution boundary.** Before draft, publish, sync, retry, or any other
   remote SHEIN action can create an API client, revalidate the task's resolved
   store against the currently authenticated tenant. A persisted snapshot is a
   selection record, not a permanent authorization grant.

The validator accepts a trusted tenant ID, a store ID, and an expected platform.
It verifies that the store exists within the tenant scope, that the returned
record still has the same tenant, that its platform matches, and that it is
enabled. Callers receive a validated store record; they must not re-query an
unscoped store or reconstruct credentials from the unvalidated ID.

## Request and execution flow

```text
authenticated tenant
  -> requested source/target store ID
  -> tenant-scoped store-access validator
  -> task creation + store snapshot

persisted task / store snapshot
  -> authenticated tenant
  -> tenant-scoped store-access validator
  -> profile, pricing/cache, SHEIN API client
  -> remote draft, publish, sync, or retry
```

For an ordinary ListingKit task, an explicit SHEIN store must validate before
the task is created. For a task without a SHEIN platform/store, no unrelated
store lookup is added. The default-store path resolves a candidate only from
the authenticated tenant's configured profiles and validates it before use.

For an 1688 handoff, both `SourceStoreID` and `SheinStoreID` are required and
must validate for the authenticated tenant as platforms `1688` and `SHEIN`
respectively. The command remains fail-closed if the store dependency is absent.

At execution time, a task that was valid when created is stopped if the store
has since been disabled, reassigned, removed, or changed to an incompatible
platform. No API client, cookie refresh, pricing lookup, resolution-cache read,
or remote call is made after that failure.

## Failure semantics

Public errors do not disclose whether another tenant's store exists.

| Condition | HTTP/result code | User action | Side effect |
| --- | --- | --- | --- |
| Store absent, foreign, or wrong platform | `listingkit_store_unavailable` | Select an available store or contact support | No task write or remote action |
| Store is disabled | `listingkit_store_disabled` | Re-enable the store or select another one | No task write or remote action |
| Task snapshot no longer passes validation | `listingkit_store_snapshot_stale` | Re-select and reconfirm the store | No client/cache/remote action |
| Store credentials or required platform permission unavailable after ownership validation | Existing stable platform-readiness error | Repair store connection and retry | No remote submission |

Server-side errors preserve enough structured context for logs and support
without recording tokens, cookies, or secret fields. The HTTP boundary maps
store-access failures to stable client errors and leaves unrelated internal
failures as internal errors.

## Testing and acceptance evidence

Focused tests will prove:

1. Tenant A cannot create a ListingKit task with tenant B's SHEIN store, and
   the task repository receives no write.
2. Tenant A cannot create a 1688 handoff with tenant B's source or target
   store; wrong-platform and disabled stores use the same fail-closed path.
3. A task whose store was reassigned or disabled after creation cannot reach a
   SHEIN API client, pricing/cache lookup, draft, publish, sync, or retry.
4. A tenant-scoped store query that returns a mismatched tenant record is still
   rejected defensively.
5. Valid same-tenant stores retain existing task creation and submission
   behavior.
6. HTTP callers receive the documented stable codes and no response contains
   token, cookie, or secret data.

## Rollout and rollback

The change adds fail-closed validation only and needs no data migration. A
previously created task can be rejected on its next external action if its
store is no longer valid; that is intentional and requires explicit operator
repair rather than a compatibility bypass. Rollback is a normal code rollback,
but must not be used to bypass a detected ownership problem.

## Non-goals and follow-ups

- PAY-013 owns uploaded-object and image-cache tenant isolation.
- PAY-014 owns the broader commercial cross-tenant security suite.
- PAY-031 owns freezing the confirmed commercial submission payload, pricing,
  currency, inventory, and store snapshot for an approved submission intent.
- Platform-admin cross-tenant store management remains on its separate
  permission and audit path.
