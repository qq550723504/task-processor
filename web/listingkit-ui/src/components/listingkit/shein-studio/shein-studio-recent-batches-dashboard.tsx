"use client";

import { Button } from "@/components/ui/button";
import type { SheinStudioRecentBatchSummary } from "@/lib/types/shein-studio";

export function SheinStudioRecentBatchesDashboard({
  summaries,
  onCreateBatch,
  onSelectSummary,
}: {
  summaries: SheinStudioRecentBatchSummary[];
  onCreateBatch: () => void;
  onSelectSummary: (summary: SheinStudioRecentBatchSummary) => void;
}) {
  return (
    <section className="space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            Recent Batches
          </p>
          <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
            最近批次
          </h2>
          <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">
            先从最近的独立批次继续编辑，再决定是否新建新批次。
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {summaries.length > 0 ? (
            <Button
              onClick={() => onSelectSummary(summaries[0])}
              type="button"
              variant="secondary"
            >
              继续最近批次
            </Button>
          ) : null}
          <Button onClick={onCreateBatch} type="button">
            新建批次
          </Button>
        </div>
      </div>

      {summaries.length === 0 ? (
        <div className="rounded-3xl border border-dashed border-zinc-200 bg-zinc-50 px-5 py-8 text-sm text-zinc-600">
          还没有可继续的批次。先在选品区选择 SDS 商品，创建第一批内容。
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
          {summaries.map((summary) => (
            <button
              className="rounded-3xl border border-zinc-200 bg-zinc-50 px-4 py-4 text-left transition hover:border-zinc-400 hover:bg-white"
              key={`${summary.source}:${summary.id}`}
              onClick={() => onSelectSummary(summary)}
              type="button"
            >
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-zinc-950">
                    {summary.title}
                  </p>
                  <p className="mt-1 text-xs text-zinc-500">
                    {summary.isRecoverableDraft ? "未保存草稿" : "已保存批次"}
                  </p>
                </div>
                <span className="rounded-full bg-zinc-900 px-2.5 py-1 text-[11px] font-medium text-white">
                  {summary.productCount} 款商品
                </span>
              </div>
              <dl className="mt-4 space-y-2 text-sm text-zinc-700">
                <div className="flex items-start justify-between gap-3">
                  <dt className="text-zinc-500">主商品</dt>
                  <dd className="text-right">{summary.primaryProductName}</dd>
                </div>
                <div className="flex items-start justify-between gap-3">
                  <dt className="text-zinc-500">店铺分发</dt>
                  <dd className="text-right">{summary.storeSummary}</dd>
                </div>
                <div className="flex items-start justify-between gap-3">
                  <dt className="text-zinc-500">设计/任务</dt>
                  <dd className="text-right">
                    {summary.designCount} 图 / {summary.createdTaskCount} 任务
                  </dd>
                </div>
              </dl>
              <p className="mt-4 line-clamp-2 text-sm text-zinc-600">
                {summary.promptPreview}
              </p>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
