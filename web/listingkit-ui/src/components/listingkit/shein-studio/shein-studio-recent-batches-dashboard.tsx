"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  dispatchSheinStudioRecentBatchesRecommendation,
  SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT,
  SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT,
  type SheinStudioRecentBatchesFocusDetail,
  type SheinStudioRecentBatchesRecommendationDetail,
} from "@/lib/shein-studio/recent-batches-focus";
import { getSDSBaselineReasonShortLabel } from "@/lib/shein-studio/sds-baseline-ui";
import type { SheinStudioRecentBatchSummary } from "@/lib/types/shein-studio";

type StoreOption = {
  id: string;
  label: string;
};

type RecentBatchCardAction = "generate" | "review" | "tasks";
type RecentBatchStatusFilter = "all" | "generate" | "review" | "tasks" | "risk";
type RecentBatchResultFilter = "all" | "success" | "failure";

type RecentBatchAlertAction = {
  action: RecentBatchCardAction;
  label: string;
};

function summaryHasPendingGeneration(summary: SheinStudioRecentBatchSummary) {
  return (
    summary.batchStatus === "draft" ||
    summary.batchStatus === "generating" ||
    summary.batchStatus === "partially_materialized" ||
    summary.batchStatus === "partially_failed" ||
    summary.batchStatus === "failed"
  );
}

const DASHBOARD_PREFERENCES_STORAGE_KEY =
  "listingkit:shein-studio:recent-batches-dashboard";

type RecentBatchesDashboardPreferences = {
  statusFilter?: RecentBatchStatusFilter;
  resultFilter?: RecentBatchResultFilter;
  activeRiskLabel?: string;
  activeRiskReasonCode?: string;
  selectedSummaryIds?: string[];
  lastBulkActionSummary?: string;
};

function readStoredDashboardPreferences() {
  if (typeof window === "undefined") {
    return {} as RecentBatchesDashboardPreferences;
  }
  try {
    const raw = window.localStorage.getItem(DASHBOARD_PREFERENCES_STORAGE_KEY);
    return raw ? (JSON.parse(raw) as RecentBatchesDashboardPreferences) : {};
  } catch {
    return {} as RecentBatchesDashboardPreferences;
  }
}

function recentBatchAlertToneClass(tone: "warning" | "danger") {
  return tone === "danger"
    ? "border-rose-200 bg-rose-50 text-rose-700"
    : "border-amber-200 bg-amber-50 text-amber-700";
}

function recentBatchResultToneClass(tone: "success" | "warning" | "danger") {
  switch (tone) {
    case "success":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "danger":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-amber-200 bg-amber-50 text-amber-700";
  }
}

function isBaselineRiskLabel(label: string) {
  return (
    label === "Baseline 未就绪" ||
    label === "Baseline 待校验" ||
    label === "Baseline 校验未通过"
  );
}

