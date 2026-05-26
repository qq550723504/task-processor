"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import type { SheinStudioRecentBatchSummary } from "@/lib/types/shein-studio";

type StoreOption = {
  id: string;
  label: string;
};

export function SheinStudioRecentBatchesDashboard({
  summaries,
  storeOptions = [],
  onBulkUpdateStore,
  onCreateBatch,
  onDeleteSummary,
  onDuplicateSummary,
  onRenameSummary,
  onSelectSummary,
}: {
  summaries: SheinStudioRecentBatchSummary[];
  storeOptions?: StoreOption[];
  onBulkUpdateStore?: (summaryIds: string[], storeId: string) => void;
  onCreateBatch: () => void;
  onDeleteSummary?: (summary: SheinStudioRecentBatchSummary) => void;
  onDuplicateSummary?: (summary: SheinStudioRecentBatchSummary) => void;
  onRenameSummary?: (summary: SheinStudioRecentBatchSummary, name: string) => void;
  onSelectSummary: (summary: SheinStudioRecentBatchSummary) => void;
}) {
  const [selectedSummaryIds, setSelectedSummaryIds] = useState<string[]>([]);
  const [editingSummaryId, setEditingSummaryId] = useState("");
  const [draftName, setDraftName] = useState("");
  const [bulkStoreId, setBulkStoreId] = useState("");

  const selectedCount = selectedSummaryIds.length;
  const summaryById = useMemo(
    () =>
      new Map<string, SheinStudioRecentBatchSummary>(
        summaries.map((summary) => [`${summary.source}:${summary.id}`, summary]),
      ),
    [summaries],
  );

  function toggleSelection(summary: SheinStudioRecentBatchSummary) {
    const key = `${summary.source}:${summary.id}`;
    setSelectedSummaryIds((current) =>
      current.includes(key)
        ? current.filter((item) => item !== key)
        : [...current, key],
    );
  }

  function beginRename(summary: SheinStudioRecentBatchSummary) {
    setEditingSummaryId(`${summary.source}:${summary.id}`);
    setDraftName(summary.title);
  }

  function clearRename() {
    setEditingSummaryId("");
    setDraftName("");
  }

  function applyRename(summary: SheinStudioRecentBatchSummary) {
    const nextName = draftName.trim();
    if (!nextName || !onRenameSummary) {
      clearRename();
      return;
    }
    onRenameSummary(summary, nextName);
    clearRename();
  }

  function applyBulkStoreUpdate() {
    if (!onBulkUpdateStore || selectedSummaryIds.length === 0) {
      return;
    }
    const ids = selectedSummaryIds
      .map((key) => summaryById.get(key))
      .filter((summary): summary is SheinStudioRecentBatchSummary => Boolean(summary))
      .map((summary) => summary.id);
    onBulkUpdateStore(ids, bulkStoreId);
  }

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

      {selectedCount > 0 ? (
        <div className="space-y-3 rounded-3xl border border-zinc-200 bg-zinc-50 px-4 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="text-sm font-medium text-zinc-900">
              已选择 {selectedCount} 个批次
            </div>
            <Button
              onClick={() => setSelectedSummaryIds([])}
              size="sm"
              type="button"
              variant="ghost"
            >
              清除选择
            </Button>
          </div>
          <div className="flex flex-wrap items-end gap-3">
            <label className="min-w-[220px] text-sm text-zinc-600">
              <span className="mb-1 block">目标店铺</span>
              <select
                aria-label="目标店铺"
                className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900"
                onChange={(event) => setBulkStoreId(event.target.value)}
                value={bulkStoreId}
              >
                <option value="">跟随当前店铺</option>
                {storeOptions.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <Button
              disabled={!onBulkUpdateStore}
              onClick={applyBulkStoreUpdate}
              type="button"
            >
              应用到已选批次
            </Button>
          </div>
        </div>
      ) : null}

      {summaries.length === 0 ? (
        <div className="rounded-3xl border border-dashed border-zinc-200 bg-zinc-50 px-5 py-8 text-sm text-zinc-600">
          还没有可继续的批次。先在选品区选择 SDS 商品，创建第一批内容。
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
          {summaries.map((summary) => {
            const summaryKey = `${summary.source}:${summary.id}`;
            const isSelected = selectedSummaryIds.includes(summaryKey);
            const isEditing = editingSummaryId === summaryKey;
            return (
              <div
                className={`rounded-3xl border px-4 py-4 transition ${
                  isSelected
                    ? "border-emerald-400 bg-emerald-50/60"
                    : "border-zinc-200 bg-zinc-50"
                }`}
                key={summaryKey}
              >
                <div className="flex items-start justify-between gap-3">
                  <label className="inline-flex items-center gap-2 text-xs text-zinc-500">
                    <input
                      aria-label={`select ${summary.id}`}
                      checked={isSelected}
                      onChange={() => toggleSelection(summary)}
                      type="checkbox"
                    />
                    选择
                  </label>
                  <div className="flex flex-wrap justify-end gap-2">
                    {!summary.isRecoverableDraft && onRenameSummary ? (
                      <Button
                        onClick={() => beginRename(summary)}
                        size="sm"
                        type="button"
                        variant="ghost"
                      >
                        重命名
                      </Button>
                    ) : null}
                    {!summary.isRecoverableDraft && onDuplicateSummary ? (
                      <Button
                        onClick={() => onDuplicateSummary(summary)}
                        size="sm"
                        type="button"
                        variant="ghost"
                      >
                        复制
                      </Button>
                    ) : null}
                    {!summary.isRecoverableDraft && onDeleteSummary ? (
                      <Button
                        onClick={() => onDeleteSummary(summary)}
                        size="sm"
                        type="button"
                        variant="ghost"
                      >
                        删除
                      </Button>
                    ) : null}
                  </div>
                </div>

                {isEditing ? (
                  <div className="mt-3 space-y-2">
                    <label className="block text-sm text-zinc-700">
                      <span className="mb-1 block">批次名称</span>
                      <input
                        aria-label="批次名称"
                        className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900"
                        onChange={(event) => setDraftName(event.target.value)}
                        value={draftName}
                      />
                    </label>
                    <div className="flex gap-2">
                      <Button
                        onClick={() => applyRename(summary)}
                        size="sm"
                        type="button"
                      >
                        保存名称
                      </Button>
                      <Button
                        onClick={clearRename}
                        size="sm"
                        type="button"
                        variant="ghost"
                      >
                        取消
                      </Button>
                    </div>
                  </div>
                ) : (
                  <button
                    className="mt-3 w-full text-left"
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
                )}
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
