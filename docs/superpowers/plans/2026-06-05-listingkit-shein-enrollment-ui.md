# ListingKit SHEIN Enrollment UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first usable ListingKit frontend workflow for SHEIN product sync, cost maintenance, candidate review, and manual activity enrollment.

**Architecture:** Reuse the existing Next.js App Router + client-component pattern already used by `listingkit-ui`. Keep backend integration thin by adding one focused SHEIN enrollment API module, one focused TanStack Query hook module, and a dedicated `shein-enrollment` component area that renders a multi-store dashboard plus a single-store workbench with tab-driven views.

**Tech Stack:** Next.js App Router, React 19, TypeScript, TanStack Query, shadcn/ui table/form primitives, Vitest + Testing Library.

---

## File Structure

### New files

- `web/listingkit-ui/src/app/listing-kits/shein-enrollment/page.tsx`
  Dashboard route entry for the multi-store overview.
- `web/listingkit-ui/src/app/listing-kits/shein-enrollment/[storeId]/page.tsx`
  Single-store workbench route entry driven by `searchParams.tab`.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.tsx`
  Dashboard page shell and store overview grid.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.test.tsx`
  Dashboard rendering and navigation tests.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx`
  Single-store shell that wires header, tab state, and view switching.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`
  Workbench rendering and tab switching tests.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-header.tsx`
  Store summary strip and primary actions.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx`
  “同步商品” tab table.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-cost-price-table.tsx`
  “成本价维护” tab table.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.tsx`
  “候选池” tab table, selection, review actions.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-runs-table.tsx`
  “报名记录” tab table.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.ts`
  Shared view-model helpers for tab parsing, count formatting, and status labels.
- `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.test.ts`
  Pure model tests.
- `web/listingkit-ui/src/lib/api/shein-enrollment.ts`
  HTTP client functions for dashboard, products, costs, candidates, and enrollments.
- `web/listingkit-ui/src/lib/api/shein-enrollment.test.ts`
  API request contract tests.
- `web/listingkit-ui/src/lib/query/use-shein-enrollment.ts`
  TanStack Query hooks and mutations.
- `web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx`
  Query invalidation and mutation tests.
- `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts`
  Shared frontend types for summary cards, products, candidates, and runs.

### Modified files

- `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
  Add the new `SHEIN 活动报名` navigation entry under operations.
- `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`
  Cover the new nav entry and active route behavior.
- `web/listingkit-ui/src/lib/query/keys.ts`
  Add stable query keys for the new dashboard, store summary, products, candidates, and runs.
- `web/listingkit-ui/src/lib/types/listingkit.ts`
  Re-export the new SHEIN enrollment types.

### Existing files to reference while implementing

- `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
- `web/listingkit-ui/src/components/listingkit/shared/listingkit-page-shell.tsx`
- `web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.tsx`
- `web/listingkit-ui/src/components/listingkit/stores/tenant-store-directory-panel.test.tsx`
- `web/listingkit-ui/src/lib/api/tenant-stores.ts`
- `web/listingkit-ui/src/lib/query/use-store-profiles.ts`

## Task 1: Add typed SHEIN enrollment API surface

**Files:**
- Create: `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-enrollment.ts`
- Create: `web/listingkit-ui/src/lib/api/shein-enrollment.test.ts`
- Modify: `web/listingkit-ui/src/lib/types/listingkit.ts`

- [ ] **Step 1: Write the failing API contract test**

```ts
import { describe, expect, it, vi } from "vitest";

import {
  getSheinEnrollmentDashboard,
  getSheinEnrollmentStoreSummary,
  triggerSheinStoreSync,
  updateSheinProductCost,
} from "@/lib/api/shein-enrollment";

const mocks = vi.hoisted(() => ({
  apiRequest: vi.fn(),
}));

vi.mock("@/lib/api/client", () => ({
  apiRequest: (...args: unknown[]) => mocks.apiRequest(...args),
}));

describe("shein-enrollment api", () => {
  it("requests dashboard and store actions through the listingkit sync endpoints", async () => {
    mocks.apiRequest.mockResolvedValue({});

    await getSheinEnrollmentDashboard();
    await getSheinEnrollmentStoreSummary(12);
    await triggerSheinStoreSync(12);
    await updateSheinProductCost(88, { manualCostPrice: 19.5 });

    expect(mocks.apiRequest).toHaveBeenNthCalledWith(
      1,
      "/listing-kits/shein-sync/dashboard",
    );
    expect(mocks.apiRequest).toHaveBeenNthCalledWith(
      2,
      "/listing-kits/shein-sync/stores/12/summary",
    );
    expect(mocks.apiRequest).toHaveBeenNthCalledWith(
      3,
      "/listing-kits/shein-sync/stores/12/sync",
      { method: "POST" },
    );
    expect(mocks.apiRequest).toHaveBeenNthCalledWith(
      4,
      "/listing-kits/shein-sync/products/88/cost",
      {
        method: "PATCH",
        body: { manualCostPrice: 19.5 },
      },
    );
  });
});
```

