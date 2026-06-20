"use client";

import { useCallback, useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  cancelSheinStudioBatchRun,
  getSheinStudioBatchRun,
  listSheinStudioBatchRunItems,
  recoverSheinStudioBatchRun,
} from "@/lib/api/shein-studio-batch-runs";
import type {
  SheinStudioBatchRun,
  SheinStudioBatchRunItem,
  SheinStudioBatchRunItemStatus,
  SheinStudioBatchRunStatus,
} from "@/lib/types/shein-studio-batch-runs";

const TERMINAL_BATCH_RUN_STATUSES = new Set<SheinStudioBatchRunStatus>([
  "succeeded",
  "partially_succeeded",
  "failed",
  "cancelled",
]);

function isTerminalBatchRunStatus(status?: SheinStudioBatchRunStatus) {
  return status ? TERMINAL_BATCH_RUN_STATUSES.has(status) : false;
}

function isCancellingBatchRun(run?: Pick<SheinStudioBatchRun, "status" | "cancelRequested"> | null) {
  return Boolean(run?.cancelRequested && !isTerminalBatchRunStatus(run.status));
}

function canRecoverBatchRun(
  run?: Pick<SheinStudioBatchRun, "status" | "cancelRequested"> | null,
) {
  if (!run || run.cancelRequested) {
    return false;
  }
  return run.status === "failed" || run.status === "cancelled" || run.status === "partially_succeeded";
}

function formatBatchRunStatus(
  status?: SheinStudioBatchRunStatus,
  cancelRequested?: boolean,
) {
  if (cancelRequested && !isTerminalBatchRunStatus(status)) {
    return "取消中";
  }
  switch (status) {
    case "pending":
      return "等待开始";
    case "running":
      return "生成中";
    case "succeeded":
      return "已完成";
    case "partially_succeeded":
      return "部分完成";
    case "failed":
      return "执行失败";
    case "cancelled":
      return "已取消";
    default:
      return "状态未知";
  }
}

function progressHeading(
  mode?: SheinStudioBatchRun["mode"],
  status?: SheinStudioBatchRunStatus,
  cancelRequested?: boolean,
) {
  const actionLabel = mode === "create_tasks" ? "批量创建任务" : "批量生成";
  if (cancelRequested && !isTerminalBatchRunStatus(status)) {
    return `正在取消${actionLabel}`;
  }
  if (status === "succeeded" || status === "partially_succeeded") {
    return `${actionLabel}结果`;
  }
  if (status === "failed" || status === "cancelled") {
    return `${actionLabel}已结束`;
  }
  return `运行中${actionLabel}`;
}

function runActionLabel(mode?: SheinStudioBatchRun["mode"]) {
  return mode === "create_tasks" ? "任务创建" : "生成";
}

function getBatchRunFailureMessage(error: unknown) {
  return error instanceof Error
    ? error.message
    : "批量生成状态暂时没有成功同步，请稍后重试。";
}

function formatBatchItemStatus(status?: string) {
  switch (status) {
    case "pending":
      return "待提交";
    case "submitted":
      return "已提交";
    case "processing":
      return "处理中";
    case "succeeded":
      return "已完成";
    case "failed":
      return "执行失败";
    case "cancelled":
      return "已取消";
    default:
      return status ?? "状态未知";
  }
}

function getItemFailureReason(item: SheinStudioBatchRunItem) {
  return item.errorMessage || item.batchLastError;
}