function actionForRiskAlert(label: string): RecentBatchAlertAction | null {
  if (isBaselineRiskLabel(label)) {
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

function summaryHasRecentResultTone(
  summary: SheinStudioRecentBatchSummary,
  tone: "success" | "danger",
) {
  return Boolean(summary.recentResults?.some((result) => result.tone === tone));
}

function riskLabelPriority(label: string) {
  switch (label) {
    case "Baseline 未就绪":
      return 0;
    case "Baseline 待校验":
      return 0;
    case "Baseline 校验未通过":
      return 0;
    case "生成失败":
      return 1;
    case "待确认款式":
      return 2;
    default:
      return 10;
  }
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

function resultFilterDescription(filter: RecentBatchResultFilter) {
  if (filter === "success") {
    return "当前只显示最近处理成功的批次。";
  }
  if (filter === "failure") {
    return "当前只显示最近处理失败的批次。";
  }
  return "";
}

function applyRiskFocus(
  label: string,
  setStatusFilter: (value: RecentBatchStatusFilter) => void,
  setActiveRiskLabel: (value: string) => void,
  setActiveRiskReasonCode: (value: string) => void,
  setFocusedRiskLabel: (value: string) => void,
) {
  setStatusFilter("risk");
  setActiveRiskLabel(label);
  setActiveRiskReasonCode("");
  setFocusedRiskLabel(label);
}

function restoredResultFilterDescription(filter: RecentBatchResultFilter) {
  if (filter === "success") {
    return "已恢复上次的最近处理成功视图。";
  }
  if (filter === "failure") {
    return "已恢复上次的最近处理失败视图。";
  }
  return "";
}

export function SheinStudioRecentBatchesDashboard({
  summaries,
  selectedSummaryIds: controlledSelectedSummaryIds,
  storeOptions = [],
  onBulkDeleteSummaries,
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
  onBulkDeleteSummaries?: (summaryIds: string[]) => void | Promise<void>;
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
  const router = useRouter();
  const initialPreferences = useMemo(() => readStoredDashboardPreferences(), []);
  const [localSelectedSummaryIds, setLocalSelectedSummaryIds] = useState<string[]>(
    () => initialPreferences.selectedSummaryIds ?? [],
  );
  const [editingSummaryId, setEditingSummaryId] = useState("");
  const [draftName, setDraftName] = useState("");
  const [bulkStoreId, setBulkStoreId] = useState("");
  const [bulkQueueFeedback, setBulkQueueFeedback] = useState("");
  const [lastBulkActionSummary, setLastBulkActionSummary] = useState(
    () => initialPreferences.lastBulkActionSummary?.trim() ?? "",
  );
  const [statusFilter, setStatusFilter] = useState<RecentBatchStatusFilter>(
    () => initialPreferences.statusFilter ?? "all",
  );
  const [resultFilter, setResultFilter] = useState<RecentBatchResultFilter>(
    () => initialPreferences.resultFilter ?? "all",
  );
  const [restoredResultFilterNote, setRestoredResultFilterNote] = useState(() =>
    restoredResultFilterDescription(initialPreferences.resultFilter ?? "all"),
  );
  const [activeRiskLabel, setActiveRiskLabel] = useState(
    () => initialPreferences.activeRiskLabel ?? "",
  );
  const [activeRiskReasonCode, setActiveRiskReasonCode] = useState(
    () => initialPreferences.activeRiskReasonCode ?? "",
  );
  const [focusedRiskLabel, setFocusedRiskLabel] = useState("");
  const [previousSelectedSummaryIds, setPreviousSelectedSummaryIds] = useState<
    string[] | null
  >(null);
  const selectedSummaryIds =
    controlledSelectedSummaryIds ?? localSelectedSummaryIds;
  const setSelectedSummaryIds =
    onSelectedSummaryIdsChange ?? setLocalSelectedSummaryIds;

  useEffect(() => {
    const validKeys = new Set(
      summaries.map((summary) => `${summary.source}:${summary.id}`),
    );
    setSelectedSummaryIds((current) =>
      current.every((key) => validKeys.has(key))
        ? current
        : current.filter((key) => validKeys.has(key)),
    );
  }, [setSelectedSummaryIds, summaries]);

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
  const selectedRiskyBatchCount = selectedRiskyBatches.length;
  const selectedRiskyBatchesForGenerate = useMemo(
    () =>
      selectedRiskyBatches
        .filter((summary) =>
          summary.alerts?.some(
            (alert) =>
              isBaselineRiskLabel(alert.label) || alert.label === "生成失败",
          ),
        )
        .map((summary) => summary.id),
    [selectedRiskyBatches],
  );
  const selectedRiskyBatchesForReview = useMemo(
    () =>
      selectedRiskyBatches
        .filter((summary) =>
          summary.alerts?.some((alert) => alert.label === "待确认款式"),
        )
        .map((summary) => summary.id),
    [selectedRiskyBatches],
  );
  const selectedRiskCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const summary of selectedRiskyBatches) {
      for (const alert of summary.alerts ?? []) {
        counts.set(alert.label, (counts.get(alert.label) ?? 0) + 1);
      }
    }
    return Array.from(counts.entries())
      .map(([label, count]) => ({ label, count }))
      .sort(
        (left, right) =>
          riskLabelPriority(left.label) - riskLabelPriority(right.label) ||
          right.count - left.count ||
          left.label.localeCompare(right.label),
      );
  }, [selectedRiskyBatches]);
  const selectedRecentResultCounts = useMemo(() => {
    const success = selectedPersistedBatches.filter((summary) =>
      summaryHasRecentResultTone(summary, "success"),
    ).length;
    const failure = selectedPersistedBatches.filter((summary) =>
      summaryHasRecentResultTone(summary, "danger"),
    ).length;
    const other = Math.max(
      0,
      selectedPersistedBatches.length - success - failure,
    );
    return { success, failure, other };
  }, [selectedPersistedBatches]);

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

  async function applyBulkDelete() {
    if (!onBulkDeleteSummaries || selectedPersistedBatchIds.length === 0) {
      return;
    }
    setBulkQueueFeedback("");
    await Promise.resolve(onBulkDeleteSummaries(selectedPersistedBatchIds));
    setSelectedSummaryIds((current) =>
      current.filter((key) => {
        const summary = summaryById.get(key);
        return !summary || summary.source !== "batch"
          ? true
          : !selectedPersistedBatchIds.includes(summary.id);
      }),
    );
  }

  function replaceSelectedSummaries(
    summariesToKeep: SheinStudioRecentBatchSummary[],
  ) {
    setBulkQueueFeedback("");
    setPreviousSelectedSummaryIds(selectedSummaryIds);
    setSelectedSummaryIds(
      summariesToKeep.map((summary) => `${summary.source}:${summary.id}`),
    );
  }

  function restorePreviousSelectedSummaries() {
    if (!previousSelectedSummaryIds) {
      return;
    }
    setBulkQueueFeedback("");
    setSelectedSummaryIds(previousSelectedSummaryIds);
    setPreviousSelectedSummaryIds(null);
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
    const summary =
      leftovers.length > 0
        ? `已为 ${batchIds.length} 个${label}启动处理队列。${leftovers.join("，")}可继续处理。`
        : `已为 ${batchIds.length} 个${label}启动处理队列。`;
    setBulkQueueFeedback(summary);
    setLastBulkActionSummary(summary);
    onOpenBatchQueue({
      batchIds,
      mode,
    });
  }

  function launchRiskRepairQueue(
    batchIds: string[],
    mode: "generate" | "create_tasks",
    actionLabel: string,
    leftoverLabel: string,
  ) {
    if (!onOpenBatchQueue || batchIds.length === 0) {
      return;
    }
    const leftovers = [
      leftoverLabel === "待确认款式" && selectedRiskyBatchesForReview.length > 0
        ? `另外还有 ${selectedRiskyBatchesForReview.length} 个待确认款式风险批次可继续处理。`
        : "",
      leftoverLabel === "生成处理" && selectedRiskyBatchesForGenerate.length > 0
        ? `另外还有 ${selectedRiskyBatchesForGenerate.length} 个生成处理风险批次可继续处理。`
        : "",
    ].filter(Boolean);
    const summary =
      leftovers.length > 0
        ? `已为 ${batchIds.length} 个风险批次启动${actionLabel}队列。${leftovers.join("")}`
        : `已为 ${batchIds.length} 个风险批次启动${actionLabel}队列。`;
    setBulkQueueFeedback(summary);
    setLastBulkActionSummary(summary);
    onOpenBatchQueue({
      batchIds,
      mode,
    });
  }

function primaryActionForSummary(summary: SheinStudioRecentBatchSummary): {
  action: RecentBatchCardAction;
  label: string;
} {
  if (summary.isRecoverableDraft) {
    return {
      action:
        summary.createdTaskCount > 0
          ? "tasks"
          : summary.designCount > 0
            ? "review"
            : "generate",
      label: "恢复草稿",
    };
  }
  if (summaryHasPendingGeneration(summary)) {
    return {
      action: "generate",
      label: "继续生成",
    };
  }
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
            ? summary.alerts?.some(
                (alert) =>
                  alert.label === activeRiskLabel &&
                  (!activeRiskReasonCode ||
                    alert.reasonCode === activeRiskReasonCode),
              )
            : true;
        }
        const action = primaryActionForSummary(summary).action;
        if (statusFilter !== "all" && action !== statusFilter) {
          return false;
        }
        if (resultFilter === "success") {
          return summaryHasRecentResultTone(summary, "success");
        }
        if (resultFilter === "failure") {
          return summaryHasRecentResultTone(summary, "danger");
        }
        return true;
      }),
    [activeRiskLabel, activeRiskReasonCode, resultFilter, statusFilter, summaries],
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
  const resultFilterCounts = useMemo(
    () => ({
      all: summaries.length,
      success: summaries.filter((summary) =>
        summaryHasRecentResultTone(summary, "success"),
      ).length,
      failure: summaries.filter((summary) =>
        summaryHasRecentResultTone(summary, "danger"),
      ).length,
    }),
    [summaries],
  );
  const resultFilterOptions: Array<{
    value: RecentBatchResultFilter;
    label: string;
    count: number;
  }> = [
    { value: "all", label: "全部结果", count: resultFilterCounts.all },
    { value: "success", label: "最近成功", count: resultFilterCounts.success },
    { value: "failure", label: "最近失败", count: resultFilterCounts.failure },
  ];
  const openSummaryRoute = (summary: SheinStudioRecentBatchSummary) => {
    if (summary.source !== "batch") {
      return false;
    }
    router.push(`/listing-kits/sds/batches/${summary.id}`);
    return true;
  };
  const riskCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const summary of summaries) {
      for (const alert of summary.alerts ?? []) {
        counts.set(alert.label, (counts.get(alert.label) ?? 0) + 1);
      }
    }
    return Array.from(counts.entries())
      .map(([label, count]) => ({ label, count }))
      .sort(
        (left, right) =>
          riskLabelPriority(left.label) - riskLabelPriority(right.label) ||
          right.count - left.count ||
          left.label.localeCompare(right.label),
      );
  }, [summaries]);
  const recommendedRiskReasonCode = useMemo(() => {
    const topRiskLabel = riskCounts[0]?.label ?? "";
    if (!isBaselineRiskLabel(topRiskLabel)) {
      return "";
    }
    const counts = new Map<string, number>();
    for (const summary of summaries) {
      for (const alert of summary.alerts ?? []) {
        if (alert.label !== topRiskLabel || !alert.reasonCode?.trim()) {
          continue;
        }
        counts.set(alert.reasonCode, (counts.get(alert.reasonCode) ?? 0) + 1);
      }
    }
    return Array.from(counts.entries()).sort(
      (left, right) => right[1] - left[1] || left[0].localeCompare(right[0]),
    )[0]?.[0] ?? "";
  }, [riskCounts, summaries]);
  const activeRiskReasonCounts = useMemo(() => {
    if (statusFilter !== "risk" || !isBaselineRiskLabel(activeRiskLabel)) {
      return [];
    }
    const counts = new Map<string, number>();
    for (const summary of summaries) {
      for (const alert of summary.alerts ?? []) {
        if (alert.label !== activeRiskLabel || !alert.reasonCode?.trim()) {
          continue;
        }
        counts.set(alert.reasonCode, (counts.get(alert.reasonCode) ?? 0) + 1);
      }
    }
    return Array.from(counts.entries())
      .map(([reasonCode, count]) => ({
        reasonCode,
        count,
        label:
          getSDSBaselineReasonShortLabel(reasonCode) || reasonCode.replaceAll("_", " "),
      }))
      .sort(
        (left, right) =>
          right.count - left.count || left.label.localeCompare(right.label),
      );
  }, [activeRiskLabel, statusFilter, summaries]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const payload: RecentBatchesDashboardPreferences = {
      statusFilter,
      resultFilter,
      activeRiskLabel,
      activeRiskReasonCode,
      selectedSummaryIds,
      lastBulkActionSummary,
    };
    window.localStorage.setItem(
      DASHBOARD_PREFERENCES_STORAGE_KEY,
      JSON.stringify(payload),
    );
  }, [
    activeRiskLabel,
    activeRiskReasonCode,
    lastBulkActionSummary,
    resultFilter,
    selectedSummaryIds,
    statusFilter,
  ]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const handleFocus = (event: Event) => {
      const detail = (event as CustomEvent<SheinStudioRecentBatchesFocusDetail>)
        .detail;
      if (!detail?.preferRisk) {
        return;
      }
      if (riskCounts.length === 0) {
        return;
      }
      setStatusFilter("risk");
      setActiveRiskLabel("");
      setActiveRiskReasonCode("");
      setFocusedRiskLabel(riskCounts[0]?.label ?? "");
      setRestoredResultFilterNote("");
    };
    window.addEventListener(
      SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT,
      handleFocus as EventListener,
    );
    return () => {
      window.removeEventListener(
        SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT,
        handleFocus as EventListener,
      );
    };
  }, [riskCounts]);
  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    dispatchSheinStudioRecentBatchesRecommendation({
      hasRecoverableBatches: summaries.length > 0,
      recommendedRiskLabel: riskCounts[0]?.label ?? "",
      recommendedRiskReasonCode,
    });
  }, [recommendedRiskReasonCode, riskCounts, summaries.length]);

  return (
    <section className="space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm">
      <div className="flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-start sm:justify-between">
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
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:flex-wrap">
          {summaries.length > 0 ? (
            <Button
              className="w-full sm:w-auto"
              onClick={() => onSelectSummary(summaries[0])}
              type="button"
              variant="secondary"
            >
              继续最近批次
            </Button>
          ) : null}
          <Button className="w-full sm:w-auto" onClick={onCreateBatch} type="button">
            {summaries.length === 0 ? "开始新建批次并选品" : "新建批次"}
          </Button>
        </div>
      </div>

      {lastBulkActionSummary && lastBulkActionSummary !== bulkQueueFeedback ? (
        <div className="rounded-2xl border border-sky-200 bg-sky-50/80 px-4 py-3 text-sm text-sky-900">
          {lastBulkActionSummary}
        </div>
      ) : null}

      {selectedCount > 0 ? (
        <div className="space-y-3 rounded-3xl border border-zinc-200 bg-zinc-50 px-4 py-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
            <div className="space-y-1">
              <div className="text-sm font-medium text-zinc-900">
                已选择 {selectedCount} 个批次
              </div>
              <div className="text-xs text-zinc-500">
                待生成 {selectedBatchesPendingGeneration.length} 个 / 待创建任务{" "}
                {selectedBatchesPendingTaskCreation.length} 个 / 已有任务{" "}
                {selectedBatchesWithTasks.length} 个
              </div>
              <div className="text-xs text-zinc-500">
                最近成功 {selectedRecentResultCounts.success} 个 / 最近失败{" "}
                {selectedRecentResultCounts.failure} 个 / 其他{" "}
                {selectedRecentResultCounts.other} 个
              </div>
            </div>
            <Button
              className="w-full sm:w-auto"
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
          {(selectedRiskyBatchCount > 0 || selectedHealthyBatches.length > 0) && (
            <div className="flex flex-wrap gap-2">
              {selectedRiskyBatchCount > 0 ? (
                <Button
                  onClick={() => replaceSelectedSummaries(selectedRiskyBatches)}
                  size="sm"
                  type="button"
                  variant="secondary"
                >
                  仅保留风险批次 {selectedRiskyBatchCount} 个
                </Button>
              ) : null}
              {selectedHealthyBatches.length > 0 ? (
                <Button
                  onClick={() => replaceSelectedSummaries(selectedHealthyBatches)}
                  size="sm"
                  type="button"
                  variant="secondary"
                >
                  仅保留可继续批次 {selectedHealthyBatches.length} 个
                </Button>
              ) : null}
              {previousSelectedSummaryIds?.length ? (
                <Button
                  onClick={restorePreviousSelectedSummaries}
                  size="sm"
                  type="button"
                  variant="secondary"
                >
                  恢复上一次选择 {previousSelectedSummaryIds.length} 个
                </Button>
              ) : null}
            </div>
          )}
          {selectedRiskyBatchCount > 0 ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-sm text-amber-900">
              本次选择里有 {selectedRiskyBatchCount} 个风险批次，建议先处理后再进入队列。
              {selectedRiskCounts.length > 0 ? (
                <div className="mt-2 text-xs text-amber-800">
                  风险拆分：
                  {selectedRiskCounts
                    .map((item) => `${item.label} ${item.count} 个`)
                    .join(" / ")}
                </div>
              ) : null}
            </div>
          ) : null}
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end">
            <label className="w-full text-sm text-zinc-600 sm:max-w-xs">
              <span className="mb-1 block">目标店铺</span>
              <select
                aria-label="目标店铺"
                className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900"
                onChange={(event) => setBulkStoreId(event.target.value)}
                value={bulkStoreId}
              >
                <option value="">跟随批次店铺</option>
                {storeOptions.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <Button
              className="w-full sm:w-auto"
              disabled={!onBulkUpdateStore}
              onClick={applyBulkStoreUpdate}
              type="button"
            >
              应用到已选批次
            </Button>
            {selectedPersistedBatchIds.length > 0 && onBulkDeleteSummaries ? (
              <Button
                className="w-full sm:w-auto"
                onClick={applyBulkDelete}
                type="button"
                variant="secondary"
              >
                批量删除 {selectedPersistedBatchIds.length} 个
              </Button>
            ) : null}
            {selectedPersistedBatchIds.length > 0 && onOpenBatchQueue ? (
              <>
                {selectedBatchesPendingGeneration.length > 0 ? (
                  <Button
                    className="w-full sm:w-auto"
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
                    className="w-full sm:w-auto"
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
                    className="w-full sm:w-auto"
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
                {selectedRiskyBatchCount > 0 ? (
                  <>
                    {selectedRiskyBatchesForGenerate.length > 0 ? (
                      <Button
                        className="w-full sm:w-auto"
                        onClick={() =>
                          launchRiskRepairQueue(
                            selectedRiskyBatchesForGenerate,
                            "generate",
                            "生成处理",
                            "待确认款式",
                          )
                        }
                        type="button"
                        variant="secondary"
                      >
                        批量去生成区处理 {selectedRiskyBatchesForGenerate.length} 个
                      </Button>
                    ) : null}
                    {selectedRiskyBatchesForReview.length > 0 ? (
                      <Button
                        className="w-full sm:w-auto"
                        onClick={() =>
                          launchRiskRepairQueue(
                            selectedRiskyBatchesForReview,
                            "create_tasks",
                            "确认设计",
                            "生成处理",
                          )
                        }
                        type="button"
                        variant="secondary"
                      >
                        批量去确认设计 {selectedRiskyBatchesForReview.length} 个
                      </Button>
                    ) : null}
                    {selectedHealthyBatchesPendingGeneration.length > 0 ? (
                      <Button
                        className="w-full sm:w-auto"
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
                        className="w-full sm:w-auto"
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
                        className="w-full sm:w-auto"
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
        <div className="space-y-3 rounded-3xl border border-border/80 bg-muted/80 px-4 py-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
            <div>
              <p className="text-xs font-medium text-foreground">按状态筛选</p>
              <p className="mt-1 text-xs text-muted-foreground">
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
                        setActiveRiskReasonCode("");
                        setFocusedRiskLabel("");
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
            <div className="flex flex-wrap gap-2 border-t border-border/80 pt-3">
              {riskCounts.map((item) => (
                <Button
                  key={item.label}
                  onClick={() => {
                  applyRiskFocus(
                    item.label,
                    setStatusFilter,
                    setActiveRiskLabel,
                    setActiveRiskReasonCode,
                    setFocusedRiskLabel,
                  );
                  }}
                  className={
                    statusFilter === "risk" &&
                    !activeRiskLabel &&
                    focusedRiskLabel === item.label
                      ? "ring-2 ring-amber-300 ring-offset-1"
                      : undefined
                  }
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
          <div className="flex flex-wrap gap-2 border-t border-border/80 pt-3">
            {resultFilterOptions.map((option) => (
              <Button
                key={option.value}
                onClick={() => {
                  setResultFilter(option.value);
                  setRestoredResultFilterNote("");
                }}
                size="sm"
                type="button"
                variant={resultFilter === option.value ? "default" : "secondary"}
              >
                {option.label} {option.count}
              </Button>
            ))}
          </div>
          {statusFilter === "risk" && activeRiskLabel ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-sm text-amber-900">
              当前只显示包含“{activeRiskLabel}”的风险批次。
              {activeRiskReasonCode
                ? ` 已细分到“${getSDSBaselineReasonShortLabel(activeRiskReasonCode) || activeRiskReasonCode}”。`
                : ""}
            </div>
          ) : null}
          {statusFilter === "risk" &&
          activeRiskLabel &&
          activeRiskReasonCounts.length > 0 ? (
            <div className="flex flex-wrap gap-2 border-t border-border/80 pt-3">
              {activeRiskReasonCounts.map((item) => (
                <Button
                  key={item.reasonCode}
                  onClick={() => {
                    setActiveRiskReasonCode((current) =>
                      current === item.reasonCode ? "" : item.reasonCode,
                    );
                  }}
                  size="sm"
                  type="button"
                  variant={
                    activeRiskReasonCode === item.reasonCode ? "default" : "secondary"
                  }
                >
                  {item.label} {item.count}
                </Button>
              ))}
            </div>
          ) : null}
          {statusFilter === "risk" && !activeRiskLabel && focusedRiskLabel ? (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-sm text-amber-900">
              <span>
                已优先切到风险视图，建议先处理“{focusedRiskLabel}”相关批次。
              </span>
              <Button
                onClick={() =>
                    applyRiskFocus(
                      focusedRiskLabel,
                      setStatusFilter,
                      setActiveRiskLabel,
                      setActiveRiskReasonCode,
                      setFocusedRiskLabel,
                    )
                  }
                size="sm"
                type="button"
                variant="secondary"
              >
                只看这一类风险
              </Button>
            </div>
          ) : null}
          {resultFilter !== "all" ? (
            <div className="rounded-2xl border border-sky-200 bg-sky-50/80 px-3 py-3 text-sm text-sky-900">
              {resultFilterDescription(resultFilter)}
            </div>
          ) : null}
          {restoredResultFilterNote ? (
            <div className="rounded-2xl border border-border bg-background px-3 py-3 text-sm text-foreground">
              {restoredResultFilterNote}
            </div>
          ) : null}
        </div>
      ) : null}

      {summaries.length === 0 ? (
        <div className="rounded-3xl border border-dashed border-border bg-muted px-5 py-8 text-sm text-muted-foreground">
          <p>还没有可继续的最近批次，建议先新建一个批次再开始选品。</p>
          <div className="mt-4">
            <Button onClick={onCreateBatch} type="button">
              开始新建批次并选品
            </Button>
          </div>
        </div>
      ) : filteredSummaries.length === 0 ? (
        <div className="rounded-3xl border border-dashed border-border bg-muted px-5 py-8 text-sm text-muted-foreground">
          当前筛选下还没有匹配的批次。可以切回其他状态继续查看。
        </div>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
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
                    : "border-border bg-muted"
                }`}
                key={summaryKey}
              >
                <div className="flex items-start justify-between gap-3">
                  <label className="inline-flex items-center gap-2 text-xs text-muted-foreground">
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
                    {onDeleteSummary ? (
                      <Button
                        onClick={() => onDeleteSummary(summary)}
                        size="sm"
                        type="button"
                        variant="ghost"
                      >
                        {summary.isRecoverableDraft ? "删除草稿" : "删除"}
                      </Button>
                    ) : null}
                  </div>
                </div>

                {isEditing ? (
                  <div className="mt-3 space-y-2">
                    <label className="block text-sm text-muted-foreground">
                      <span className="mb-1 block">批次名称</span>
                      <input
                        aria-label="批次名称"
                        className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm text-foreground"
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
                    <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-foreground">
                          {summary.title}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {summary.isRecoverableDraft ? "未保存草稿" : "已保存批次"}
                        </p>
                      </div>
                      <span className="rounded-full bg-primary px-2.5 py-1 text-[11px] font-medium text-primary-foreground">
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
                            : "bg-muted text-muted-foreground"
                        }`}
                      >
                        {hasTasks
                          ? `已建 ${summary.createdTaskCount} 个任务`
                          : "待创建任务"}
                      </span>
                      <span className="rounded-full bg-muted px-2.5 py-1 text-muted-foreground">
                        更新于 {formatRecentBatchTimestamp(summary.updatedAt)}
                      </span>
                    </div>
                    {summary.alerts?.length ? (
                      <>
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
                        {summary.alerts.some((alert) => alert.detail?.trim()) ? (
                          <div className="mt-3 space-y-1 text-xs text-muted-foreground">
                            {summary.alerts.map((alert, index) =>
                              alert.detail?.trim() ? (
                                <p key={`${summaryKey}:alert-detail:${index}`}>
                                  {alert.label}：{alert.detail.trim()}
                                </p>
                              ) : null,
                            )}
                          </div>
                        ) : null}
                      </>
                    ) : null}
                    {riskActions.length > 0 ? (
                      <div className="mt-3 flex flex-wrap gap-2">
                        {riskActions.map((riskAction, index) => (
                          <Button
                            key={`${summaryKey}:risk:${riskAction.label}:${index}`}
                            onClick={(event) => {
                              event.stopPropagation();
                              if (openSummaryRoute(summary)) {
                                return;
                              }
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
                    <dl className="mt-4 space-y-2 text-sm text-foreground">
                      <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
                        <dt className="text-muted-foreground">商品</dt>
                        <dd className="break-words text-left sm:text-right">{summary.primaryProductName}</dd>
                      </div>
                      <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
                        <dt className="text-muted-foreground">店铺分发</dt>
                        <dd className="break-words text-left sm:text-right">{summary.storeSummary}</dd>
                      </div>
                      <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
                        <dt className="text-muted-foreground">设计/任务</dt>
                        <dd className="text-left sm:text-right">
                          {summary.designCount} 图 / {summary.createdTaskCount} 任务
                        </dd>
                      </div>
                    </dl>
                    <div className="mt-4 rounded-2xl border border-border/80 bg-background/70 px-3 py-3">
                      <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">
                        最近提示词
                      </p>
                      <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">
                        {summary.promptPreview}
                      </p>
                    </div>
                    {summary.recentResults?.length ? (
                      <div className="mt-4 rounded-2xl border border-border/80 bg-background/70 px-3 py-3">
                        <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">
                          最近处理结果
                        </p>
                        <div className="mt-2 space-y-2">
                          {summary.recentResults.map((result, index) => (
                            <div
                              className={`rounded-2xl border px-3 py-2 ${recentBatchResultToneClass(result.tone)}`}
                              key={`${summaryKey}:result:${result.label}:${index}`}
                            >
                              <p className="text-sm font-medium">{result.label}</p>
                              {result.detail?.trim() ? (
                                <p className="mt-1 text-xs">{result.detail.trim()}</p>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      </div>
                    ) : null}
                    </div>
                    <div className="mt-4 flex flex-wrap gap-2">
                      <Button
                        onClick={() => {
                          if (openSummaryRoute(summary)) {
                            return;
                          }
                          onSelectSummaryAction?.(summary, primaryAction.action);
                        }}
                        size="sm"
                        type="button"
                      >
                        {primaryAction.label}
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
