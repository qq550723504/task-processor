# External Client Boundary Inventory

## Goal

This inventory records the current direct coupling between business-facing
packages and concrete external client adapters under `internal/infra/clients`.
It is not a migration plan and should not trigger broad rewrites by itself.

The goal is to make future cleanup predictable: new code should prefer local
interfaces, while existing direct dependencies should be reduced in narrow
slices when the owning domain boundary is clear.

## Client Families

Current concrete client families that frequently cross domain boundaries:

- `internal/infra/clients/management`
- `internal/infra/clients/openai`
- `internal/infra/clients/nanobanana`

These packages are allowed infrastructure adapters. The architectural risk is
not their existence. The risk is business packages importing adapter types as
part of their domain-facing contracts.

## Hotspots

Current direct dependency hotspots are:

- `internal/listingkit`
  - broad `openai` coupling across facade, studio, task, and settings code
  - should shrink behind ListingKit-owned AI interfaces before new features add
    more concrete OpenAI types
  - `internal/listingkit` root OpenAI seams are guarded by
    `TestListingKitRootOpenAIImportsStayAllowlisted`
  - `internal/listingkit/httpapi` AI runtime/bootstrap seams are guarded by
    `TestListingKitHTTPAPIExternalClientImportsStayAllowlisted`
- `internal/publishing/shein`
  - concentrated `openai` coupling in attribute, category, content, and listing
    copy helpers
  - future publishing rules should depend on local inference interfaces rather
    than concrete adapter packages
  - current direct OpenAI seams are guarded by
    `TestPublishingSheinOpenAIImportsStayAllowlisted`
- `internal/shein`
  - broad `management` coupling across inventory, scheduler, publish,
    validation, activity, mapping, and product packages
  - cleanup should follow marketplace slices, not one large adapter rewrite
- `internal/temu`
  - broad `management` coupling in sync, pricing, product, store, and scheduler
    paths, plus `openai` coupling in AI, image, SKU, product, and pipeline helpers
  - `internal/temu/sync` and `internal/temu/pricing` management seams are guarded by
    `TestTEMUSyncAndPricingManagementImportsStayAllowlisted`
  - `internal/temu/product`, `internal/temu/store`, and `internal/temu/scheduler`
    management seams are guarded by
    `TestTEMUProductStoreAndSchedulerManagementImportsStayAllowlisted`
  - TEMU OpenAI seams are guarded by `TestTEMUOpenAIImportsStayAllowlisted`
  - cleanup should begin at service constructors and package-local contracts
- `internal/app`
  - expected to construct concrete clients, but should avoid leaking concrete
    client types into business-facing contracts when a narrower port is enough
  - ProductImage model/default provider assembly seams in `internal/app/httpapi`
    are guarded by
    `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
- `internal/productimage`
  - uses `openai` and `nanobanana` as provider adapters
  - provider-facing interfaces should stay in the product image domain, with
    concrete adapter construction kept in HTTP/runtime assembly
  - current direct adapter seams are guarded by
    `TestProductImageExternalClientImportsStayAllowlisted`

## Local Interface Rule

When adding new behavior that needs a remote service:

1. Put the required capability behind a package-local interface first.
2. Keep concrete adapter construction in app, HTTP, runtime, or narrow
   bootstrap code.
3. Do not add concrete `internal/infra/clients/*` types to public domain
   contracts unless the package is explicitly an adapter boundary.
4. If an existing direct dependency remains, treat it as a migration hotspot,
   not as precedent for new code.

## Next Slice Candidates

Good next slices are small seams where a local interface can replace concrete
adapter types without changing business behavior:

1. ListingKit AI settings and studio media generation seams currently importing
   `internal/infra/clients/openai`. The HTTPAPI runtime/bootstrap seam is now
   explicitly allowlisted; root facade OpenAI seams are also explicitly
   allowlisted. The next ListingKit cleanup should move one service-facing
   capability at a time behind ListingKit-owned interfaces before adding new
   concrete OpenAI adapter call sites.
2. SHEIN publishing attribute/category inference helpers currently importing
   `internal/infra/clients/openai`. Current imports are explicitly allowlisted;
   reduce them by introducing package-local inference interfaces before adding
   new concrete OpenAI adapter call sites.
3. TEMU sync and pricing services currently importing
   `internal/infra/clients/management`. Current sync/pricing imports are explicitly
   allowlisted; future TEMU cleanup should move service-facing management calls
   behind package-local interfaces before adding new concrete adapter call sites.
   Product, store, and scheduler management imports are also explicitly allowlisted;
   keep future changes behind local interfaces or narrow adapter seams.
4. TEMU AI, image, SKU, product, and pipeline helpers currently importing
   `internal/infra/clients/openai`. Current imports are explicitly allowlisted;
   future feature work should introduce local AI/rewrite/mapping interfaces rather
   than adding new concrete OpenAI adapter call sites.
5. Product image provider construction currently importing `openai` and
   `nanobanana` outside the narrow runtime builder path. Current imports are
   explicitly allowlisted in both productimage-owned and app/httpapi ProductImage
   assembly seams; do not add new concrete adapter imports without a local
   interface seam or a documented runtime-builder exception.

## Non-goals

- Do not move every external client import in one refactor.
- Do not create generic global client interfaces that every domain depends on.
- Do not hide useful provider-specific behavior behind an interface that is too
  vague to test.
- Do not add import guards until the current hotspots have either narrow
  allowlists or local interface seams.

## Review Questions

When reviewing a change that touches external clients, ask:

1. Is this package constructing an adapter, or expressing a domain capability?
2. Would a local interface make the call site easier to test?
3. Is a concrete `management`, `openai`, or `nanobanana` type leaking into a
   public domain contract?
4. Does the change reduce coupling in one hotspot, or create a new hotspot?
5. Is any remaining direct dependency documented as a temporary exception?
