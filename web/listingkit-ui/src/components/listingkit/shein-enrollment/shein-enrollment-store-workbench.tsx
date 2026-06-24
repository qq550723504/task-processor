"use client";

import Link from "next/link";
import { useState } from "react";

import {
  parseSheinActivityType,
  parseSheinEnrollmentTab,
  SHEIN_ENROLLMENT_TABS,
  sheinEnrollmentTabLabel,
} from "@/components/listingkit/shein-enrollment/shein-enrollment-model";
import { SheinCandidatesTable } from "@/components/listingkit/shein-enrollment/shein-candidates-table";
import {
  SheinCostPriceTable,
  type SheinCostPriceSaveTarget,
} from "@/components/listingkit/shein-enrollment/shein-cost-price-table";
import { SheinEnrollmentRunsTable } from "@/components/listingkit/shein-enrollment/shein-enrollment-runs-table";
import { SheinEnrollmentStoreHeader } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-header";
import { SheinSyncedProductsTable } from "@/components/listingkit/shein-enrollment/shein-synced-products-table";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import {
  useExecuteSheinActivityEnrollment,
  useRefreshSheinActivityCandidates,
  useReviewSheinActivityCandidate,
  useSheinActivityCandidates,
  useSheinActivityEnrollmentRuns,
  useSheinEnrollmentStoreSummary,
  useSheinSDSCostGroups,
  useSheinSyncedProducts,
  useTriggerSheinStoreSync,
  useUpdateSheinSDSCostGroup,
  useUpdateSheinSyncedProductCost,
} from "@/lib/query/use-shein-enrollment";

export function SheinEnrollmentStoreWorkbench({
  initialActivityType,
  initialTab,
  storeId,
}: {
  initialActivityType?: string;
  initialTab?: string;
  storeId: number;
}) {
  const tab = parseSheinEnrollmentTab(initialTab);
  const [activityType, setActivityType] = useState(
    parseSheinActivityType(initialActivityType),
  );
  const [productKeyword, setProductKeyword] = useState("");
  const summary = useSheinEnrollmentStoreSummary(storeId, {
    activity_type: activityType,
  });

  const products = useSheinSyncedProducts(storeId, {
    skc_name: productKeyword || undefined,
    page: 1,
    page_size: 100,
  });
  const candidates = useSheinActivityCandidates(storeId, {
    activity_type: activityType,
    page: 1,
    page_size: 100,
  });
  const sdsCostGroups = useSheinSDSCostGroups(storeId, {
    page: 1,
    page_size: 100,
  });
  const syncMutation = useTriggerSheinStoreSync(storeId);
  const refreshMutation = useRefreshSheinActivityCandidates(storeId);
  const updateCostMutation = useUpdateSheinSyncedProductCost(storeId);
  const updateGroupCostMutation = useUpdateSheinSDSCostGroup(storeId);
  const reviewMutation = useReviewSheinActivityCandidate(storeId);
  const enrollMutation = useExecuteSheinActivityEnrollment(storeId);
  const runs = useSheinActivityEnrollmentRuns(storeId, {
    activity_type: activityType,
    page: 1,
    page_size: 100,
  });

  return (
    <ListingKitPageShell backgroundClassName="overflow-hidden rounded-lg bg-zinc-50" contentClassName="gap-5 px-4 py-4 sm:px-6 sm:py-6">
      <div className="flex items-center gap-2 text-sm text-zinc-500">
        <Link className="hover:text-zinc-900" href="/listing-kits/shein-enrollment">
          SHEIN 活动报名
        </Link>
        <span>/</span>
        <span>{summary.data?.summary?.store_name || `店铺 ${storeId}`}</span>
      </div>

      <SheinEnrollmentStoreHeader
        activityType={activityType}
        onActivityTypeChange={setActivityType}
        onRefreshCandidates={() =>
          void refreshMutation.mutateAsync({ activity_type: activityType })
        }
        onSync={() => void syncMutation.mutateAsync({ trigger_mode: "manual" })}
        refreshPending={refreshMutation.isPending}
        summary={summary.data?.summary}
        syncPending={syncMutation.isPending}
      />

      <nav aria-label="店铺工作台标签" className="flex flex-wrap gap-2">
        {SHEIN_ENROLLMENT_TABS.map((item) => (
          <Link
            key={item}
            className={
              item === tab
                ? "rounded-full bg-zinc-950 px-4 py-2 text-sm text-white"
                : "rounded-full border border-zinc-200 bg-white px-4 py-2 text-sm text-zinc-600"
            }
            href={`/listing-kits/shein-enrollment/${storeId}?tab=${item}&activityType=${activityType}`}
          >
            {sheinEnrollmentTabLabel(item)}
          </Link>
        ))}
      </nav>

      {tab === "products" ? (
        <section className="space-y-4">
          <input
            className="h-10 rounded-xl border border-zinc-200 bg-white px-3 text-sm"
            onChange={(event) => setProductKeyword(event.target.value)}
            placeholder="按 SKC 搜索同步商品"
            value={productKeyword}
          />
          <SheinSyncedProductsTable
            isLoading={products.isLoading}
            items={products.data?.items ?? []}
          />
        </section>
      ) : null}

      {tab === "costs" ? (
        <SheinCostPriceTable
          groups={sdsCostGroups.data?.items ?? []}
          items={products.data?.items ?? []}
          onSave={(target, manualCostPrice) =>
            saveSheinCostTarget(
              target,
              manualCostPrice,
              updateCostMutation.mutateAsync,
              updateGroupCostMutation.mutateAsync,
            )
          }
          saving={updateCostMutation.isPending || updateGroupCostMutation.isPending}
        />
      ) : null}

      {tab === "candidates" ? (
        <SheinCandidatesTable
          enrolling={enrollMutation.isPending}
          items={candidates.data?.items ?? []}
          onApprove={(candidateId) =>
            reviewMutation.mutateAsync({
              candidateId,
              input: {
                store_id: storeId,
                review_status: "approved",
              },
            }).then(() => undefined)
          }
          onEnroll={(candidateIds, activityKey) =>
            enrollMutation.mutateAsync({
              activity_type: activityType,
              activity_key: activityKey || undefined,
              trigger_mode: "manual_confirmed",
              candidate_ids: candidateIds,
            }).then(() => undefined)
          }
          onReject={(candidateId) =>
            reviewMutation.mutateAsync({
              candidateId,
              input: {
                store_id: storeId,
                review_status: "rejected",
              },
            }).then(() => undefined)
          }
        />
      ) : null}

      {tab === "runs" ? (
        <SheinEnrollmentRunsTable
          isLoading={runs.isLoading}
          items={runs.data?.items ?? []}
        />
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
