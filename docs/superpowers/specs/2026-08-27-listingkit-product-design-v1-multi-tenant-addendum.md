# ListingKit Product Design V1 — Multi-Tenant Addendum

**Date:** 2026-08-27  
**Status:** Approved constraint; supplements `2026-08-27-listingkit-product-design-v1.md`  
**Scope:** Tenant context, data isolation, tenant-aware navigation, and platform-admin delegation across the ListingKit authenticated product experience.

## 1. Why this addendum exists

ListingKit is a multi-tenant product. Tenant scope is therefore not an admin-only implementation detail; it is the global business context in which Products, Platform Drafts, Listings, stores, exceptions, rules, prompts, automation, usage, and subscriptions are interpreted.

The existing architecture already carries tenant identity through ZITADEL and tenant-aware backend contexts. Product Design V1 must preserve and make that boundary explicit rather than treating tenant as an incidental account field.

## 2. Tenant is the global product scope

The primary hierarchy becomes:

```text
Tenant
├── Products
│   ├── Product revisions
│   └── Platform Drafts
├── Stores
│   └── Listings
├── Exceptions
├── Rules
├── AI / Prompt configuration
├── Automation
├── Operational data
└── Subscription / quota
```

A Product, Platform Draft, Listing, store, exception, revision, task, workflow, and tenant-level configuration must always resolve inside exactly one effective tenant context.

User-facing pages must never silently combine business data from multiple tenants.

## 3. Effective tenant model

ListingKit should distinguish:

```text
Authenticated tenant
        ↓
Effective tenant
```

For normal operators/admins, these are the same tenant.

For platform administrators with delegated-management capability, the effective tenant may temporarily differ from the authenticated tenant after an explicit tenant switch.

The UI must always make the effective tenant visible when delegated management is active.

## 4. Tenant switching rules

### 4.1 Normal users

A normal tenant operator/admin works inside the tenant supplied by the authenticated ZITADEL identity. The primary product UI does not need a tenant filter because cross-tenant mixing is not a valid operator workflow.

### 4.2 Platform administrators

Platform administrators may explicitly switch the effective tenant for delegated administration.

Tenant switching is a global context change, not a local table filter. Once changed, all tenant-scoped pages and mutations must resolve against the selected effective tenant.

Recommended UX:

```text
当前租户
Acme Commerce

平台管理员：
[切换代管租户]
```

When delegated management is active, show a persistent but compact indicator such as:

```text
正在代管：Acme Commerce
```

and provide `返回我的租户`.

Do not make ordinary operators choose a tenant on every Product Center or Listing Center page.

## 5. Tenant isolation requirements by product surface

### 5.1 Workbench

All summary counts, recent work, attention items, platform state, and AI action inbox entries are scoped to the effective tenant.

The home page must never show a cross-tenant aggregate to a tenant operator.

A platform-wide dashboard, if needed later, is a separate platform-admin surface and is not the tenant Workbench.

### 5.2 Product Center

Product Center lists only Products owned by the effective tenant.

Search, filters, pagination, bulk selection, AI checks, platform generation, and deletion all remain tenant-scoped.

Bulk selection must not survive a tenant-context switch. A tenant switch should clear current product selection and tenant-specific query state.

### 5.3 Product Workspace

The Product Workspace inherits tenant identity from the Product and effective request context.

The UI must not permit changing a Product's tenant through normal editing.

Source lineage may reference external systems, but the resulting Product remains owned by one tenant.

### 5.4 Listing Center

A Listing is conceptually:

```text
Tenant + Product + Platform + Store
```

The Product and Store used to create a Listing must belong to the same effective tenant.

Store pickers must never offer stores from another tenant.

### 5.5 Exception Center

Exceptions are tenant-scoped. Aggregation may group repeated issues only within the effective tenant.

Do not aggregate one root cause across multiple tenants in the tenant-facing Exception Center, even if the technical platform error is identical.

Platform-wide incidents may be represented separately in platform-admin diagnostics.

### 5.6 Rules and AI configuration

Tenant-level product rules, pricing/profit rules, sensitive-word policies, operation strategies, prompts, and AI settings belong to the effective tenant unless explicitly defined as platform-global configuration.

