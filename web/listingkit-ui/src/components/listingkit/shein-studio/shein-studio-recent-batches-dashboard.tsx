"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import type { SheinStudioRecentBatchSummary } from "@/lib/types/shein-studio";

type StoreOption = {
  id: string;
  label: string;
};

type RecentBatchCardAction = "generate" | "review" | "tasks";
type RecentBatchStatusFilter = "all" | "generate" | "review" | "tasks" | "risk";

type RecentBatchAlertAction = {
  action: RecentBatchCardAction;
  label: string;
};

function recentBatchAlertToneClass(tone: "warning" | "danger") {
  return tone === "danger"
    ? "border-rose-200 bg-rose-50 text-rose-700"
    : "border-amber-200 bg-amber-50 text-amber-700";
}

function actionForRiskAlert(label: string): RecentBatchAlertAction | null {
  if (label === "Baseline 未就绪") {
    return {
      action: "generate",
      label: "去生成区处理",
    };
  }
  if (label === "生成失败") {
    return {
      action: "generate",
      label: "回到生成区重试",
    };
  }
  if (label === "待确认款式") {
    return {
      action: "review",
      label: "去确认设计",
    };
  }
  return null;
}

function isRiskySummary(summary: SheinStudioRecentBatchSummary) {
  return Boolean(summary.alerts?.length);
}

function formatRecentBatchTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "时间未知";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function SheinStudioRecentBatchesDashboard({
  summaries,
  selectedSummaryIds: controlledSelectedSummaryIds,
  storeOptions = [],
  onBulkUpdateStore,
  onCreateBatch,
  onDeleteSummary,
  onDuplicateSummary,
  onOpenBatchQueue,
  onRenameSummary,
  onSelectedSummaryIdsChange,
  onSelectSummaryAction,
  onSelectSummary,
}: {
  summaries: SheinStudioRecentBatchSummary[];
  selectedSummaryIds?: string[];
  storeOptions?: StoreOption[];
  onBulkUpdateStore?: (summaryIds: string[], storeId: string) => void;
  onCreateBatch: () => void;
  onDeleteSummary?: (summary: SheinStudioRecentBatchSummary) => void;
  onDuplicateSummary?: (summary: SheinStudioRecentBatchSummary) => void;
  onOpenBatchQueue?: (input: {
    batchIds: string[];
    mode: "generate" | "create_tasks";
  }) => void;
  onRenameSummary?: (summary: SheinStudioRecentBatchSummary, name: string) => void;
  onSelectedSummaryIdsChange?: (
    value: string[] | ((current: string[]) => string[]),
  ) => void;
  onSelectSummaryAction?: (
    summary: SheinStudioRecentBatchSummary,
    action: RecentBatchCardAction,
  ) => void;
  onSelectSummary: (summary: SheinStudioRecentBatchSummary) => void;
}) {
  const [localSelectedSummaryIds, setLocalSelectedSummaryIds] = useState<string[]>([]);
  const [editingSummaryId, setEditingSummaryId] = useState("");
  const [draftName, setDraftName] = useState("");
  const [bulkStoreId, setBulkStoreId] = useState("");
  const [bulkQueueFeedback, setBulkQueueFeedback] = useState("");
  const [statusFilter, setStatusFilter] = useState<RecentBatchStatusFilter>("all");
  const [activeRiskLabel, setActiveRiskLabel] = useState("");
  const selectedSummaryIds =
    controlledSelectedSummaryIds ?? localSelectedSummaryIds;
  const setSelectedSummaryIds =
    onSelectedSummaryIdsChange ?? setLocalSelectedSummaryIds;

  const selectedCount = selectedSummaryIds.length;
  const summaryById = useMemo(
    () =>
      new Map<string, SheinStudioRecentBatchSummary>(
        summaries.map((summary) => [`${summary.source}:${summary.id}`, summary]),
      ),
    [summaries],
  );
  const selectedPersistedBatchIds = useMemo(
    () =>
      selectedSummaryIds
        .map((key) => summaryById.get(key))
        .filter(
          (
            summary,
          ): summary is SheinStudioRecentBatchSummary & { source: "batch" } =>
            summary != null && summary.source === "batch",
        )
        .map((summary) => summary.id),
    [selectedSummaryIds, summaryById],
  );
  const selectedPersistedBatches = useMemo(
    () =>
      selectedSummaryIds
        .map((key) => summaryById.get(key))
        .filter(
          (
            summary,
          ): summary is SheinStudioRecentBatchSummary & { source: "batch" } =>
            summary != null && summary.source === "batch",
        ),
    [selectedSummaryIds, summaryById],
  );
  const selectedBatchesPendingGeneration = useMemo(
    () =>
      selectedPersistedBatches
        .filter((summary) => summary.designCount === 0)
        .map((summary) => summary.id),
    [selectedPersistedBatches],
  );
  const selectedBatchesPendingTaskCreation = useMemo(
    () =>
      selectedPersistedBatches
        .filter(
          (summary) =>
            summary.designCount > 0 && summary.createdTaskCount === 0,
        )
        .map((summary) => summary.id),
    [selectedPersistedBatches],
  );
  const selectedBatchesWithTasks = useMemo(
    () =>
      selectedPersistedBatches
        .filter((summary) => summary.createdTaskCount > 0)
        .map((summary) => summary.id),
    [selectedPersistedBatches],
  );
  const selectedRiskyBatches = useMemo(
    () => selectedPersistedBatches.filter((summary) => isRiskySummary(summary)),
    [selectedPersistedBatches],
  );
  const selectedHealthyBatches = useMemo(
    () => selectedPersistedBatches.filter((summary) => !isRiskySummary(summary)),
    [selectedPersistedBatches],
  );
  const selectedHealthyBatchesPendingGeneration = useMemo(
    () =>
      selectedHealthyBatches
        .filter((summary) => summary.designCount === 0)
        .map((summary) => summary.id),
    [selectedHealthyBatches],
  );
  const selectedHealthyBatchesPendingTaskCreation = useMemo(
    () =>
      selectedHealthyBatches
        .filter(
          (summary) =>
            summary.designCount > 0 && summary.createdTaskCount === 0,
        )
        .map((summary) => summary.id),
    [selectedHealthyBatches],
  );
  const selectedHealthyBatchesWithTasks = useMemo(
    () =>
      selectedHealthyBatches
        .filter((summary) => summary.createdTaskCount > 0)
        .map((summary) => summary.id),
    [selectedHealthyBatches],
  );

  function toggleSelection(summary: SheinStudioRecentBatchSummary) {
    const key = `${summary.source}:${summary.id}`;
    setBulkQueueFeedback("");
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
    setBulkQueueFeedback("");
    const ids = selectedSummaryIds
      .map((key) => summaryById.get(key))
      .filter((summary): summary is SheinStudioRecentBatchSummary => Boolean(summary))
      .map((summary) => summary.id);
    onBulkUpdateStore(ids, bulkStoreId);
  }

  function launchBulkQueue(
    batchIds: string[],
    mode: "generate" | "create_tasks",
    label: string,
  ) {
    if (!onOpenBatchQueue || batchIds.length === 0) {
      return;
    }
    const leftovers = [
      selectedBatchesPendingGeneration.length > 0 &&
      batchIds !== selectedBatchesPendingGeneration
        ? `另外还有 ${selectedBatchesPendingGeneration.length} 个待生成批次`
        : "",
      selectedBatchesPendingTaskCreation.length > 0 &&
      batchIds !== selectedBatchesPendingTaskCreation
        ? `另外还有 ${selectedBatchesPendingTaskCreation.length} 个待创建任务批次`
        : "",
      selectedBatchesWithTasks.length > 0 && batchIds !== selectedBatchesWithTasks
        ? `另外还有 ${selectedBatchesWithTasks.length} 个已有任务批次`
        : "",
    ].filter(Boolean);
    setBulkQueueFeedback(
      leftovers.length > 0
        ? `已为 ${batchIds.length} 个${label}启动处理队列。${leftovers.join("，")}可继续处理。`
        : `已为 ${batchIds.length} 个${label}启动处理队列。`,
    );
    onOpenBatchQueue({
      batchIds,
      mode,
    });
  }

  function primaryActionForSummary(summary: SheinStudioRecentBatchSummary): {
    action: RecentBatchCardAction;
    label: string;
  } {
    if (summary.createdTaskCount > 0) {
      return {
        action: "tasks",
        label: "查看任务",
      };
    }
    if (summary.designCount > 0) {
      return {
        action: "review",
        label: "去创建任务",
      };
    }
    return {
      action: "generate",
      label: "继续生成",
    };
  }

  const filteredSummaries = useMemo(
    () =>
      summaries.filter((summary) => {
        if (statusFilter === "risk") {
          if (!isRiskySummary(summary)) {
            return false;
          }
          return activeRiskLabel
            ? summary.alerts?.some((alert) => alert.label === activeRiskLabel)
            : true;
        }
        const action = primaryActionForSummary(summary).action;
        return statusFilter === "all" ? true : action === statusFilter;
      }),
    [activeRiskLabel, statusFilter, summaries],
  );
  const filterCounts = useMemo(
    () => ({
      all: summaries.length,
      generate: summaries.filter(
        (summary) => primaryActionForSummary(summary).action === "generate",
      ).length,
      review: summaries.filter(
        (summary) => primaryActionForSummary(summary).action === "review",
      ).length,
      tasks: summaries.filter(
        (summary) => primaryActionForSummary(summary).action === "tasks",
      ).length,
      risk: summaries.filter((summary) => isRiskySummary(summary)).length,
    }),
    [summaries],
  );
  const filterOptions: Array<{
    value: RecentBatchStatusFilter;
    label: string;
    count: number;
  }> = [
    { value: "all", label: "全部", count: filterCounts.all },
    { value: "generate", label: "待生成", count: filterCounts.generate },
    { value: "review", label: "待创建任务", count: filterCounts.review },
    { value: "tasks", label: "已有任务", count: filterCounts.tasks },
    { value: "risk", label: "有风险", count: filterCounts.risk },
  ];
  const riskCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const summary of summaries) {
      for (const alert of summary.alerts ?? []) {
        counts.set(alert.label, (counts.get(alert.label) ?? 0) + 1);
      }
    }
    return Array.from(counts.entries())
      .map(([label, count]) => ({ label, count }))
      .sort((left, right) => right.count - left.count || left.label.localeCompare(right.label));
  }, [summaries]);

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
            <div className="space-y-1">
              <div className="text-sm font-medium text-zinc-900">
                已选择 {selectedCount} 个批次
              </div>
              <div className="text-xs text-zinc-500">
                待生成 {selectedBatchesPendingGeneration.length} 个 / 待创建任务{" "}
                {selectedBatchesPendingTaskCreation.length} 个 / 已有任务{" "}
                {selectedBatchesWithTasks.length} 个
              </div>
            </div>
            <Button
              onClick={() => {
                setSelectedSummaryIds([]);
                setBulkQueueFeedback("");
              }}
              size="sm"
              type="button"
              variant="ghost"
            >
              清除选择
            </Button>
          </div>
          {selectedRiskyBatches.length > 0 ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-sm text-amber-900">
              本次选择里有 {selectedRiskyBatches.length} 个风险批次，建议先处理后再进入队列。
            </div>
          ) : null}
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
            {selectedPersistedBatchIds.length > 0 && onOpenBatchQueue ? (
              <>
                {selectedBatchesPendingGeneration.length > 0 ? (
                  <Button
                    onClick={() =>
                      launchBulkQueue(
                        selectedBatchesPendingGeneration,
                        "generate",
                        "待生成批次",
                      )
                    }
                    type="button"
                    variant="secondary"
                  >
                    批量继续生成 {selectedBatchesPendingGeneration.length} 个
                  </Button>
                ) : null}
                {selectedBatchesPendingTaskCreation.length > 0 ? (
                  <Button
                    onClick={() =>
                      launchBulkQueue(
                        selectedBatchesPendingTaskCreation,
                        "create_tasks",
                        "待创建任务批次",
                      )
                    }
                    type="button"
                    variant="secondary"
                  >
                    批量去创建任务 {selectedBatchesPendingTaskCreation.length} 个
                  </Button>
                ) : null}
                {selectedBatchesWithTasks.length > 0 ? (
                  <Button
                    onClick={() =>
                      launchBulkQueue(
                        selectedBatchesWithTasks,
                        "create_tasks",
                        "已有任务批次",
                      )
                    }
                    type="button"
                    variant="secondary"
                  >
                    批量查看任务 {selectedBatchesWithTasks.length} 个
                  </Button>
                ) : null}
                {selectedRiskyBatches.length > 0 ? (
                  <>
                    {selectedHealthyBatchesPendingGeneration.length > 0 ? (
                      <Button
                        onClick={() =>
                          launchBulkQueue(
                            selectedHealthyBatchesPendingGeneration,
                            "generate",
                            "可继续批次",
                          )
                        }
                        type="button"
                        variant="secondary"
                      >
                        仅处理可继续批次 {selectedHealthyBatchesPendingGeneration.length} 个
                      </Button>
                    ) : null}
                    {selectedHealthyBatchesPendingTaskCreation.length > 0 ? (
                      <Button
                        onClick={() =>
                          launchBulkQueue(
                            selectedHealthyBatchesPendingTaskCreation,
                            "create_tasks",
                            "可继续批次",
                          )
                        }
                        type="button"
                        variant="secondary"
                      >
                        仅处理可继续批次 {selectedHealthyBatchesPendingTaskCreation.length} 个
                      </Button>
                    ) : null}
                    {selectedHealthyBatchesWithTasks.length > 0 ? (
                      <Button
                        onClick={() =>
                          launchBulkQueue(
                            selectedHealthyBatchesWithTasks,
                            "create_tasks",
                            "可继续批次",
                          )
                        }
                        type="button"
                        variant="secondary"
                      >
                        仅处理可继续批次 {selectedHealthyBatchesWithTasks.length} 个
                      </Button>
                    ) : null}
                  </>
                ) : null}
              </>
            ) : null}
          </div>
          {bulkQueueFeedback ? (
            <div className="rounded-2xl border border-emerald-200 bg-emerald-50/70 px-3 py-3 text-sm text-emerald-900">
              {bulkQueueFeedback}
            </div>
          ) : null}
        </div>
      ) : null}

      {summaries.length > 0 ? (
        <div className="space-y-3 rounded-3xl border border-zinc-200/80 bg-zinc-50/80 px-4 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="text-xs font-medium text-zinc-900">按状态筛选</p>
              <p className="mt-1 text-xs text-zinc-500">
                当前显示 {filteredSummaries.length} / {summaries.length} 个批次
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              {filterOptions.map((option) => (
                <Button
                  key={option.value}
                  onClick={() => {
                    setStatusFilter(option.value);
                    if (option.value !== "risk") {
                      setActiveRiskLabel("");
                    }
                  }}
                  size="sm"
                  type="button"
                  variant={statusFilter === option.value ? "default" : "secondary"}
                >
                  {option.label} {option.count}
                </Button>
              ))}
            </div>
          </div>
          {riskCounts.length > 0 ? (
            <div className="flex flex-wrap gap-2 border-t border-zinc-200/80 pt-3">
              {riskCounts.map((item) => (
                <Button
                  key={item.label}
                  onClick={() => {
                    setStatusFilter("risk");
                    setActiveRiskLabel(item.label);
                  }}
                  size="sm"
                  type="button"
                  variant={
                    statusFilter === "risk" && activeRiskLabel === item.label
                      ? "default"
                      : "secondary"
                  }
                >
                  {item.label} {item.count}
                </Button>
              ))}
            </div>
          ) : null}
          {statusFilter === "risk" && activeRiskLabel ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-sm text-amber-900">
              当前只显示包含“{activeRiskLabel}”的风险批次。
            </div>
          ) : null}
        </div>
      ) : null}

      {summaries.length === 0 ? (
        <div className="rounded-3xl border border-dashed border-zinc-200 bg-zinc-50 px-5 py-8 text-sm text-zinc-600">
          还没有可继续的批次。先在选品区选择 SDS 商品，创建第一批内容。
        </div>
      ) : filteredSummaries.length === 0 ? (
        <div className="rounded-3xl border border-dashed border-zinc-200 bg-zinc-50 px-5 py-8 text-sm text-zinc-600">
          当前筛选下还没有匹配的批次。可以切回其他状态继续查看。
        </div>
      ) : (
        <div className="grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
          {filteredSummaries.map((summary) => {
            const summaryKey = `${summary.source}:${summary.id}`;
            const isSelected = selectedSummaryIds.includes(summaryKey);
            const isEditing = editingSummaryId === summaryKey;
            const hasDesigns = summary.designCount > 0;
            const hasTasks = summary.createdTaskCount > 0;
            const primaryAction = primaryActionForSummary(summary);
            const riskActions =
              summary.alerts
                ?.map((alert) => actionForRiskAlert(alert.label))
                .filter((value): value is RecentBatchAlertAction => value != null) ?? [];
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
                  <div className="mt-3">
                    <div
                      className="w-full cursor-pointer text-left"
                      onClick={() => onSelectSummary(summary)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          onSelectSummary(summary);
                        }
                      }}
                      role="button"
                      tabIndex={0}
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
                    <div className="mt-3 flex flex-wrap gap-2 text-[11px] font-medium">
                      <span
                        className={`rounded-full px-2.5 py-1 ${
                          hasDesigns
                            ? "bg-emerald-100 text-emerald-700"
                            : "bg-amber-100 text-amber-700"
                        }`}
                      >
                        {hasDesigns
                          ? `已有 ${summary.designCount} 张设计`
                          : "待生成设计"}
                      </span>
                      <span
                        className={`rounded-full px-2.5 py-1 ${
                          hasTasks
                            ? "bg-sky-100 text-sky-700"
                            : "bg-zinc-200 text-zinc-700"
                        }`}
                      >
                        {hasTasks
                          ? `已建 ${summary.createdTaskCount} 个任务`
                          : "待创建任务"}
                      </span>
                      <span className="rounded-full bg-zinc-200 px-2.5 py-1 text-zinc-700">
                        更新于 {formatRecentBatchTimestamp(summary.updatedAt)}
                      </span>
                    </div>
                    {summary.alerts?.length ? (
                      <div className="mt-3 flex flex-wrap gap-2 text-[11px] font-medium">
                        {summary.alerts.map((alert, index) => (
                          <span
                            className={`rounded-full border px-2.5 py-1 ${recentBatchAlertToneClass(alert.tone)}`}
                            key={`${summaryKey}:alert:${index}`}
                            title={alert.detail}
                          >
                            {alert.label}
                          </span>
                        ))}
                      </div>
                    ) : null}
                    {riskActions.length > 0 ? (
                      <div className="mt-3 flex flex-wrap gap-2">
                        {riskActions.map((riskAction, index) => (
                          <Button
                            key={`${summaryKey}:risk:${riskAction.label}:${index}`}
                            onClick={(event) => {
                              event.stopPropagation();
                              onSelectSummaryAction?.(summary, riskAction.action);
                            }}
                            size="sm"
                            type="button"
                            variant="secondary"
                          >
                            {riskAction.label}
                          </Button>
                        ))}
                      </div>
                    ) : null}
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
                    <div className="mt-4 rounded-2xl border border-zinc-200/80 bg-white/70 px-3 py-3">
                      <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
                        最近提示词
                      </p>
                      <p className="mt-2 line-clamp-2 text-sm text-zinc-600">
                        {summary.promptPreview}
                      </p>
                    </div>
                    </div>
                    <div className="mt-4 flex flex-wrap gap-2">
                      <Button
                        onClick={() => {
                          onSelectSummaryAction?.(summary, primaryAction.action);
                        }}
                        size="sm"
                        type="button"
                      >
                        {primaryAction.label}
                      </Button>
                      <Button
                        onClick={() => {
                          onSelectSummary(summary);
                        }}
                        size="sm"
                        type="button"
                        variant="ghost"
                      >
                        打开批次
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
