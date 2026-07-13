# Product Sourcing Tenant Boundary Design

## Goal

Protect `POST /api/v1/product-sourcing/1688/listingkit/tasks` with verified
ZITADEL identity, an explicit write permission, and tenant-scoped source and
target store ownership checks before it creates a ListingKit task.

## Scope

This change is PR 0 from the Product Sourcing closeout review. It deliberately
does not add source-lineage persistence, crawling, SHEIN readiness work, or CI
alignment; those are separate, independently reviewable changes.

## Decisions

### Identity source

The API accepts no caller-controlled tenant or user identity. The handler will
derive both values exclusively from the identity established by the ZITADEL
middleware. Request-body `tenant_id` and `user_id` fields are removed from the
public request contract. Request headers supplied by a client cannot override
the authenticated identity because the middleware overwrites identity headers
before the handler reads them.

The command service will also treat a verified tenant identity in `context.Context`
as authoritative. A command whose supplied tenant differs from that identity is
rejected rather than changing the context to match caller data. This retains the
security invariant for non-HTTP callers.

### Route authorization

`product-sourcing` joins the ZITADEL-protected route modules. The create route
declares a new `product_sourcing.write` permission. The existing Casbin-backed
authorizer grants that permission to the same operational roles that may create
ListingKit work (`listingkit_operator`, `listingkit_admin`, and `platform_admin`),
while preserving the existing platform-admin user and role configuration.

### Store ownership

Both store references are `listing_store.id` values:

* `source_store_id` must be an active store owned by the authenticated tenant
  whose platform is `1688`.
* `shein_store_id` must be an active store owned by the authenticated tenant
  whose platform is `SHEIN`.

The command service receives a narrow store-lookup dependency. It parses the
authenticated tenant to the legacy numeric tenant ID, loads each store under
that tenant scope, and rejects missing, cross-tenant, deleted, or
platform-mismatched stores before preparing a source envelope or invoking the
ListingKit task creator. Store access is enforced in the service, not merely in
the HTTP handler, so future callers retain the same boundary.

## Request flow

```text
Bearer token
  -> ZITADEL introspection
  -> verified tenant/user/roles placed on request + context
  -> product_sourcing.write authorization
  -> handler constructs command from verified identity and client business data
  -> command service validates identity/context consistency
  -> tenant-scoped 1688 and SHEIN store validation
  -> existing 1688 envelope -> ListingKit task creation
```

## Errors

Missing or invalid credentials return `401`; an authenticated caller lacking
`product_sourcing.write` returns `403`. Invalid tenant identity, missing or
wrong-platform stores, and mismatched caller identity return a stable
client-error response and never invoke task creation. A store that belongs to a
different tenant is intentionally indistinguishable from a missing store.

## Acceptance tests

* The route is protected when ZITADEL is configured; missing and invalid bearer
  tokens do not reach the handler.
* A valid identity without `product_sourcing.write` receives `403`.
* Body `tenant_id`/`user_id` are not accepted, and forged identity headers do
  not change the command identity.
* A command with an identity mismatch fails before task creation.
* A valid caller can use only an owned 1688 source store and an owned SHEIN
  target store.
* Cross-tenant, missing, deleted, and platform-mismatched source or target
  stores fail before task creation.
* Existing successful 1688 task creation behavior remains intact for valid
  identity and stores.
