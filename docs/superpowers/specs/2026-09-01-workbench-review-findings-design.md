# Workbench Review Findings Design

## Goal

Resolve the six additional PR#270 review findings without weakening organization isolation, idempotency, or destructive-operation concurrency guarantees.

## Scope and architecture

The changes stay inside the existing Workbench BFF, Workbench context provider, Store Center service, Store quota ledger, and local ZITADEL provisioner boundaries. No new service or dependency is introduced.

The quota ledger will durably bind a reservation to the normalized immutable create payload. Store deletion will continue to use the server-owned delete operation key and will expose a narrowly gated recovery path for a client that lost that key before the first durable deleting-phase audit was written. Existing concurrent different-key conflicts remain semantic conflicts.

## Design

### 1. Separate context-read and context-switch response contracts

Add a `context-switch` response contract alongside the existing context contract. Both contracts validate the same context success and error schemas. Successful context reads and successful switches update the effective-organization cookie. Only `context-get` error responses may clear the cookie for revoked or denied current selections; a denied switch must preserve the prior cookie.

### 2. Bind quota reservations to the create request

Add a request fingerprint to `StoreQuotaReserveInput`, `StoreQuotaAllocation`, and the durable quota allocation row. The Store Center computes the fingerprint after request normalization from organization ID, platform, region, external store ID, and name using the repository's length-prefixed SHA-256 helper. `Reserve` stores it on a new allocation and returns it on replay. A same-key replay with a different fingerprint, or a legacy allocation without a fingerprint, fails closed with an identity error and never creates a Store from the changed request.

The new column is migrated through the existing Workbench `AutoMigrateStoreQuotaLedger` boundary with a safe empty default for existing rows; new reservations always write a non-empty fingerprint. Schema and replay tests verify the column and mismatch behavior.

### 3. Recover a deletion after the marked-deleting audit boundary

When a Store is already `deleting`, the service normally requires the persisted delete operation key. If the caller supplies a new key with the current Store version, the service may recover only when the persisted operation has its valid `delete_started` audit but does not yet have its `store_marked_deleting` audit. Recovery then substitutes the persisted operation key for all remaining audit and quota transitions. A stale request or a request that already has its own competing delete intent continues to return the existing lifecycle/version conflict.

This preserves the winner of a concurrent delete race while allowing a fresh UI request to finish a deletion whose client-held key was lost after the versioned Store save.

### 4. Use provider-generated organization IDs

Change the local provisioner organization-create request to send only the supported `name` field. Make `ensureAcceptanceOrganization` return the actual organization ID found or created. `ProvisionLocalMultiOrganizationAcceptance` stores that returned ID in its name-to-ID map before creating grants, authorizations, and verification records. On retries, an exact owned name match is accepted even when the provider-generated ID differs from the configured preferred ID. The returned ID remains the source of truth for all subsequent calls.

### 5. Accept a single loopback issuer root slash

Update local issuer validation to accept an empty URL path and exactly `/`, while continuing to reject non-root paths, query strings, fragments, credentials, non-loopback hosts, and ambiguous URL forms.

### 6. Make refreshed context authoritative

When the context provider retries, clear `switchedContext` and re-enable the context query before refetching. The next successful query result then becomes the provider's source of truth rather than remaining shadowed by the old successful switch result.

## Error handling and compatibility

- Failed switch responses preserve the current selection cookie.
- Invalid or missing reservation fingerprints fail closed and are surfaced as dependency/identity failures; they are never used to bind a new payload retroactively.
- Delete recovery uses only a currently persisted operation and the current Store version; it cannot steal an active delete with a stale version or competing durable intent.
- Provider IDs returned by ZITADEL are used consistently for grants, authorizations, read-back, and result persistence.

## Verification

Add failing tests before implementation for:

- denied switch responses preserving the selection cookie;
- quota same-key replay rejecting a changed payload and persisting the fingerprint;
- deletion recovery with a new client key after `store_marked_deleting` audit failure;
- strict organization-create payloads and provider-generated IDs;
- loopback issuer URLs with `/`;
- context retry replacing stale switched context.

Run focused Go and Vitest tests for each change, then the related Go packages, frontend full test suite, TypeScript check, ESLint, and `git diff --check`.

## Non-goals

- No background job system is introduced for deletion recovery.
- No change is made to unrelated ZITADEL APIs or Store Center lifecycle transitions.
- No new frontend persistence mechanism is added for operation keys.
