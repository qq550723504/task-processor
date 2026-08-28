# ListingKit Product Design V1 Phase 1 — Tenant Scope Addendum

> **For agentic workers:** Read this file together with `docs/superpowers/plans/2026-08-27-listingkit-product-design-v1-phase-1-ia-language.md` before implementation.

**Spec Addendum:** `docs/superpowers/specs/2026-08-27-listingkit-product-design-v1-multi-tenant-addendum.md`

## Purpose

Phase 1 remains an information-architecture and product-language slice. Multi-tenancy does not require a backend rewrite in this phase, but tenant context must be preserved as a first-class global scope while the application shell is reorganized.

## Additional Global Constraints

- ListingKit is multi-tenant; all business pages operate inside one effective tenant context.
- Preserve the current authenticated tenant display in the global shell.
- Preserve platform-admin delegated tenant switching and `返回我的租户` behavior.
- Do not convert tenant switching into a Product Center table filter.
- Do not add tenant selectors to normal operator pages.
- Do not remove or weaken current tenant/role visibility while simplifying the shell.
- Do not change the existing `tenant_id` delegated-management query behavior in Phase 1.
- Do not change tenant header propagation, backend tenant context, ZITADEL resource-owner handling, or tenant authorization logic.

## Task 1 additions — Application shell

The shell-navigation task must preserve these existing product requirements:

1. The account button continues to show the current tenant summary.
2. The account menu continues to show `当前租户`.
3. A non-platform-admin tenant administrator does not see delegated tenant controls.
4. A platform admin continues to see delegated tenant controls.
5. When `tenant_id` is active, the UI continues to expose the delegated tenant state and `返回我的租户`.
6. Navigation relabeling must not remove tenant identity from the shell.

Add or retain test coverage equivalent to:

```tsx
expect(screen.getByText("当前租户")).toBeInTheDocument();
```

For a normal tenant administrator:

```tsx
expect(screen.queryByRole("combobox", { name: "代管租户 ID" })).not.toBeInTheDocument();
```

For a platform administrator after opening the account menu:

```tsx
expect(screen.getByRole("combobox", { name: "代管租户 ID" })).toBeInTheDocument();
expect(screen.getByRole("button", { name: "切换租户" })).toBeInTheDocument();
```

Existing account-menu tests already cover much of this behavior; do not duplicate them unnecessarily. Treat those tests as regression protection while changing the shell navigation.

## Product Center and Product Detail additions

Phase 1 does not add a visible tenant column or tenant filter to Product Center.

Reason: the current tenant is the global operating scope. A normal operator should experience the Product Center as containing only their tenant's products.

The Phase 1 Product Center copy remains:

```text
商品中心
管理 ListingKit 已整理的商品资料，并查看审核与平台准备情况。
```

Do not add wording such as `全部租户商品` or `租户商品` to routine page headings.

Product Detail must not expose a tenant-edit control. Product tenant ownership is immutable through the ordinary product editing experience.

## Verification additions

During Phase 1 verification, ensure shell changes do not regress tenant context:

- normal tenant admin account menu still shows current tenant;
- platform admin tenant delegation still works at the UI level;
- current `tenant_id` query parameter behavior remains unchanged;
- no Product Center or Product Detail change introduces a cross-tenant selector;
- no backend tenant propagation file changes appear in the diff.

Add this scope check to final diff review:

```powershell
git diff origin/main...HEAD -- web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx
```

Confirm that tenant-account code changes, if any, are limited to preserving compatibility with the new navigation and do not change delegated-tenant semantics.

## Acceptance criteria additions

Phase 1 is not complete unless:

1. current tenant identity remains visible in the shell/account context;
2. ordinary tenant users remain scoped to their authenticated tenant;
3. platform-admin delegated tenant controls remain available and explicit;
4. the product-oriented navigation does not create any cross-tenant browse behavior;
5. tenant context remains orthogonal to Product/Task terminology changes.
