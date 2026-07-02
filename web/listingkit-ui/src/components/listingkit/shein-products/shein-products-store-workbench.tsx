"use client";

import Link from "next/link";
import { useState } from "react";

import {
  isSheinProductsTab,
  parseSheinProductsTab,
  SHEIN_PRODUCTS_TABS,
  sheinProductsTabLabel,
} from "@/components/listingkit/shein-products/shein-products-model";
import { SheinProductsStoreHeader } from "@/components/listingkit/shein-products/shein-products-store-header";
import {
  SheinCostPriceTable,
  type SheinCostPriceSaveTarget,
} from "@/components/listingkit/shein-products/shein-cost-price-table";
import { SheinSyncedProductsTable } from "@/components/listingkit/shein-products/shein-synced-products-table";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { ListingKitPagination } from "@/components/listingkit/shared/listingkit-pagination";
import {
  useSheinEnrollmentStoreSummary,
  useSheinSDSCostGroups,
  useSheinSourceSDSCostGroups,
  useSheinSyncedProducts,
  useSyncSheinSourceSDSProduct,
  useTriggerSheinStoreSync,
  useUpdateSheinSDSCostGroup,
  useUpdateSheinSyncedProductCost,
} from "@/lib/query/use-shein-enrollment";

const SHEIN_PRODUCTS_PAGE_SIZE = 100;

export function SheinProductsStoreWorkbench({
  initialTab,
  storeId,
}: {
  initialTab?: string;
  storeId: number;
}) {
  const tab = parseSheinProductsTab(initialTab);
  const [productKeyword, setProductKeyword] = useState("");
  const [productsPage, setProductsPage] = useState(1);
  const [costsPage, setCostsPage] = useState(1);
  const productsTabActive = tab === "products";
  const costsTabActive = tab === "costs";
  const summary = useSheinEnrollmentStoreSummary(storeId, {
    activity_type: "PROMOTION",
  });
  const products = useSheinSyncedProducts(
    storeId,
    {
      skc_name: productKeyword || undefined,
      page: productsPage,
      page_size: SHEIN_PRODUCTS_PAGE_SIZE,
    },
    { enabled: productsTabActive },
  );
  const costProducts = useSheinSyncedProducts(
    storeId,
    {
      page: costsPage,
      page_size: SHEIN_PRODUCTS_PAGE_SIZE,
    },
    { enabled: false },
  );
  const sdsCostGroups = useSheinSDSCostGroups(
    storeId,
    {
      page: costsPage,
      page_size: SHEIN_PRODUCTS_PAGE_SIZE,
    },
    { enabled: false },
  );
  const sourceSDSCostGroups = useSheinSourceSDSCostGroups(
    storeId,
    {
      page: costsPage,
      page_size: SHEIN_PRODUCTS_PAGE_SIZE,
    },
    { enabled: costsTabActive },
  );
  const syncMutation = useTriggerSheinStoreSync(storeId);
  const syncSourceProductMutation = useSyncSheinSourceSDSProduct(storeId);
  const updateCostMutation = useUpdateSheinSyncedProductCost(storeId);
  const updateGroupCostMutation = useUpdateSheinSDSCostGroup(storeId);

  return (
    <ListingKitPageShell backgroundClassName="overflow-hidden rounded-lg bg-zinc-50" contentClassName="gap-5 px-4 py-4 sm:px-6 sm:py-6">
      <div className="flex items-center gap-2 text-sm text-zinc-500">
        <Link className="hover:text-zinc-900" href="/listing-kits/shein-products">
          SHEIN 商品同步
        </Link>
        <span>/</span>
        <span>{summary.data?.summary?.store_name || `店铺 ${storeId}`}</span>
      </div>

      <SheinProductsStoreHeader
        onSync={() => void syncMutation.mutateAsync({ trigger_mode: "manual" })}
        summary={summary.data?.summary}
        syncError={syncMutation.error}
        syncPending={syncMutation.isPending}
      />

      <nav aria-label="商品同步工作台标签" className="flex flex-wrap gap-2">
        {SHEIN_PRODUCTS_TABS.map((item) => (
          <Link
            key={item}
            className={
              item === tab
                ? "rounded-full bg-zinc-950 px-4 py-2 text-sm text-white"
                : "rounded-full border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-600"
            }
            href={`/listing-kits/shein-products/${storeId}?tab=${item}`}
          >
            {sheinProductsTabLabel(item)}
          </Link>
        ))}
      </nav>

      {isSheinProductsTab(tab) && tab === "products" ? (
        <section className="space-y-4">
          <input
            className="h-10 rounded-xl border border-zinc-200 bg-white px-3 text-sm"
            onChange={(event) => {
              setProductKeyword(event.target.value);
              setProductsPage(1);
            }}
            placeholder="按 SKC 搜索同步商品"
            value={productKeyword}
          />
          <SheinSyncedProductsTable
            isLoading={products.isLoading}
            items={products.data?.items ?? []}
          />
          <ListingKitPagination
            onPageChange={setProductsPage}
            page={productsPage}
            pageSize={SHEIN_PRODUCTS_PAGE_SIZE}
            total={products.data?.total ?? products.data?.items?.length ?? 0}
          />
        </section>
      ) : null}

      {tab === "costs" ? (
        <section className="space-y-4">
          <SheinCostPriceTable
            groups={sdsCostGroups.data?.items ?? []}
            items={costProducts.data?.items ?? []}
            onSave={(target, manualCostPrice) =>
              saveSheinCostTarget(
                target,
                manualCostPrice,
                updateCostMutation.mutateAsync,
                updateGroupCostMutation.mutateAsync,
              )
            }
            onSyncSourceSDSProduct={(sourceCode) =>
              syncSourceProductMutation.mutateAsync(sourceCode).then(() => undefined)
            }
            saving={updateCostMutation.isPending || updateGroupCostMutation.isPending}
            shipmentArea={summary.data?.summary?.region}
            sourceGroups={sourceSDSCostGroups.data?.items ?? []}
            storeId={storeId}
            syncingSourceCode={syncSourceProductMutation.variables}
          />
          <ListingKitPagination
            onPageChange={setCostsPage}
            page={costsPage}
            pageSize={SHEIN_PRODUCTS_PAGE_SIZE}
            total={
              sourceSDSCostGroups.data?.total ??
              sourceSDSCostGroups.data?.items?.length ??
              0
            }
          />
        </section>
      ) : null}
    </ListingKitPageShell>
  );
}

function saveSheinCostTarget(
  target: SheinCostPriceSaveTarget,
  manualCostPrice: number | null,
  updateProductCost: (input: {
    productId: number;
    manual_cost_price?: number | null;
  }) => Promise<unknown>,
  updateGroupCost: (input: {
    groupKey: string;
    group_label?: string;
    manual_cost_price?: number | null;
  }) => Promise<unknown>,
) {
  if (target.groupKey.startsWith("product:") && target.productId) {
    return updateProductCost({
      productId: target.productId,
      manual_cost_price: manualCostPrice,
    }).then(() => undefined);
  }
  return updateGroupCost({
    groupKey: target.groupKey,
    group_label: target.groupLabel,
    manual_cost_price: manualCostPrice,
  }).then(() => undefined);
}
