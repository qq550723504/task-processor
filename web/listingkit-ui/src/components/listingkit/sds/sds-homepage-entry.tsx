"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";

import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { Button } from "@/components/ui/button";
import { loadLocalSheinStudioDraftSnapshot } from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";
import { getSDSBaselineReasonShortLabel } from "@/lib/shein-studio/sds-baseline-ui";
import type { SheinStudioRecentBatchSummary } from "@/lib/types/shein-studio";
import {
  deleteSheinStudioBatch,
  getSheinStudioBatch,
  listSheinStudioBatches,
  saveSheinStudioBatch,
} from "@/lib/utils/shein-studio-batches";

function pickRecommendedRisk(summaries: SheinStudioRecentBatchSummary[]) {
  return (
    summaries.flatMap((summary) => summary.alerts ?? [])[0] ?? null
  );
}

function summarizeHomepageStatus(summary: SheinStudioRecentBatchSummary) {
  if (summary.alerts?.length) {
    return summary.alerts[0]?.label ?? "有风险";
  }
  if (summary.createdTaskCount > 0) {
    return "已有任务";
  }
  if (summary.designCount > 0) {
    return "待创建任务";
  }
  return "待生成";
}

function isMissingStudioBatchDeleteError(error: unknown) {
  return error instanceof Error && /studio session not found/i.test(error.message);
}

