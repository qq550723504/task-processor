"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";

import { SheinStudioBatchRunProgress } from "@/components/listingkit/shein-studio/shein-studio-batch-run-progress";
import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/client";
import { startSheinStudioBatchRun } from "@/lib/api/shein-studio-batch-runs";
import {
  clearLocalSheinStudioDraftSnapshot,
  loadLocalSheinStudioDraftSnapshotDetail,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { buildDuplicatedSheinStudioBatchInput } from "@/lib/shein-studio/duplicate-batch";
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

function stepForSummaryAction(
  summary: SheinStudioRecentBatchSummary,
  action?: "generate" | "review" | "tasks",
) {
  if (action) {
    return action;
  }
  if (summary.createdTaskCount > 0) {
    return "tasks";
  }
  if (summary.designCount > 0) {
    return "review";
  }
  return "generate";
}

function buildSummaryRoute(
  summary: SheinStudioRecentBatchSummary,
  action?: "generate" | "review" | "tasks",
) {
  if (summary.source === "batch") {
    return `/listing-kits/sds/batches/${summary.id}`;
  }
  const step = stepForSummaryAction(summary, action);
  return `/listing-kits/sds/new?step=${step}`;
}

function isMissingStudioBatchDeleteError(error: unknown) {
  return error instanceof Error && /studio session not found/i.test(error.message);
}

function isRecentBatchRejectedError(error: unknown) {
  if (error instanceof ApiError) {
    return error.status === 401 || error.status === 403;
  }
  return (
    error instanceof Error &&
    /missing zitadel session|missing zitadel bearer token|zitadel/i.test(
      error.message,
    )
  );
}

function isRecentBatchInactiveTokenError(error: unknown) {
  return (
    error instanceof Error &&
    /inactive token|same zitadel issuer\/client configuration|different issuer\/client/i.test(
      error.message,
    )
  );
}

function getRecentBatchErrorMessage(error: unknown) {
  if (isRecentBatchInactiveTokenError(error)) {
    return "最近批次接口拿到的是一张当前后端不认可的 ZITADEL token。既然其他页面正常，这通常不是你没登录，而是前端和 API 用的 ZITADEL issuer 或 client 配置没对齐。";
  }
  if (isRecentBatchRejectedError(error)) {
    return "最近批次接口这次请求被拒绝了。既然其他页面正常，这更像是这个接口自己的鉴权或会话透传有问题，请重试；如果持续失败，再单独排查这个接口。";
  }
  return "最近批次这次没有成功加载出来，请重试；如果持续失败，再检查登录态或后端服务。";
}

function getBatchRunStartErrorMessage(error: unknown) {
  if (error instanceof ApiError && error.status === 404) {
    return "这轮批量生成里有批次已经不存在了。请先刷新最近批次列表，再重新选择。";
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return "这轮批量生成没有成功启动，请稍后重试。";
}

export function SdsHomepageEntry() {
  const router = useRouter();
  const [localDraftSnapshotDetail, setLocalDraftSnapshotDetail] = useState(
    () => loadLocalSheinStudioDraftSnapshotDetail(),
  );
  const [summaries, setSummaries] = useState<SheinStudioRecentBatchSummary[]>([]);
  const [summariesError, setSummariesError] = useState("");
  const [isLoadingSummaries, setIsLoadingSummaries] = useState(true);
  const [showAllBatches, setShowAllBatches] = useState(false);
  const [activeBatchRunId, setActiveBatchRunId] = useState("");
  const [batchRunError, setBatchRunError] = useState("");
  const fullDashboardHeadingRef = useRef<HTMLHeadingElement | null>(null);

  const refreshSummaries = useCallback(async () => {
    setIsLoadingSummaries(true);
    setSummariesError("");
    try {
      const batches = await listSheinStudioBatches();
      setSummaries(
        buildRecentBatchSummaries(batches, {
          draft: localDraftSnapshotDetail?.draft ?? null,
          draftBatchId: localDraftSnapshotDetail?.batchId,
        }),
      );
    } catch (error) {
      setSummaries([]);
      setSummariesError(getRecentBatchErrorMessage(error));
    } finally {
      setIsLoadingSummaries(false);
    }
  }, [localDraftSnapshotDetail]);

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      setIsLoadingSummaries(true);
      setSummariesError("");
      try {
        const batches = await listSheinStudioBatches();
        if (cancelled) {
          return;
        }
        setSummaries(
          buildRecentBatchSummaries(batches, {
            draft: localDraftSnapshotDetail?.draft ?? null,
            draftBatchId: localDraftSnapshotDetail?.batchId,
          }),
        );
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSummaries([]);
        setSummariesError(getRecentBatchErrorMessage(error));
      } finally {
        if (!cancelled) {
          setIsLoadingSummaries(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [localDraftSnapshotDetail]);

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

  async function handleOpenBatchQueue(input: {
    batchIds: string[];
    mode: "generate" | "create_tasks";
  }) {
    if (input.batchIds.length === 0) {
      return;
    }
    if (input.mode !== "generate") {
      router.push(`/listing-kits/sds/batches/${input.batchIds[0]}`);
      return;
    }

    setBatchRunError("");
    try {
      const response = await startSheinStudioBatchRun(input.batchIds, input.mode);
      setActiveBatchRunId(response.run.id);
    } catch (error) {
      setBatchRunError(getBatchRunStartErrorMessage(error));
    }
  }

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
      router.push(buildSummaryRoute(latestPersistedBatch));
      return;
    }
    const latestRecoverableDraft = summaries.find(
      (summary) => summary.source === "local_draft",
    );
    if (latestRecoverableDraft) {
      router.push(buildSummaryRoute(latestRecoverableDraft));
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

  function handleOpenSummary(
    summary: SheinStudioRecentBatchSummary,
    action?: "generate" | "review" | "tasks",
  ) {
    router.push(buildSummaryRoute(summary, action));
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
        buildDuplicatedSheinStudioBatchInput(batch),
        { makeActive: false },
      );
      await refreshSummaries();
    },
    [refreshSummaries],
  );

  const handleDeleteSummary = useCallback(
    async (summary: SheinStudioRecentBatchSummary) => {
      if (summary.source === "local_draft") {
        clearLocalSheinStudioDraftSnapshot();
        setLocalDraftSnapshotDetail(null);
        await refreshSummaries();
        return;
      }
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

  if (activeBatchRunId) {
    return (
      <div className="flex-1 overflow-hidden bg-background">
        <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 px-4 py-6 lg:px-6">
          <SheinStudioBatchRunProgress
            onBack={() => {
              setActiveBatchRunId("");
              void refreshSummaries();
            }}
            runId={activeBatchRunId}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-hidden bg-background">
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-6 px-4 py-6 lg:px-6">
        <section className="grid gap-4 rounded-lg border border-border bg-card px-5 py-5 shadow-sm xl:grid-cols-[minmax(0,1fr)_auto] xl:items-center">
          <div className="space-y-3">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
              POD
            </p>
            <div className="space-y-1">
              <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                从 POD 商品生成上架资料
              </h1>
              <p className="max-w-2xl text-sm leading-7 text-muted-foreground">
                先继续最近批次，或新建一个批次再开始选品。
              </p>
            </div>
            <p className="max-w-2xl text-sm text-muted-foreground">
              {summariesError
                ? summariesError
                : summaries.length === 0
                ? "还没有可继续的最近批次，建议先新建一个批次再开始选品。"
                : recommendedRiskLabel
                  ? `如果只是接着处理上一轮内容，优先从最近批次进入会更快，建议先处理“${recommendedRiskLabel}${recommendedRiskDetail ? ` · ${recommendedRiskDetail}` : ""}”。`
                  : "如果只是接着处理上一轮内容，优先从最近批次进入会更快。"}
            </p>
          </div>

          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
            {summaries.length > 0 ? (
              <Button
                className="w-full sm:w-auto"
                onClick={handleContinueRecent}
                type="button"
                variant="secondary"
              >
                {recommendedRiskLabel
                  ? `继续最近批次（优先处理 ${recommendedRiskLabel}${recommendedRiskDetail ? ` · ${recommendedRiskDetail}` : ""}）`
                  : "继续最近批次"}
              </Button>
            ) : null}
            <Button
              className="w-full sm:w-auto"
              onClick={handleQuickSingleGenerate}
              type="button"
              variant="outline"
            >
              快速单个生成
            </Button>
            <Button className="w-full sm:w-auto" onClick={handleCreateNew} type="button">
              新建批次并选品
            </Button>
          </div>
        </section>

        <section className="space-y-3" id="sds-recent-batches">
          {batchRunError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-5 py-4 text-sm text-rose-900 shadow-sm">
              {batchRunError}
            </div>
          ) : null}
          {summariesError ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 px-5 py-5 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
                RECENT BATCHES
              </p>
              <h2 className="mt-1 text-lg font-semibold tracking-tight text-zinc-950">
                最近批次暂时加载失败
              </h2>
              <p className="mt-2 max-w-2xl text-sm leading-7 text-zinc-700">
                {summariesError}
              </p>
              <div className="mt-4 flex flex-wrap gap-2">
                <Button onClick={() => void refreshSummaries()} type="button">
                  重新加载最近批次
                </Button>
                <Button onClick={handleCreateNew} type="button" variant="outline">
                  新建批次并选品
                </Button>
              </div>
            </div>
          ) : isLoadingSummaries ? (
            <div className="rounded-lg border border-border bg-card px-5 py-5 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                RECENT BATCHES
              </p>
              <h2 className="mt-1 text-lg font-semibold tracking-tight text-foreground">
                正在加载最近批次
              </h2>
              <p className="mt-2 max-w-2xl text-sm leading-7 text-muted-foreground">
                正在同步最近批次摘要和状态，请稍等。
              </p>
            </div>
          ) : featuredSummaries.length > 0 ? (
            <div className="rounded-lg border border-border bg-card px-5 py-5 shadow-sm">
              <div className="space-y-1">
                <div className="space-y-1">
                  <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                    RECENT BATCHES
                  </p>
                  <h2 className="text-xl font-semibold tracking-tight text-foreground">
                    最近批次摘要
                  </h2>
                  <p className="max-w-2xl text-sm leading-7 text-muted-foreground">
                    默认只展示最近 3 个批次，先快速决定继续哪一个；需要批量处理时再展开完整看板。
                  </p>
                </div>
              </div>

              {!showAllBatches ? (
                <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {featuredSummaries.map((summary) => (
                    <button
                      className="rounded-xl border border-border bg-muted px-4 py-4 text-left transition hover:border-foreground/20 hover:bg-card"
                      key={`${summary.source}:${summary.id}`}
                      onClick={() => handleOpenSummary(summary)}
                      type="button"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="space-y-1">
                          <p className="line-clamp-2 text-sm font-semibold text-foreground">
                            {summary.title}
                          </p>
                          <p className="line-clamp-1 text-xs text-muted-foreground">
                            {summary.primaryProductName}
                          </p>
                        </div>
                        <span className="rounded-full border border-border bg-background px-2 py-1 text-[11px] text-muted-foreground">
                          {summarizeHomepageStatus(summary)}
                        </span>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                        <span>{summary.productCount} 款商品</span>
                        <span>{summary.storeSummary}</span>
                        <span>{summary.designCount} 图 / {summary.createdTaskCount} 任务</span>
                      </div>
                      <p className="mt-3 text-xs text-muted-foreground">
                        更新于 {new Date(summary.updatedAt).toLocaleString("zh-CN")}
                      </p>
                    </button>
                  ))}
                </div>
              ) : (
                <div className="mt-4 rounded-xl border border-dashed border-border bg-muted px-4 py-4 text-sm text-muted-foreground">
                  最近 3 个批次摘要已折叠，避免和下面的完整看板重复；处理完成后可以随时返回首页摘要。
                </div>
              )}
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-border bg-card px-5 py-5 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                RECENT BATCHES
              </p>
              <h2 className="mt-1 text-lg font-semibold tracking-tight text-foreground">
                还没有可继续的最近批次
              </h2>
              <p className="mt-2 max-w-2xl text-sm leading-7 text-muted-foreground">
                首页先保留为空态入口，等你创建第一个批次后，这里会显示最近批次摘要和完整看板入口。
              </p>
            </div>
          )}

          {summaries.length > 0 ? (
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
                  className="text-xl font-semibold tracking-tight text-foreground"
                  ref={fullDashboardHeadingRef}
                  tabIndex={-1}
                >
                  全部批次看板
                </h2>
                <p className="max-w-2xl text-sm leading-7 text-muted-foreground">
                  在这里继续使用筛选、风险分诊、批量操作和队列入口。
                </p>
              </div>
              <SheinStudioRecentBatchesDashboard
                onBulkDeleteSummaries={handleBulkDeleteSummaries}
                onCreateBatch={handleCreateNew}
                onDeleteSummary={handleDeleteSummary}
                onDuplicateSummary={handleDuplicateSummary}
                onOpenBatchQueue={(input) => {
                  void handleOpenBatchQueue(input);
                }}
                onRenameSummary={handleRenameSummary}
                onSelectSummary={handleOpenSummary}
                onSelectSummaryAction={(summary, action) =>
                  handleOpenSummary(summary, action)
                }
                summaries={summaries}
              />
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}