- [ ] **Step 2: Run the API test to verify it fails**

Run: `npm test -- --run web/listingkit-ui/src/lib/api/shein-enrollment.test.ts`

Expected: FAIL with `Cannot find module '@/lib/api/shein-enrollment'` or missing export errors.

- [ ] **Step 3: Add minimal types and API client implementation**

```ts
// web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts
export type SheinEnrollmentStoreSummary = {
  storeId: number;
  storeName: string;
  storeUsername: string;
  lastSyncAt: string | null;
  syncedProductCount: number;
  missingCostCount: number;
  pendingCandidateCount: number;
  enrollableCandidateCount: number;
  lastEnrollmentStatus: "idle" | "success" | "partial" | "failed";
};

export type SheinEnrollmentDashboard = {
  stores: SheinEnrollmentStoreSummary[];
};

export type SheinProductCostInput = {
  manualCostPrice: number | null;
};
```

```ts
// web/listingkit-ui/src/lib/api/shein-enrollment.ts
import { apiRequest } from "@/lib/api/client";
import type {
  SheinEnrollmentDashboard,
  SheinEnrollmentStoreSummary,
  SheinProductCostInput,
} from "@/lib/types/listingkit/shein-enrollment";

export async function getSheinEnrollmentDashboard(): Promise<SheinEnrollmentDashboard> {
  return apiRequest<SheinEnrollmentDashboard>("/listing-kits/shein-sync/dashboard");
}

export async function getSheinEnrollmentStoreSummary(
  storeId: number,
): Promise<SheinEnrollmentStoreSummary> {
  return apiRequest<SheinEnrollmentStoreSummary>(
    `/listing-kits/shein-sync/stores/${storeId}/summary`,
  );
}

export async function triggerSheinStoreSync(storeId: number): Promise<void> {
  await apiRequest(`/listing-kits/shein-sync/stores/${storeId}/sync`, {
    method: "POST",
  });
}

export async function updateSheinProductCost(
  productId: number,
  input: SheinProductCostInput,
): Promise<void> {
  await apiRequest(`/listing-kits/shein-sync/products/${productId}/cost`, {
    method: "PATCH",
    body: input,
  });
}
```

```ts
// web/listingkit-ui/src/lib/types/listingkit.ts
export * from "./listingkit/shein-enrollment";
```

- [ ] **Step 4: Expand the API module to cover the full first-version UI data set**

```ts
export type SheinSyncedProductQuery = {
  keyword?: string;
  page: number;
  pageSize: number;
};

export type SheinCandidateReviewInput = {
  decision: "approved" | "rejected";
};

export async function getSheinSyncedProducts(
  storeId: number,
  query: SheinSyncedProductQuery,
) {
  return apiRequest(
    `/listing-kits/shein-sync/stores/${storeId}/products`,
    { query },
  );
}

export async function refreshSheinCandidates(storeId: number): Promise<void> {
  await apiRequest(`/listing-kits/shein-sync/stores/${storeId}/candidates/refresh`, {
    method: "POST",
  });
}

export async function reviewSheinCandidate(
  candidateId: number,
  input: SheinCandidateReviewInput,
): Promise<void> {
  await apiRequest(`/listing-kits/shein-sync/candidates/${candidateId}/review`, {
    method: "PATCH",
    body: input,
  });
}

export async function createSheinEnrollmentRun(
  storeId: number,
  input: { activityKey: string; candidateIds: number[] },
): Promise<void> {
  await apiRequest(`/listing-kits/shein-sync/stores/${storeId}/enrollments`, {
    method: "POST",
    body: input,
  });
}
```

- [ ] **Step 5: Run API tests to verify they pass**

Run: `npm test -- --run web/listingkit-ui/src/lib/api/shein-enrollment.test.ts`

Expected: PASS with 1 test file green.

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts web/listingkit-ui/src/lib/types/listingkit.ts web/listingkit-ui/src/lib/api/shein-enrollment.ts web/listingkit-ui/src/lib/api/shein-enrollment.test.ts
git commit -m "feat: add shein enrollment ui api client"
```

## Task 2: Add query keys and TanStack Query hooks

**Files:**
- Create: `web/listingkit-ui/src/lib/query/use-shein-enrollment.ts`
- Create: `web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx`
- Modify: `web/listingkit-ui/src/lib/query/keys.ts`

- [ ] **Step 1: Write the failing query hook test**

```tsx
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  useRefreshSheinCandidates,
  useSheinEnrollmentDashboard,
} from "@/lib/query/use-shein-enrollment";

const mocks = vi.hoisted(() => ({
  getSheinEnrollmentDashboard: vi.fn(),
  refreshSheinCandidates: vi.fn(),
}));