export function SdsHomepageEntry() {
  const router = useRouter();
  const [summaries, setSummaries] = useState<SheinStudioRecentBatchSummary[]>([]);
  const [showAllBatches, setShowAllBatches] = useState(false);
  const fullDashboardHeadingRef = useRef<HTMLHeadingElement | null>(null);

  const refreshSummaries = useCallback(async () => {
    const batches = await listSheinStudioBatches();
    setSummaries(
      buildRecentBatchSummaries(batches, {
        draft: loadLocalSheinStudioDraftSnapshot(),
      }),
    );
  }, []);

  useEffect(() => {
    let cancelled = false;

    void listSheinStudioBatches()
      .then((batches) => {
        if (cancelled) {
          return;
        }
        setSummaries(
          buildRecentBatchSummaries(batches, {
            draft: loadLocalSheinStudioDraftSnapshot(),
          }),
        );
      })
      .catch(() => {
        if (!cancelled) {
          setSummaries([]);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!showAllBatches) {
      return;
    }
    const heading = fullDashboardHeadingRef.current;
    if (!heading) {
      return;
    }
    if (typeof heading.scrollIntoView === "function") {
      heading.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    }
    heading.focus();
  }, [showAllBatches]);

  const recommendedRisk = useMemo(
    () => pickRecommendedRisk(summaries),
    [summaries],
  );
  const recommendedRiskLabel = recommendedRisk?.label ?? "";
  const recommendedRiskDetail = recommendedRisk?.reasonCode
    ? getSDSBaselineReasonShortLabel(recommendedRisk.reasonCode) ||
      recommendedRisk.reasonCode
    : "";
  const featuredSummaries = summaries.slice(0, 3);

  function handleCreateNew() {
    router.push("/listing-kits/sds/new");
  }

  function handleQuickSingleGenerate() {
    router.push("/listing-kits/sds/new?entry=single");
  }

  function handleContinueRecent() {
    if (summaries.length === 0) {
      handleCreateNew();
      return;
    }
    const latestPersistedBatch = summaries.find((summary) => summary.source === "batch");
    if (latestPersistedBatch) {
      router.push(`/listing-kits/sds/batches/${latestPersistedBatch.id}`);
      return;
    }
    setShowAllBatches(true);
    const recentBatches = document.getElementById("sds-recent-batches");
    if (recentBatches && typeof recentBatches.scrollIntoView === "function") {
      recentBatches.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    }
  }

  function handleOpenSummary(summary: SheinStudioRecentBatchSummary) {
    if (summary.source === "batch") {
      router.push(`/listing-kits/sds/batches/${summary.id}`);
      return;
    }
    setShowAllBatches(true);
  }

  function handleToggleAllBatches() {
    setShowAllBatches((current) => !current);
  }

  const handleRenameSummary = useCallback(
    async (summary: SheinStudioRecentBatchSummary, name: string) => {
      if (summary.source !== "batch") {
        return;
      }
      const nextName = name.trim();
      if (!nextName) {
        return;
      }
      const batch = await getSheinStudioBatch(summary.id);
      if (!batch) {
        return;
      }
      await saveSheinStudioBatch(
        {
          ...batch,
          name: nextName,
        },
        { makeActive: false },
      );
      await refreshSummaries();
    },
    [refreshSummaries],
  );

  const handleDuplicateSummary = useCallback(
    async (summary: SheinStudioRecentBatchSummary) => {
      if (summary.source !== "batch") {
        return;
      }
      const batch = await getSheinStudioBatch(summary.id);
      if (!batch) {
        return;
      }
      await saveSheinStudioBatch(
        {
          ...batch,
          id: undefined,
          name: `${batch.name} 副本`,
        },
        { makeActive: false },
      );
      await refreshSummaries();
    },
    [refreshSummaries],
  );

  const handleDeleteSummary = useCallback(
    async (summary: SheinStudioRecentBatchSummary) => {
      if (summary.source !== "batch") {
        return;
      }
      await deleteSheinStudioBatch(summary.id);
      await refreshSummaries();
    },
    [refreshSummaries],
  );

  const handleBulkDeleteSummaries = useCallback(
    async (summaryIds: string[]) => {
      if (summaryIds.length === 0) {
        return;
      }
      const results = await Promise.allSettled(
        summaryIds.map((summaryId) => deleteSheinStudioBatch(summaryId)),
      );
      await refreshSummaries();
      const failed = results.find(
        (result) =>
          result.status === "rejected" &&
          !isMissingStudioBatchDeleteError(result.reason),
      );
      if (failed?.status === "rejected") {
        throw failed.reason;
      }
    },
    [refreshSummaries],
  );

  return (
    <div className="flex-1 overflow-hidden bg-zinc-50">
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 px-4 py-6 lg:px-6">
        <section className="grid gap-4 rounded-lg border border-zinc-200 bg-white px-5 py-5 shadow-sm lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
          <div className="space-y-3">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
              POD
            </p>
            <div className="space-y-1">
              <h1 className="text-2xl font-semibold tracking-tight text-zinc-950">
                从 POD 商品生成上架资料
              </h1>
              <p className="max-w-2xl text-sm leading-7 text-zinc-600">
                先继续最近批次，或新建一个批次再开始选品。
              </p>
            </div>
            <p className="max-w-2xl text-sm text-zinc-600">
              {summaries.length === 0
                ? "还没有可继续的最近批次，建议先新建一个批次再开始选品。"
                : recommendedRiskLabel
                  ? `如果只是接着处理上一轮内容，优先从最近批次进入会更快，建议先处理“${recommendedRiskLabel}${recommendedRiskDetail ? ` · ${recommendedRiskDetail}` : ""}”。`
                  : "如果只是接着处理上一轮内容，优先从最近批次进入会更快。"}
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            {summaries.length > 0 ? (
              <Button onClick={handleContinueRecent} type="button" variant="secondary">
                {recommendedRiskLabel
                  ? `继续最近批次（优先处理 ${recommendedRiskLabel}${recommendedRiskDetail ? ` · ${recommendedRiskDetail}` : ""}）`
                  : "继续最近批次"}
              </Button>
            ) : null}
            <Button onClick={handleQuickSingleGenerate} type="button" variant="outline">
              快速单个生成
            </Button>
            <Button onClick={handleCreateNew} type="button">
              新建批次并选品
            </Button>
          </div>
        </section>

        <section className="space-y-3" id="sds-recent-batches">
          {featuredSummaries.length > 0 ? (
            <div className="rounded-lg border border-zinc-200 bg-white px-5 py-5 shadow-sm">
              <div className="space-y-1">
                <div className="space-y-1">
                  <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                    RECENT BATCHES
                  </p>
                  <h2 className="text-xl font-semibold tracking-tight text-zinc-950">
                    最近批次摘要
                  </h2>
                  <p className="max-w-2xl text-sm leading-7 text-zinc-600">
                    默认只展示最近 3 个批次，先快速决定继续哪一个；需要批量处理时再展开完整看板。
                  </p>
                </div>
              </div>

              {!showAllBatches ? (
                <div className="mt-4 grid gap-3 lg:grid-cols-3">
                  {featuredSummaries.map((summary) => (
                    <button
                      className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-4 text-left transition hover:border-zinc-300 hover:bg-white"
                      key={`${summary.source}:${summary.id}`}
                      onClick={() => handleOpenSummary(summary)}
                      type="button"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="space-y-1">
                          <p className="line-clamp-2 text-sm font-semibold text-zinc-950">
                            {summary.title}
                          </p>
                          <p className="line-clamp-1 text-xs text-zinc-500">
                            {summary.primaryProductName}
                          </p>
                        </div>
                        <span className="rounded-full border border-zinc-200 bg-white px-2 py-1 text-[11px] text-zinc-700">
                          {summarizeHomepageStatus(summary)}
                        </span>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-x-3 gap-y-1 text-xs text-zinc-600">
                        <span>{summary.productCount} 款商品</span>
                        <span>{summary.storeSummary}</span>
                        <span>{summary.designCount} 图 / {summary.createdTaskCount} 任务</span>
                      </div>
                      <p className="mt-3 text-xs text-zinc-500">
                        更新于 {new Date(summary.updatedAt).toLocaleString("zh-CN")}
                      </p>
                    </button>
                  ))}
                </div>
              ) : (
                <div className="mt-4 rounded-xl border border-dashed border-zinc-200 bg-zinc-50 px-4 py-4 text-sm text-zinc-600">
                  最近 3 个批次摘要已折叠，避免和下面的完整看板重复；处理完成后可以随时返回首页摘要。
                </div>
              )}
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-zinc-200 bg-white px-5 py-5 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                RECENT BATCHES
              </p>
              <h2 className="mt-1 text-lg font-semibold tracking-tight text-zinc-950">
                还没有可继续的最近批次
              </h2>
              <p className="mt-2 max-w-2xl text-sm leading-7 text-zinc-600">
                首页先保留为空态入口，等你创建第一个批次后，这里会显示最近批次摘要和完整看板入口。
              </p>
            </div>
          )}

          {summaries.length > 3 ? (
            <div className="flex justify-end">
              <Button
                onClick={handleToggleAllBatches}
                type="button"
                variant="ghost"
              >
                {showAllBatches ? "返回首页摘要（最近 3 个）" : "查看全部批次"}
              </Button>
            </div>
          ) : null}
          {showAllBatches ? (
            <div className="space-y-3">
              <div className="space-y-1 px-1">
                <h2
                  className="text-xl font-semibold tracking-tight text-zinc-950"
                  ref={fullDashboardHeadingRef}
                  tabIndex={-1}
                >
                  全部批次看板
                </h2>
                <p className="max-w-2xl text-sm leading-7 text-zinc-600">
                  在这里继续使用筛选、风险分诊、批量操作和队列入口。
                </p>
              </div>
              <SheinStudioRecentBatchesDashboard
                onBulkDeleteSummaries={handleBulkDeleteSummaries}
                onCreateBatch={handleCreateNew}
                onDeleteSummary={handleDeleteSummary}
                onDuplicateSummary={handleDuplicateSummary}
                onRenameSummary={handleRenameSummary}
                onSelectSummary={handleOpenSummary}
                onSelectSummaryAction={handleOpenSummary}
                summaries={summaries}
              />
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}
