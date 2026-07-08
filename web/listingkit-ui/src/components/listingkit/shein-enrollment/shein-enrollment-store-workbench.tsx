"use client";

import Link from "next/link";
import { useState } from "react";

import {
  isSheinActivityStrategyReady,
  SheinActivityStrategyCard,
} from "@/components/listingkit/shein-enrollment/shein-activity-strategy-card";
import {
  parseSheinActivityType,
  parseSheinEnrollmentTab,
  SHEIN_ENROLLMENT_TABS,
  sheinEnrollmentTabLabel,
} from "@/components/listingkit/shein-enrollment/shein-enrollment-model";
import { SheinCandidatesTable } from "@/components/listingkit/shein-enrollment/shein-candidates-table";
import { SheinEnrollmentRunsTable } from "@/components/listingkit/shein-enrollment/shein-enrollment-runs-table";
import { SheinEnrollmentStoreHeader } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-header";
import { isSheinProductsTab } from "@/components/listingkit/shein-products/shein-products-model";
import { SheinProductsStoreWorkbench } from "@/components/listingkit/shein-products/shein-products-store-workbench";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { ListingKitPagination } from "@/components/listingkit/shared/listingkit-pagination";
import {
  useExecuteSheinActivityEnrollment,
  useSheinActivityStrategy,
  useRefreshSheinActivityCandidates,
  useResetSheinActivityCandidates,
  useReviewSheinActivityCandidate,
  useSheinActivityCandidates,
  useSheinActivityEnrollmentRunItems,
  useSheinActivityEnrollmentRuns,
  useSheinEnrollmentStoreSummary,
  useUpdateSheinActivityStrategy,
} from "@/lib/query/use-shein-enrollment";

const SHEIN_ENROLLMENT_PAGE_SIZE = 100;
const SHEIN_ENROLLMENT_RUN_ITEM_PAGE_SIZE = 50;

export function SheinEnrollmentStoreWorkbench({
  initialActivityType,
  initialTab,
  storeId,
}: {
  initialActivityType?: string;
  initialTab?: string;
  storeId: number;
}) {
  if (isSheinProductsTab(initialTab)) {
    return <SheinProductsStoreWorkbench initialTab={initialTab} storeId={storeId} />;
  }

  return (
    <SheinEnrollmentActivityWorkbench
      initialActivityType={initialActivityType}
      initialTab={initialTab}
      storeId={storeId}
    />
  );
}

function SheinEnrollmentActivityWorkbench({
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
  const [showExecutableOnly, setShowExecutableOnly] = useState(false);
  const [candidateSkcKeyword, setCandidateSkcKeyword] = useState("");
  const [candidatesPage, setCandidatesPage] = useState(1);
  const [runsPage, setRunsPage] = useState(1);
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null);
  const candidatesTabActive = tab === "candidates";
  const runsTabActive = tab === "runs";
  const candidateSkcName = candidateSkcKeyword.trim();
  const summary = useSheinEnrollmentStoreSummary(storeId, {
    activity_type: activityType,
  });
  const activityStrategy = useSheinActivityStrategy(storeId, activityType);

  const candidates = useSheinActivityCandidates(
    storeId,
    {
      activity_type: activityType,
      executable_only: showExecutableOnly || undefined,
      ...(candidateSkcName ? { skc_name: candidateSkcName } : {}),
      page: candidatesPage,
      page_size: SHEIN_ENROLLMENT_PAGE_SIZE,
    },
    { enabled: candidatesTabActive },
  );
  const refreshMutation = useRefreshSheinActivityCandidates(storeId);
  const resetCandidatesMutation = useResetSheinActivityCandidates(storeId);
  const updateActivityStrategyMutation = useUpdateSheinActivityStrategy(storeId);
  const reviewMutation = useReviewSheinActivityCandidate(storeId);
  const enrollMutation = useExecuteSheinActivityEnrollment(storeId);
  const runs = useSheinActivityEnrollmentRuns(
    storeId,
    {
      activity_type: activityType,
      page: runsPage,
      page_size: SHEIN_ENROLLMENT_PAGE_SIZE,
    },
    { enabled: runsTabActive },
  );
  const runItems = useSheinActivityEnrollmentRunItems(
    storeId,
    selectedRunId ?? 0,
    {
      page: 1,
      page_size: SHEIN_ENROLLMENT_RUN_ITEM_PAGE_SIZE,
    },
    { enabled: runsTabActive && selectedRunId != null },
  );
  const activityStrategyReady = isSheinActivityStrategyReady(
    activityStrategy.data,
    activityType,
  );

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
        onActivityTypeChange={(nextActivityType) => {
          setActivityType(nextActivityType);
          setCandidatesPage(1);
          setRunsPage(1);
          setSelectedRunId(null);
        }}
        onRefreshCandidates={() =>
          void refreshMutation.mutateAsync({ activity_type: activityType })
        }
        refreshPending={refreshMutation.isPending}
        summary={summary.data?.summary}
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

      {tab === "candidates" ? (
        <section className="space-y-4">
          <SheinActivityStrategyCard
            activityType={activityType}
            configured={activityStrategy.data?.configured}
            onSave={(input) =>
              updateActivityStrategyMutation
                .mutateAsync({ ...input, activity_type: activityType })
                .then(() => undefined)
            }
            saving={updateActivityStrategyMutation.isPending}
            strategy={activityStrategy.data?.strategy}
          />
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <input
              className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm sm:w-80"
              onChange={(event) => {
                setCandidateSkcKeyword(event.target.value);
                setCandidatesPage(1);
              }}
              placeholder="输入完整 SKC 搜索候选商品"
              value={candidateSkcKeyword}
            />
            <label className="flex w-fit items-center gap-2 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-700">
              <input
                checked={showExecutableOnly}
                onChange={(event) => {
                  setShowExecutableOnly(event.target.checked);
                  setCandidatesPage(1);
                }}
                type="checkbox"
              />
              只看可报名
            </label>
          </div>
          <SheinCandidatesTable
            enrollmentDisabled={!activityStrategyReady}
            enrollmentDisabledReason={
              activityStrategyReady ? undefined : "先完善活动报名设置"
            }
            enrolling={enrollMutation.isPending}
            items={candidates.data?.items ?? []}
            key={`${activityType}:${showExecutableOnly}:${candidateSkcName}:${candidatesPage}`}
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
            onReset={(candidateIds) =>
              resetCandidatesMutation.mutateAsync({
                activity_type: activityType,
                candidate_ids: candidateIds,
              }).then(() => undefined)
            }
            resetting={resetCandidatesMutation.isPending}
          />
          <ListingKitPagination
            onPageChange={setCandidatesPage}
            page={candidatesPage}
            pageSize={SHEIN_ENROLLMENT_PAGE_SIZE}
            total={candidates.data?.total ?? candidates.data?.items?.length ?? 0}
          />
        </section>
      ) : null}

      {tab === "runs" ? (
        <section className="space-y-4">
          <SheinEnrollmentRunsTable
            detailItems={runItems.data?.items ?? []}
            detailLoading={runItems.isLoading}
            isLoading={runs.isLoading}
            items={runs.data?.items ?? []}
            onViewDetails={(runId) => setSelectedRunId(runId)}
            selectedRunId={selectedRunId}
          />
          <ListingKitPagination
            onPageChange={setRunsPage}
            page={runsPage}
            pageSize={SHEIN_ENROLLMENT_PAGE_SIZE}
            total={runs.data?.total ?? runs.data?.items?.length ?? 0}
          />
        </section>
      ) : null}
    </ListingKitPageShell>
  );
}