The UI must clearly distinguish tenant overrides from platform defaults when both exist.

### 5.7 Subscription and quotas

Usage, plan, quotas, store limits, task limits, and entitlement checks resolve against the effective tenant.

Bulk actions should validate tenant quota before execution and present business-language quota errors.

## 6. Domain model implications

The long-term product model should make tenant ownership explicit:

```text
Tenant
  ↓
Product
  ↓
Platform Draft
  ↓
Listing
```

Recommended ownership keys conceptually:

```text
Product.tenant_id
PlatformDraft.tenant_id
Listing.tenant_id
Store.tenant_id
Exception.tenant_id
Task.tenant_id
Revision.tenant_id
```

Whether every table physically stores `tenant_id` or derives it through a parent relation is an implementation decision. The product contract is that tenant ownership is deterministic and enforceable.

Stable Product identity introduced later must be unique within an unambiguous tenant scope; global uniqueness is optional if IDs already provide it.

## 7. Query and cache rules

Tenant identity must be part of the effective query/cache scope for all tenant-dependent data.

When the effective tenant changes:

1. tenant-scoped queries must be invalidated or keyed by tenant;
2. in-progress local selections must be cleared;
3. tenant-specific drafts/forms must not leak into the new tenant context;
4. mutations started for the old tenant must not be reinterpreted under the new tenant;
5. the UI should reload or re-resolve tenant-scoped capabilities where necessary.

## 8. URL and routing rules

V1 may continue using existing routes without adding a tenant id to every path.

Tenant scope should come from authenticated/effective request context rather than requiring URLs such as:

```text
/tenants/:tenantId/products
```

A platform-admin delegated tenant may continue to use the existing explicit tenant-switch mechanism if it safely propagates to tenant-aware APIs.

The presence of a tenant query parameter must not turn ordinary tenant pages into cross-tenant browse surfaces.

## 9. Authorization rules

Tenant isolation is an authorization boundary, not only a frontend filter.

Frontend behavior must assume the backend enforces tenant ownership for reads and writes. Hiding another tenant's Product in the UI is not sufficient security.

Role and tenant checks are complementary:

```text
Can user perform action?
= role permission
+ effective tenant ownership/scope
+ resource/business constraints
```

## 10. UX rules for tenant context

1. Show current tenant identity in the global shell/account context.
2. Show delegated tenant state prominently enough to prevent accidental writes to the wrong tenant.
3. Do not add tenant columns to every table for normal tenant users; the tenant is already the global scope.
4. Do not add a tenant filter to normal Product/Listing/Exception pages.
5. Platform-admin cross-tenant search/analytics must be separate from tenant-operational views.
6. Destructive or high-impact delegated actions should make the target tenant explicit in confirmation copy.
7. A tenant switch clears selections and context-sensitive bulk state.

## 11. Testing additions

All later Product Design implementation phases must include tenant-scope coverage.

At minimum test:

1. normal operator sees current-tenant context and no delegated-tenant controls;
2. platform admin can enter and leave delegated tenant context;
3. tenant switch changes request/query scope and clears tenant-specific selections;
4. Product Center data is tenant-scoped;
5. Listing creation cannot combine Product and Store from different tenants;
6. Exception aggregation never mixes tenant data;
7. tenant-scoped AI/rule configuration remains isolated;
8. task/workflow execution retains the originating tenant context through retries/resume.

## 12. Effect on Product Design V1 phases

### Phase 1

Preserve the existing current-tenant display and delegated-tenant controls while reorganizing navigation. Do not accidentally hide or weaken tenant context.

### Phase 2

Workbench and Product Center queries, counts, bulk state, filters, and cache keys must be defined as effective-tenant scoped.

### Phase 3

Product Workspace must inherit and retain Product tenant ownership; revision/history/AI actions stay in that tenant.

### Phase 4

Listing and Exception models must explicitly include effective tenant ownership and same-tenant Product/Store validation.

### Phase 5

Stable Product identity and any backend migrations must preserve deterministic tenant ownership and enforce it server-side.

## 13. Acceptance criterion

A tenant operator should experience ListingKit as if their tenant is the entire product universe, while a platform administrator can explicitly enter another tenant's universe without ever mixing the two.