vi.mock("@/lib/api/shein-enrollment", () => ({
  getSheinEnrollmentDashboard: (...args: unknown[]) =>
    mocks.getSheinEnrollmentDashboard(...args),
  refreshSheinCandidates: (...args: unknown[]) =>
    mocks.refreshSheinCandidates(...args),
}));

describe("use-shein-enrollment", () => {
  it("loads the dashboard and invalidates store data after refresh", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    mocks.getSheinEnrollmentDashboard.mockResolvedValue({ stores: [] });
    mocks.refreshSheinCandidates.mockResolvedValue(undefined);
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );

    const dashboard = renderHook(() => useSheinEnrollmentDashboard(), { wrapper });
    await waitFor(() => expect(dashboard.result.current.isSuccess).toBe(true));

    const mutation = renderHook(() => useRefreshSheinCandidates(), { wrapper });
    await mutation.result.current.mutateAsync(5);

    expect(invalidateSpy).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the hook test to verify it fails**

Run: `npm test -- --run web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx`

Expected: FAIL with missing `use-shein-enrollment` module or missing query keys.

- [ ] **Step 3: Add query keys for the new workflow**

```ts
// web/listingkit-ui/src/lib/query/keys.ts
sheinEnrollmentDashboard: () => ["listingkit", "shein-enrollment", "dashboard"] as const,
sheinEnrollmentStoreSummary: (storeId: number) =>
  ["listingkit", "shein-enrollment", storeId, "summary"] as const,
sheinEnrollmentProducts: (storeId: number, query: { keyword?: string; page: number; pageSize: number }) =>
  ["listingkit", "shein-enrollment", storeId, "products", compactQueryKeyObject(query)] as const,
sheinEnrollmentCandidates: (storeId: number) =>
  ["listingkit", "shein-enrollment", storeId, "candidates"] as const,
sheinEnrollmentRuns: (storeId: number) =>
  ["listingkit", "shein-enrollment", storeId, "runs"] as const,
```

- [ ] **Step 4: Implement the hooks and invalidation rules**

```ts
// web/listingkit-ui/src/lib/query/use-shein-enrollment.ts
"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createSheinEnrollmentRun,
  getSheinEnrollmentDashboard,
  getSheinEnrollmentStoreSummary,
  getSheinSyncedProducts,
  refreshSheinCandidates,
  reviewSheinCandidate,
  triggerSheinStoreSync,
  updateSheinProductCost,
} from "@/lib/api/shein-enrollment";
import { listingKitKeys } from "@/lib/query/keys";

export function useSheinEnrollmentDashboard() {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentDashboard(),
    queryFn: getSheinEnrollmentDashboard,
  });
}

export function useSheinEnrollmentStoreSummary(storeId: number) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentStoreSummary(storeId),
    queryFn: () => getSheinEnrollmentStoreSummary(storeId),
    enabled: Number.isFinite(storeId),
  });
}

export function useTriggerSheinStoreSync() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (storeId: number) => triggerSheinStoreSync(storeId),
    onSuccess: async (_, storeId) => {
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentDashboard() });
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentStoreSummary(storeId) });
      await client.invalidateQueries({ queryKey: ["listingkit", "shein-enrollment", storeId] });
    },
  });
}

export function useRefreshSheinCandidates() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (storeId: number) => refreshSheinCandidates(storeId),
    onSuccess: async (_, storeId) => {
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentStoreSummary(storeId) });
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentCandidates(storeId) });
    },
  });
}
```

- [ ] **Step 5: Run hook tests to verify they pass**

Run: `npm test -- --run web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx`

Expected: PASS with query invalidation assertions green.

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/lib/query/keys.ts web/listingkit-ui/src/lib/query/use-shein-enrollment.ts web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx
git commit -m "feat: add shein enrollment query hooks"
```

## Task 3: Add the navigation entry and dashboard route

**Files:**
- Create: `web/listingkit-ui/src/app/listing-kits/shein-enrollment/page.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx`

- [ ] **Step 1: Write the failing navigation and dashboard tests**

```tsx
import { render, screen } from "@testing-library/react";

import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";
import { SheinEnrollmentDashboardPage } from "@/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page";

describe("shein enrollment navigation", () => {
  it("shows the SHEIN enrollment entry in operations navigation", async () => {
    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>content</div>
      </ListingKitAppShell>,
    );

    expect(screen.queryByRole("link", { name: "SHEIN 活动报名" })).not.toBeInTheDocument();
  });
});

describe("SheinEnrollmentDashboardPage", () => {
  it("renders store-level summary cards", () => {
    render(<SheinEnrollmentDashboardPage />);
    expect(screen.getByText("SHEIN 活动报名")).toBeInTheDocument();
    expect(screen.getByText("按店铺查看同步、成本价和候选报名状态。")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the dashboard tests to verify they fail**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.test.tsx`

Expected: FAIL because the new link and page component do not exist yet.

- [ ] **Step 3: Add the new navigation item**

```tsx
// web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx
{
  label: "SHEIN 活动报名",
  href: "/listing-kits/shein-enrollment",
  icon: ShoppingBag,
  match: "prefix",
  requiredRoles: MENU_ROLES.operator,
},
```

Place this item in the `店铺运营` section next to `我的店铺配置`, because this workflow is store-operations oriented and should sit near store management, not in the generic primary task flow.

- [ ] **Step 4: Implement the dashboard route and page**

```tsx
// web/listingkit-ui/src/app/listing-kits/shein-enrollment/page.tsx
import { Suspense } from "react";

import { SheinEnrollmentDashboardPage } from "@/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page";

export default function SheinEnrollmentDashboardRoute() {
  return (
    <Suspense>
      <SheinEnrollmentDashboardPage />
    </Suspense>
  );
}
```

```tsx
// web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.tsx
"use client";

import Link from "next/link";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useSheinEnrollmentDashboard } from "@/lib/query/use-shein-enrollment";

export function SheinEnrollmentDashboardPage() {
  const dashboard = useSheinEnrollmentDashboard();
  const stores = dashboard.data?.stores ?? [];

  return (
    <ListingKitPageShell backgroundClassName="overflow-hidden rounded-lg bg-[linear-gradient(180deg,#f7f4ea_0%,#ffffff_100%)]" contentClassName="gap-6 px-4 py-4 sm:px-6 sm:py-6">
      <section className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
          SHEIN ENROLLMENT
        </p>
        <h1 className="text-3xl font-semibold tracking-tight text-zinc-950">
          SHEIN 活动报名
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-zinc-600">
          按店铺查看同步、成本价和候选报名状态。
        </p>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {stores.map((store) => (
          <article key={store.storeId} className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-zinc-950">{store.storeName}</h2>
                <p className="text-sm text-zinc-500">{store.storeUsername}</p>
              </div>
              <Badge variant="neutral">待审核 {store.pendingCandidateCount}</Badge>
            </div>
            <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
              <div><dt className="text-zinc-500">已同步</dt><dd className="font-medium text-zinc-950">{store.syncedProductCount}</dd></div>
              <div><dt className="text-zinc-500">缺成本</dt><dd className="font-medium text-zinc-950">{store.missingCostCount}</dd></div>
              <div><dt className="text-zinc-500">可报名</dt><dd className="font-medium text-zinc-950">{store.enrollableCandidateCount}</dd></div>
              <div><dt className="text-zinc-500">最近同步</dt><dd className="font-medium text-zinc-950">{store.lastSyncAt ?? "-"}</dd></div>
            </dl>
            <Button asChild className="mt-5 w-full">
              <Link href={`/listing-kits/shein-enrollment/${store.storeId}`}>进入店铺工作台</Link>
            </Button>
          </article>
        ))}
      </section>
    </ListingKitPageShell>
  );
}
```

- [ ] **Step 5: Run dashboard tests to verify they pass**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.test.tsx`

Expected: PASS and the nav test should now find `SHEIN 活动报名`.

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/app/listing-kits/shein-enrollment/page.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.test.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx
git commit -m "feat: add shein enrollment dashboard route"
```

## Task 4: Build the single-store workbench shell and tab routing

**Files:**
- Create: `web/listingkit-ui/src/app/listing-kits/shein-enrollment/[storeId]/page.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.ts`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.test.ts`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-header.tsx`

- [ ] **Step 1: Write the failing workbench model test**

```ts
import { describe, expect, it } from "vitest";

import { parseSheinEnrollmentTab } from "@/components/listingkit/shein-enrollment/shein-enrollment-model";

describe("parseSheinEnrollmentTab", () => {
  it("defaults unknown tabs to candidates", () => {
    expect(parseSheinEnrollmentTab(undefined)).toBe("candidates");
    expect(parseSheinEnrollmentTab("bogus")).toBe("candidates");
    expect(parseSheinEnrollmentTab("products")).toBe("products");
  });
});
```

- [ ] **Step 2: Run the workbench model test to verify it fails**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.test.ts`

Expected: FAIL with missing module error.

- [ ] **Step 3: Implement the tab model helper**

```ts
// web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.ts
export const SHEIN_ENROLLMENT_TABS = [
  "products",
  "costs",
  "candidates",
  "runs",
] as const;

export type SheinEnrollmentTab = (typeof SHEIN_ENROLLMENT_TABS)[number];

export function parseSheinEnrollmentTab(value: string | undefined): SheinEnrollmentTab {
  if (value === "products" || value === "costs" || value === "candidates" || value === "runs") {
    return value;
  }
  return "candidates";
}
```

- [ ] **Step 4: Add the store route and workbench shell**

```tsx
// web/listingkit-ui/src/app/listing-kits/shein-enrollment/[storeId]/page.tsx
import { Suspense } from "react";

import { SheinEnrollmentStoreWorkbench } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-workbench";

export default async function SheinEnrollmentStoreRoute({
  params,
  searchParams,
}: {
  params: Promise<{ storeId: string }>;
  searchParams: Promise<{ tab?: string }>;
}) {
  const [{ storeId }, { tab }] = await Promise.all([params, searchParams]);

  return (
    <Suspense>
      <SheinEnrollmentStoreWorkbench storeId={Number(storeId)} initialTab={tab} />
    </Suspense>
  );
}
```

```tsx
// web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx
"use client";

import Link from "next/link";

import {
  parseSheinEnrollmentTab,
  SHEIN_ENROLLMENT_TABS,
} from "@/components/listingkit/shein-enrollment/shein-enrollment-model";
import { SheinEnrollmentStoreHeader } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-header";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { useSheinEnrollmentStoreSummary } from "@/lib/query/use-shein-enrollment";

export function SheinEnrollmentStoreWorkbench({
  storeId,
  initialTab,
}: {
  storeId: number;
  initialTab?: string;
}) {
  const tab = parseSheinEnrollmentTab(initialTab);
  const summary = useSheinEnrollmentStoreSummary(storeId);

  return (
    <ListingKitPageShell backgroundClassName="overflow-hidden rounded-lg bg-zinc-50" contentClassName="gap-5 px-4 py-4 sm:px-6 sm:py-6">
      <SheinEnrollmentStoreHeader storeId={storeId} summary={summary.data} />
      <nav className="flex flex-wrap gap-2" aria-label="店铺工作台标签">
        {SHEIN_ENROLLMENT_TABS.map((item) => (
          <Link
            key={item}
            href={`/listing-kits/shein-enrollment/${storeId}?tab=${item}`}
            className={item === tab ? "rounded-full bg-zinc-950 px-4 py-2 text-sm text-white" : "rounded-full border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-600"}
          >
            {item === "products" ? "同步商品" : item === "costs" ? "成本价维护" : item === "candidates" ? "候选池" : "报名记录"}
          </Link>
        ))}
      </nav>
      <section data-testid="shein-enrollment-tab-panel">{tab}</section>
    </ListingKitPageShell>
  );
}
```

- [ ] **Step 5: Write and run the workbench component test**

```tsx
import { render, screen } from "@testing-library/react";

import { SheinEnrollmentStoreWorkbench } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-workbench";

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinEnrollmentStoreSummary: () => ({
    data: {
      storeId: 12,
      storeName: "SHEIN US",
      storeUsername: "shein-us",
      lastSyncAt: null,
      syncedProductCount: 18,
      missingCostCount: 4,
      pendingCandidateCount: 3,
      enrollableCandidateCount: 2,
      lastEnrollmentStatus: "idle",
    },
  }),
}));

it("defaults the workbench tab to candidates", () => {
  render(<SheinEnrollmentStoreWorkbench storeId={12} />);

  expect(screen.getByText("SHEIN US")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "候选池" })).toHaveAttribute(
    "href",
    "/listing-kits/shein-enrollment/12?tab=candidates",
  );
  expect(screen.getByTestId("shein-enrollment-tab-panel")).toHaveTextContent("candidates");
});
```

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.test.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`

Expected: PASS with both tests green.

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/app/listing-kits/shein-enrollment/[storeId]/page.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-model.test.ts web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-header.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx
git commit -m "feat: add shein enrollment store workbench shell"
```

## Task 5: Implement products and cost-maintenance tables

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-cost-price-table.tsx`
- Modify: `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/lib/query/use-shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx`

- [ ] **Step 1: Write the failing products/cost table test**

```tsx
import { render, screen } from "@testing-library/react";

import { SheinSyncedProductsTable } from "@/components/listingkit/shein-enrollment/shein-synced-products-table";

describe("SheinSyncedProductsTable", () => {
  it("renders publish status, sync time, and cost status", () => {
    render(
      <SheinSyncedProductsTable
        items={[
          {
            id: 1,
            productName: "Summer Dress",
            skcCode: "SKC001",
            shelfStatus: "ON_SHELF",
            publishTime: "2026-06-01 12:00:00",
            lastSyncAt: "2026-06-05 08:00:00",
            autoCostPrice: 12.3,
            manualCostPrice: null,
            effectiveCostPrice: 12.3,
          },
        ]}
        isLoading={false}
      />,
    );

    expect(screen.getByText("Summer Dress")).toBeInTheDocument();
    expect(screen.getByText("SKC001")).toBeInTheDocument();
    expect(screen.getByText("ON_SHELF")).toBeInTheDocument();
    expect(screen.getByText("12.3")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the table test to verify it fails**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx`

Expected: FAIL with missing component file.

- [ ] **Step 3: Extend the types and queries for products**

```ts
export type SheinSyncedProduct = {
  id: number;
  productName: string;
  skcCode: string;
  shelfStatus: string;
  publishTime: string | null;
  lastSyncAt: string | null;
  autoCostPrice: number | null;
  manualCostPrice: number | null;
  effectiveCostPrice: number | null;
};

export type SheinSyncedProductPage = {
  items: SheinSyncedProduct[];
  total: number;
  page: number;
  pageSize: number;
};
```

```ts
export function useSheinSyncedProducts(
  storeId: number,
  query: { keyword?: string; page: number; pageSize: number },
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentProducts(storeId, query),
    queryFn: () => getSheinSyncedProducts(storeId, query),
    enabled: Number.isFinite(storeId),
  });
}

export function useUpdateSheinProductCost(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ productId, manualCostPrice }: { productId: number; manualCostPrice: number | null }) =>
      updateSheinProductCost(productId, { manualCostPrice }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentStoreSummary(storeId) });
      await client.invalidateQueries({ queryKey: ["listingkit", "shein-enrollment", storeId, "products"] });
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentCandidates(storeId) });
    },
  });
}
```

- [ ] **Step 4: Implement the two tables and wire them into the workbench**

```tsx
// web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx
"use client";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { SheinSyncedProduct } from "@/lib/types/listingkit/shein-enrollment";

