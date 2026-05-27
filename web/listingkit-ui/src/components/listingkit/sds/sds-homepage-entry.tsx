"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { Button } from "@/components/ui/button";
import { loadLocalSheinStudioDraftSnapshot } from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";
import type { SheinStudioRecentBatchSummary } from "@/lib/types/shein-studio";
import { listSheinStudioBatches } from "@/lib/utils/shein-studio-batches";

function pickRecommendedRiskLabel(summaries: SheinStudioRecentBatchSummary[]) {
  return (
    summaries.flatMap((summary) => summary.alerts?.map((alert) => alert.label) ?? [])[0] ??
    ""
  );
}

export function SdsHomepageEntry() {
  const router = useRouter();
  const [summaries, setSummaries] = useState<SheinStudioRecentBatchSummary[]>([]);
  const [showAllBatches, setShowAllBatches] = useState(false);

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

  const recommendedRiskLabel = useMemo(
    () => pickRecommendedRiskLabel(summaries),
    [summaries],
  );
  const visibleSummaries = showAllBatches ? summaries : summaries.slice(0, 3);

  function handleCreateNew() {
    router.push("/listing-kits/sds/new");
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
                  ? `如果只是接着处理上一轮内容，优先从最近批次进入会更快，建议先处理“${recommendedRiskLabel}”。`
                  : "如果只是接着处理上一轮内容，优先从最近批次进入会更快。"}
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            {summaries.length > 0 ? (
              <Button onClick={handleContinueRecent} type="button" variant="secondary">
                {recommendedRiskLabel
                  ? `继续最近批次（优先处理 ${recommendedRiskLabel}）`
                  : "继续最近批次"}
              </Button>
            ) : null}
            <Button onClick={handleCreateNew} type="button">
              新建批次并选品
            </Button>
          </div>
        </section>

        <section className="space-y-3" id="sds-recent-batches">
          <SheinStudioRecentBatchesDashboard
            onCreateBatch={handleCreateNew}
            onSelectSummary={handleOpenSummary}
            onSelectSummaryAction={handleOpenSummary}
            summaries={visibleSummaries}
          />
          {summaries.length > 3 ? (
            <div className="flex justify-end">
              <Button
                onClick={() => setShowAllBatches((current) => !current)}
                type="button"
                variant="ghost"
              >
                {showAllBatches ? "收起到最近 3 个" : "查看全部批次"}
              </Button>
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}
