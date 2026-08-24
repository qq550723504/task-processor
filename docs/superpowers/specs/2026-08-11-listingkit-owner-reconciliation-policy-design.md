# ListingKit owner reconciliation policy

## Goal

Make the production identity preflight reflect the approved ownership rules
without weakening fail-closed behavior for rows that still have a plausible
user owner.

## Approved policy

1. `listing_product_import_mapping` is created by the SHEIN listing system.
   Its owner is derived only from the related store. Row-level and import-task
   creator fields are not authoritative for this table.
2. For ordinary legacy owner-scoped tables, `creator` is authoritative when it
   is present. `created_by` is considered only when `creator` is blank. A
   non-empty creator that cannot resolve to a ZITADEL subject remains blocked,
   even when `created_by` resolves.
3. A row with no usable candidate owner is system-owned. It is retained in the
   reconciliation audit report but excluded from the owner-scope preflight.
   Native ListingKit rows with no candidate source follow the same rule.
4. Legacy users are resolved through the existing migration metadata contract
   (`yudao_tenant_id` and `yudao_user_id`). The migration script currently
   imports only active, non-deleted `system_users`; referenced users that exist
   in the legacy database but lack ZITADEL metadata require an explicit
   migration recovery step. IDs absent from the legacy source remain unresolved
   and are not assigned arbitrarily.

## Design

The owner-reconciliation inventory will carry an explicit candidate policy per
table. Candidate selection is deterministic and happens before report
classification:

- `legacyCreatorFirst`: creator, then created_by only if creator is blank;
- `storeOwnerOnly`: store creator, then store created_by only if store creator
  is blank;
- `noCandidateSystemOwned`: no candidate columns; report as system-owned and
  exclude from preflight.

The identity preflight repository will use the same fixed policy to exclude
only rows proven to have no candidate source. Rows with a non-empty but
unmapped or conflicting candidate remain visible to the gate. No runtime SQL
identifiers or policy values are accepted from callers.

The migration recovery report will distinguish:

- active legacy users missing ZITADEL metadata;
- inactive/deleted legacy users referenced by owner data;
- owner values absent from `system_users`.

Only an explicitly approved migration action may create or restore a ZITADEL
identity. The reconciliation write path remains bounded, report-fingerprint
confirmed, and refuses to execute while unresolved user-owned rows remain.

## Verification

Tests will cover:

- creator wins over a different created_by subject;
- non-empty unmapped creator does not fall back to created_by;
- import mappings use related store ownership only;
- blank/no-candidate rows are reported as system-owned and excluded from the
  preflight inventory;
- unresolved, conflicting, and absent legacy mappings remain fail-closed;
- the migration recovery classification matches the existing metadata keys.

The existing preflight driver fast-failure fix remains independent: failed Jobs
must return logs immediately, while pending Jobs retain the 15-minute timeout.
