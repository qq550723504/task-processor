# 1688 Public Source and Optional Account Boundary Design

## Status

Approved design; implementation has not started.

## Problem

The current 1688 crawler treats `source_account_id` as an account/profile
selector, but resolves it through `listingadmin.StoreRepository` and the
`listing_store` table. `listing_store` represents target marketplace stores
(for example SHEIN and Temu), not 1688 source login accounts. The same
`StoreAccessValidator` is also used by the ListingKit handoff for both the
source account and the SHEIN target store. This couples two unrelated domains
and makes a valid public 1688 URL depend on a target-store record.

The 1688 product page is public information in the normal case. An account
may improve access to pages that require login, trigger a challenge, or omit
important fields, but it must not be a mandatory prerequisite for ordinary
source ingestion.

## Goals

- Make public 1688 crawling the default, complete path.
- Keep account-assisted crawling as an explicit, optional enhancement.
- Remove all 1688 source-account access from `listing_store` and
  `listingadmin.StoreRepository`.
- Keep `listing_store` and `shein_store_id` semantics unchanged for target
  marketplace stores.
- Preserve `source_account_id` as a compatible request field while making it
  optional.
- Keep source identity product-based; an account is an access mechanism, not a
  product identity.
- Avoid storing plaintext passwords, cookies, browser profile contents, or
  proxy credentials.

## Non-goals

- No automatic creation or migration of 1688 accounts from `listing_store`.
- No new marketplace store semantics in the source-account model.
- No automatic task creation, ListingKit submission, or SHEIN publishing.
- No replacement of the existing Playwright extractor in this change.
- No external crawler service integration in this change.

## Domain model

Introduce a dedicated source-account boundary, with a repository owned by the
source-account domain rather than `internal/listingadmin`:

```text
SourceAccount
- id
- tenant_id
- platform = 1688
- label
- profile_ref
- proxy_ref
- login_url
- status
- last_verified_at
- created_at / updated_at
```

The concrete storage name may be selected during implementation, but it must
be a dedicated source-account table and must not be `listing_store` or
`listingkit_store_profiles`. Secret-bearing values are references managed by
the runtime secret boundary; they are not stored as plaintext fields in the
source-account record.

The public API keeps `source_account_id` for compatibility:

```text
source_account_id = null or 0  => public access mode
source_account_id > 0          => account-assisted access mode
```

Internally, the crawler request and handoff use an explicit
`SourceAccessMode` plus an optional account ID so callers do not have to infer
mode from an integer sentinel.

## Fetch flow

```text
Crawl(url, source_account_id?)
        |
        +-- public fetch
        |     +-- complete product -> return product
        |     +-- login/challenge/missing required fields
        |
        +-- account id supplied?
        |     +-- yes: resolve SourceAccount and retry with its Profile
        |     +-- no: return source_public_unavailable
        ```

Rules:

1. The existing unauthenticated `Process` path remains the public fetcher.
2. A supplied account is only used after a recoverable public-access failure.
3. Invalid URLs, parser errors, and non-retryable transport failures do not
   trigger account fallback.
4. An unavailable or disabled explicit account returns
   `source_account_unavailable` or `source_account_disabled`; it never falls
   back to `listing_store`.
5. Successful results record the access mode and fallback reason as redacted
   provenance metadata, without profile paths or credentials.

## API and task contracts

The crawler endpoint accepts an optional `source_account_id`. The authenticated
tenant is still attached to every task for ownership and result isolation; it
is not derived from request JSON.

`CrawlerTask` carries the explicit source access mode and optional account ID.
For backward-compatible task payloads, an absent mode is interpreted as public
when the account ID is zero and as account-assisted when it is positive.

The ListingKit 1688 handoff changes as follows:

- Public source results omit `source_account_id` from the handoff request.
- Account-assisted results include the account ID as an access selector.
- `source_identity` remains derived from the normalized product/source facts.
- `shein_store_id` remains required for the EndToEnd handoff and is validated
  through `listing_store` with expected platform `SHEIN`.
- `source_store_id` remains rejected.

The source-account check and SHEIN store check use separate interfaces and
error domains. A source-account error must not be represented as a generic
marketplace-store error.

## Storage and migration

Add a dedicated source-account schema and repository. Do not backfill or copy
rows from `listing_store`; existing rows are target stores and have different
ownership, credentials, and lifecycle semantics. Since the current production
database has no 1688 source-account records, the migration has no data
backfill step.

Account-management API/UI work may provision independent source-account
records, but public crawling must not depend on that provisioning path.

## Release and acceptance

Release in this order:

1. Deploy the public path and the domain-boundary changes.
2. Run read-only health and authenticated source preflight.
3. Run a controlled public Crawl with an explicit
   `CREATE-1688-TASK` confirmation.
4. Provision and verify an independent source account only when account
   enhancement is needed.
5. Run an account-assisted fallback acceptance separately.

Operational metrics and redacted output distinguish `public`,
`account_assisted`, `source_public_unavailable`,
`source_account_unavailable`, and `source_account_disabled`.

## Testing

Required coverage:

- public Crawl succeeds without `source_account_id`;
- public failure invokes account fallback only when an account ID is supplied;
- source-account repository is independent from `listingadmin.StoreRepository`;
- a SHEIN `listing_store` row cannot satisfy a 1688 source-account lookup;
- tenant and disabled-account boundaries are enforced;
- public ListingKit handoff omits the account selector;
- account-assisted handoff includes it without changing source identity;
- SHEIN target-store validation remains unchanged;
- migration and repository tests cover tenant isolation;
- existing Pester, Go, and Kubernetes read-only acceptance gates remain green.

## Explicit safety boundary

The implementation must never solve a missing 1688 source account by inserting
an artificial 1688 row into `listing_store`. If public access fails and no
independent account is available, the correct result is an explicit source
availability error, not a target-store lookup or silent fallback.