export function SheinSyncedProductsTable({
  items,
  isLoading,
}: {
  items: SheinSyncedProduct[];
  isLoading: boolean;
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>商品</TableHead>
          <TableHead>SKC</TableHead>
          <TableHead>上架状态</TableHead>
          <TableHead>发布时间</TableHead>
          <TableHead>同步时间</TableHead>
          <TableHead>生效成本</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {isLoading ? (
          <TableRow><TableCell colSpan={6}>加载中...</TableCell></TableRow>
        ) : items.length === 0 ? (
          <TableRow><TableCell colSpan={6}>暂无同步商品</TableCell></TableRow>
        ) : items.map((item) => (
          <TableRow key={item.id}>
            <TableCell>{item.productName}</TableCell>
            <TableCell>{item.skcCode}</TableCell>
            <TableCell>{item.shelfStatus}</TableCell>
            <TableCell>{item.publishTime ?? "-"}</TableCell>
            <TableCell>{item.lastSyncAt ?? "-"}</TableCell>
            <TableCell>{item.effectiveCostPrice ?? "-"}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
```

```tsx
// web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-cost-price-table.tsx
"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { SheinSyncedProduct } from "@/lib/types/listingkit/shein-enrollment";

export function SheinCostPriceTable({
  items,
  onSave,
}: {
  items: SheinSyncedProduct[];
  onSave: (productId: number, manualCostPrice: number | null) => Promise<void>;
}) {
  const [drafts, setDrafts] = useState<Record<number, string>>({});

  return (
    <div className="space-y-3">
      {items.map((item) => (
        <div key={item.id} className="flex items-center gap-3 rounded-xl border border-zinc-200 bg-white p-3">
          <div className="min-w-0 flex-1">
            <p className="font-medium text-zinc-950">{item.productName}</p>
            <p className="text-xs text-zinc-500">{item.skcCode}</p>
          </div>
          <Input
            aria-label={`成本价 ${item.productName}`}
            className="w-32"
            value={drafts[item.id] ?? String(item.manualCostPrice ?? item.autoCostPrice ?? "")}
            onChange={(event) => setDrafts((current) => ({ ...current, [item.id]: event.target.value }))}
          />
          <Button
            onClick={() => onSave(item.id, drafts[item.id] ? Number(drafts[item.id]) : null)}
            size="sm"
          >
            保存成本价
          </Button>
        </div>
      ))}
    </div>
  );
}
```

In `SheinEnrollmentStoreWorkbench`, render:
- `SheinSyncedProductsTable` when `tab === "products"`
- `SheinCostPriceTable` when `tab === "costs"`

- [ ] **Step 5: Run targeted tests to verify they pass**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx`

Expected: PASS after adding the new test file and updating the workbench expectations.

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-synced-products-table.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-cost-price-table.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts web/listingkit-ui/src/lib/api/shein-enrollment.ts web/listingkit-ui/src/lib/query/use-shein-enrollment.ts
git commit -m "feat: add shein enrollment product and cost tabs"
```

## Task 6: Implement candidate review and manual enrollment actions

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-runs-table.tsx`
- Modify: `web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/lib/query/use-shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx`

- [ ] **Step 1: Write the failing candidate table interaction test**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SheinCandidatesTable } from "@/components/listingkit/shein-enrollment/shein-candidates-table";

describe("SheinCandidatesTable", () => {
  it("selects candidates and submits a manual enrollment action", async () => {
    const user = userEvent.setup();
    const onEnroll = vi.fn().mockResolvedValue(undefined);

    render(
      <SheinCandidatesTable
        items={[
          {
            id: 11,
            productName: "Summer Dress",
            candidateStatus: "pending_review",
            reason: "利润满足规则",
            effectiveCostPrice: 18.2,
          },
        ]}
        onApprove={vi.fn()}
        onReject={vi.fn()}
        onEnroll={onEnroll}
      />,
    );

    await user.click(screen.getByRole("checkbox", { name: "选择 Summer Dress" }));
    await user.click(screen.getByRole("button", { name: "报名活动" }));

    expect(onEnroll).toHaveBeenCalledWith([11]);
  });
});
```

- [ ] **Step 2: Run the candidate test to verify it fails**

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.test.tsx`

Expected: FAIL because the candidate table file does not exist yet.

- [ ] **Step 3: Extend the types, API, and hooks for candidates and runs**

```ts
export type SheinActivityCandidate = {
  id: number;
  productName: string;
  candidateStatus: "pending_review" | "approved" | "rejected" | "auto_queued";
  reason: string;
  effectiveCostPrice: number | null;
};

export type SheinEnrollmentRun = {
  id: number;
  activityKey: string;
  triggerMode: "manual" | "schedule" | "automatic";
  successCount: number;
  failureCount: number;
  status: "running" | "success" | "partial" | "failed";
  createdAt: string;
};
```

```ts
export function useSheinCandidates(storeId: number) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentCandidates(storeId),
    queryFn: () => getSheinCandidates(storeId),
    enabled: Number.isFinite(storeId),
  });
}

export function useReviewSheinCandidate(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ candidateId, decision }: { candidateId: number; decision: "approved" | "rejected" }) =>
      reviewSheinCandidate(candidateId, { decision }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentStoreSummary(storeId) });
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentCandidates(storeId) });
    },
  });
}

export function useCreateSheinEnrollmentRun(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ activityKey, candidateIds }: { activityKey: string; candidateIds: number[] }) =>
      createSheinEnrollmentRun(storeId, { activityKey, candidateIds }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentStoreSummary(storeId) });
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentCandidates(storeId) });
      await client.invalidateQueries({ queryKey: listingKitKeys.sheinEnrollmentRuns(storeId) });
    },
  });
}
```

- [ ] **Step 4: Implement the candidate and run tables**

```tsx
// web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.tsx
"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { SheinActivityCandidate } from "@/lib/types/listingkit/shein-enrollment";

export function SheinCandidatesTable({
  items,
  onApprove,
  onReject,
  onEnroll,
}: {
  items: SheinActivityCandidate[];
  onApprove: (candidateId: number) => Promise<void>;
  onReject: (candidateId: number) => Promise<void>;
  onEnroll: (candidateIds: number[], activityKey: string) => Promise<void>;
}) {
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [activityKey, setActivityKey] = useState("default");
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <Input
          aria-label="活动 key"
          className="w-56"
          value={activityKey}
          onChange={(event) => setActivityKey(event.target.value)}
        />
        <Button onClick={() => onEnroll(selectedIds, activityKey)} disabled={selectedIds.length === 0}>
          报名活动
        </Button>
      </div>
      <div className="space-y-3">
        {items.map((item) => (
          <div key={item.id} className="rounded-2xl border border-zinc-200 bg-white p-4">
            <div className="flex items-start gap-3">
              <input
                aria-label={`选择 ${item.productName}`}
                checked={selectedSet.has(item.id)}
                onChange={(event) =>
                  setSelectedIds((current) =>
                    event.target.checked
                      ? [...current, item.id]
                      : current.filter((id) => id !== item.id),
                  )
                }
                type="checkbox"
              />
              <div className="min-w-0 flex-1">
                <p className="font-medium text-zinc-950">{item.productName}</p>
                <p className="text-sm text-zinc-500">{item.reason}</p>
              </div>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" onClick={() => onApprove(item.id)}>通过</Button>
                <Button size="sm" variant="outline" onClick={() => onReject(item.id)}>驳回</Button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
```

```tsx
// web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-runs-table.tsx
"use client";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { SheinEnrollmentRun } from "@/lib/types/listingkit/shein-enrollment";

export function SheinEnrollmentRunsTable({ items }: { items: SheinEnrollmentRun[] }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>活动</TableHead>
          <TableHead>触发方式</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>成功</TableHead>
          <TableHead>失败</TableHead>
          <TableHead>时间</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.length === 0 ? (
          <TableRow><TableCell colSpan={6}>暂无报名记录</TableCell></TableRow>
        ) : items.map((item) => (
          <TableRow key={item.id}>
            <TableCell>{item.activityKey}</TableCell>
            <TableCell>{item.triggerMode}</TableCell>
            <TableCell>{item.status}</TableCell>
            <TableCell>{item.successCount}</TableCell>
            <TableCell>{item.failureCount}</TableCell>
            <TableCell>{item.createdAt}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
```

- [ ] **Step 5: Wire candidate/runs views into the workbench and run the tests**

In `SheinEnrollmentStoreWorkbench`:
- render `SheinCandidatesTable` when `tab === "candidates"`
- render `SheinEnrollmentRunsTable` when `tab === "runs"`
- pass mutations from `useReviewSheinCandidate(storeId)` and `useCreateSheinEnrollmentRun(storeId)`

Run: `npm test -- --run web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx`

Expected: PASS with candidate selection and invalidation checks green.

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-candidates-table.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-runs-table.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.tsx web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts web/listingkit-ui/src/lib/api/shein-enrollment.ts web/listingkit-ui/src/lib/query/use-shein-enrollment.ts
git commit -m "feat: add shein enrollment candidate workflow"
```

## Task 7: Final verification and polish

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-enrollment/*`
- Modify: `web/listingkit-ui/src/lib/api/shein-enrollment.ts`
- Modify: `web/listingkit-ui/src/lib/query/use-shein-enrollment.ts`

- [ ] **Step 1: Add missing empty/loading/error states discovered during manual pass**

```tsx
if (dashboard.isLoading) {
  return <ListingKitPageShell ...><p>加载店铺概览中...</p></ListingKitPageShell>;
}

if (dashboard.isError) {
  return <ListingKitPageShell ...><p>店铺概览加载失败，请稍后重试。</p></ListingKitPageShell>;
}
```

Apply the same pattern to:
- store summary header
- products tab
- candidates tab
- runs tab

Do not add a new global state framework; keep the first version aligned with existing `listingkit-ui` page patterns.

- [ ] **Step 2: Run focused frontend checks**

Run:

```bash
npm test -- --run web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page.test.tsx web/listingkit-ui/src/components/listingkit/shein-enrollment/shein-enrollment-store-workbench.test.tsx web/listingkit-ui/src/lib/api/shein-enrollment.test.ts web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx
npm run typecheck
npm run lint
```

Expected:
- `vitest` PASS for all touched test files
- `tsc --noEmit` exits `0`
- `eslint` exits `0`

- [ ] **Step 3: Smoke-check the route entry points**

Run:

```bash
npm test -- --run web/listingkit-ui/src/app/listing-kits/listingkit-smoke.test.tsx
```

Expected: PASS and no route-level regression from adding the new route tree.

- [ ] **Step 4: Commit**

```bash
git add web/listingkit-ui/src/app/listing-kits/shein-enrollment web/listingkit-ui/src/components/listingkit/shein-enrollment web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.tsx web/listingkit-ui/src/components/listingkit/shared/listingkit-app-shell.test.tsx web/listingkit-ui/src/lib/api/shein-enrollment.ts web/listingkit-ui/src/lib/api/shein-enrollment.test.ts web/listingkit-ui/src/lib/query/use-shein-enrollment.ts web/listingkit-ui/src/lib/query/use-shein-enrollment.test.tsx web/listingkit-ui/src/lib/query/keys.ts web/listingkit-ui/src/lib/types/listingkit.ts web/listingkit-ui/src/lib/types/listingkit/shein-enrollment.ts
git commit -m "feat: complete shein enrollment ui workflow"
```

## Notes for execution

- Reuse the existing open-source building blocks already in the repo:
  - `@tanstack/react-query` for async state and invalidation
  - shadcn/ui table, button, input, badge primitives
  - existing `ListingKitPageShell` / `ListingKitAppShell` layout components
- Do not invent a parallel state-management layer.
- Keep first-version filtering simple:
  - products tab: keyword only
  - costs tab: use existing synced product list, then iterate toward dedicated missing-cost filter later if needed
  - candidates tab: selection + approve/reject + enroll only
- Do not expand scope into:
  - cross-store bulk enrollment
  - automatic enrollment policy editor
  - a detailed enrollment-run item drill-down page
  - CSV import for cost prices

## Self-review

- Spec coverage check:
  - Multi-store overview: covered by Task 3
  - Single-store workbench with header + tabs: covered by Task 4
  - Synced products + cost maintenance: covered by Task 5
  - Candidate review + manual enrollment + runs: covered by Task 6
  - Route/nav integration and verification: covered by Tasks 3 and 7
- Placeholder scan:
  - No `TODO` / `TBD` placeholders left in task steps.
  - Every code-writing step includes a concrete code block or explicit wiring instruction.
- Type consistency:
  - Shared names stay consistent across tasks: `SheinEnrollmentStoreSummary`, `SheinSyncedProduct`, `SheinActivityCandidate`, `SheinEnrollmentRun`, `parseSheinEnrollmentTab`, `useCreateSheinEnrollmentRun`.