export function SheinStudioBatchRunProgress({
  runId,
  onBack,
}: {
  runId: string;
  onBack: () => void;
}) {
  const [run, setRun] = useState<SheinStudioBatchRun | null>(null);
  const [items, setItems] = useState<SheinStudioBatchRunItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isCancelling, setIsCancelling] = useState(false);
  const [isRecovering, setIsRecovering] = useState(false);
  const [error, setError] = useState("");
  const isCancellingRun = isCancellingBatchRun(run);
  const canRecoverRun = canRecoverBatchRun(run);

  const refreshRun = useCallback(async () => {
    setError("");
    const [nextRun, nextItems] = await Promise.all([
      getSheinStudioBatchRun(runId),
      listSheinStudioBatchRunItems(runId),
    ]);
    setRun(nextRun);
    setItems(nextItems);
    return nextRun;
  }, [runId]);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const sync = async (showLoading: boolean) => {
      if (showLoading) {
        setIsLoading(true);
      }
      try {
        const nextRun = await refreshRun();
        if (cancelled) {
          return;
        }
        if (!isTerminalBatchRunStatus(nextRun.status)) {
          timer = setTimeout(() => {
            void sync(false);
          }, 2_000);
        }
      } catch (nextError) {
        if (!cancelled) {
          setError(getBatchRunFailureMessage(nextError));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void sync(true);

    return () => {
      cancelled = true;
      if (timer) {
        clearTimeout(timer);
      }
    };
  }, [refreshRun]);

  const failedItems = items.filter(
    (item) => item.status === "failed" || item.status === "cancelled",
  );

  async function handleCancel() {
    setIsCancelling(true);
    setError("");
    try {
      await cancelSheinStudioBatchRun(runId);
      await refreshRun();
    } catch (nextError) {
      setError(getBatchRunFailureMessage(nextError));
    } finally {
      setIsCancelling(false);
    }
  }

  async function handleRecover() {
    setIsRecovering(true);
    setError("");
    try {
      await recoverSheinStudioBatchRun(runId);
      await refreshRun();
    } catch (nextError) {
      setError(getBatchRunFailureMessage(nextError));
    } finally {
      setIsRecovering(false);
    }
  }

  return (
    <section className="space-y-4 rounded-2xl border border-zinc-200 bg-white px-5 py-5 shadow-sm">
      <div className="flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-start sm:justify-between">
        <div className="space-y-1">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              STUDIO BATCH RUN
            </p>
          <h2 className="text-xl font-semibold tracking-tight text-zinc-950">
            {progressHeading(run?.mode, run?.status, run?.cancelRequested)}
          </h2>
          <p className="break-all text-sm text-zinc-600">
            当前运行 ID：{runId}
          </p>
        </div>
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:flex-wrap">
          {!isTerminalBatchRunStatus(run?.status) && !isCancellingRun ? (
            <Button
              className="w-full sm:w-auto"
              onClick={() => void handleCancel()}
              type="button"
              variant="secondary"
            >
              {isCancelling ? "正在取消..." : `取消本轮${runActionLabel(run?.mode)}`}
            </Button>
          ) : null}
          {canRecoverRun ? (
            <Button
              className="w-full sm:w-auto"
              disabled={isRecovering}
              onClick={() => void handleRecover()}
              type="button"
              variant="secondary"
            >
              {isRecovering ? "正在恢复..." : `恢复本轮${runActionLabel(run?.mode)}`}
            </Button>
          ) : null}
          <Button className="w-full sm:w-auto" onClick={onBack} type="button" variant="ghost">
            返回最近批次
          </Button>
        </div>
      </div>

      {error ? (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
          {error}
        </div>
      ) : null}

      {isLoading && !run ? (
        <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-4 text-sm text-zinc-600">
          正在同步这轮批量生成进度，请稍等。
        </div>
      ) : null}

      {run ? (
        <>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3">
              <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">状态</p>
              <p className="mt-2 text-lg font-semibold text-zinc-950">
                {formatBatchRunStatus(run.status, run.cancelRequested)}
              </p>
            </div>
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3">
              <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">进度</p>
              <p className="mt-2 text-lg font-semibold text-zinc-950">
                {run.completedBatches} / {run.totalBatches}
              </p>
            </div>
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3">
              <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">成功</p>
              <p className="mt-2 text-lg font-semibold text-emerald-700">
                {run.succeededBatches}
              </p>
            </div>
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3">
              <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">失败</p>
              <p className="mt-2 text-lg font-semibold text-rose-700">
                {run.failedBatches}
              </p>
            </div>
          </div>

          {isCancellingRun ? (
            <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              已提交取消请求，当前批次收尾后会结束这轮{runActionLabel(run?.mode)}。
            </div>
          ) : null}

          {run.currentBatchId ? (
            <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-700">
              {isCancellingRun ? "当前批次正在收尾，第 " : "当前正在处理第 "}
              {run.currentIndex} / {run.totalBatches} 个批次：
              <span className="break-all font-medium text-zinc-950"> {run.currentBatchId}</span>
            </div>
          ) : null}

          {run.lastError ? (
            <div className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-900">
              最近一次失败：{run.lastError}
            </div>
          ) : null}

          <div className="rounded-xl border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h3 className="text-sm font-semibold text-zinc-950">批次明细</h3>
                <p className="mt-1 text-xs text-zinc-500">
                  已同步 {items.length} 个子批次状态。
                </p>
              </div>
            </div>
            <div className="mt-3 space-y-2">
              {items.map((item) => (
                <div
                  className="flex flex-col items-start gap-3 rounded-lg border border-zinc-200 bg-white px-3 py-3 text-sm sm:flex-row sm:flex-wrap sm:items-center sm:justify-between"
                  key={item.id}
                >
                  <div className="min-w-0">
                    <p className="break-all font-medium text-zinc-950">{item.batchId}</p>
                    {item.batchStatus ? (
                      <p className="mt-1 text-xs text-zinc-500">
                        子任务状态：{formatBatchItemStatus(item.batchStatus)}
                        {item.asyncJobId ? ` · Async Job：${item.asyncJobId}` : ""}
                      </p>
                    ) : item.asyncJobId ? (
                      <p className="mt-1 text-xs text-zinc-500">
                        Async Job：{item.asyncJobId}
                      </p>
                    ) : null}
                    {getItemFailureReason(item) ? (
                      <p className="mt-1 text-xs text-rose-700">{getItemFailureReason(item)}</p>
                    ) : null}
                  </div>
                  <span className="rounded-full border border-zinc-200 px-2.5 py-1 text-xs text-zinc-700">
                    {formatBatchRunStatus(
                      item.status as SheinStudioBatchRunItemStatus as SheinStudioBatchRunStatus,
                      isCancellingRun && item.status === "running",
                    )}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {failedItems.length > 0 ? (
            <div className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-4">
              <h3 className="text-sm font-semibold text-rose-950">失败批次</h3>
              <ul className="mt-2 space-y-2 text-sm text-rose-900">
                {failedItems.map((item) => (
                  <li key={`failed:${item.id}`}>
                    {item.batchId}
                    {getItemFailureReason(item) ? `：${getItemFailureReason(item)}` : ""}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
