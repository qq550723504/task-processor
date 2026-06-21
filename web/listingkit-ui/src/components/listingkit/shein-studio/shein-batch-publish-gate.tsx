"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useQueries, useQueryClient } from "@tanstack/react-query";

import { shouldPollTaskResult } from "@/components/listingkit/tasks/task-status-query";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getListingKitPreview } from "@/lib/api/preview";
import { submitTask } from "@/lib/api/submit";
import { getListingKitTaskResult } from "@/lib/api/task-result";
import { getSheinSubmissionState } from "@/lib/listingkit/semantic-fields";
import { listingKitKeys } from "@/lib/query/keys";
import type { ListingKitPreview, ListingKitTaskResult } from "@/lib/types/listingkit";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio";

type GateStatus = {
  task: SheinStudioCreatedTask;
  result?: ListingKitTaskResult;
  preview?: ListingKitPreview;
};

type GateFilter = "all" | "draft" | "publish" | "blocked" | "submitted";
type BatchSubmitAction = "save_draft" | "publish";
type BatchSubmitStatus = "succeeded" | "failed";
type BatchSubmitResults = Record<
  BatchSubmitAction,
  Record<string, { error?: string; status: BatchSubmitStatus }>
>;

const emptyBatchSubmitResults: BatchSubmitResults = {
  save_draft: {},
  publish: {},
};

function deriveGateState(item: GateStatus) {
  const taskStatus = item.result?.status;
  const readiness = item.preview?.shein?.submit_readiness?.status;
  const submission = getSheinSubmissionState(item.preview?.shein);
  const canSaveDraft =
    taskStatus === "completed" &&
    (readiness === "ready" || readiness === "ready_with_warnings");
  const canPublish = taskStatus === "completed" && readiness === "ready";

  return {
    taskStatus: taskStatus ?? "unknown",
    readiness: readiness ?? "unknown",
    canSaveDraft,
    canPublish,
    lastSubmissionStatus: submission?.last_status,
    lastSubmissionAction: submission?.last_action,
    lastSubmissionError: submission?.last_error,
  };
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    blocked: "已阻断",
    completed: "已完成",
    failed: "失败",
    ready: "可发布",
    ready_with_warnings: "可保存草稿",
    unknown: "未知",
  };
  return labels[status] ?? status;
}

export function SheinBatchPublishGate({
  tasks,
}: {
  tasks: SheinStudioCreatedTask[];
}) {
  const client = useQueryClient();
  const [activeFilter, setActiveFilter] = useState<GateFilter>("all");
  const [isSavingDrafts, setIsSavingDrafts] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [submissionResults, setSubmissionResults] = useState<BatchSubmitResults>(
    emptyBatchSubmitResults,
  );

  const resultQueries = useQueries({
    queries: tasks.map((task) => ({
      queryKey: listingKitKeys.taskResult(task.id),
      queryFn: () => getListingKitTaskResult(task.id),
      refetchInterval: (query: { state: { data?: ListingKitTaskResult } }) =>
        shouldPollTaskResult(query.state.data?.status) ? 5000 : false,
      refetchOnWindowFocus: true,
    })),
  });

  const previewQueries = useQueries({
    queries: tasks.map((task) => ({
      queryKey: listingKitKeys.preview(task.id),
      queryFn: () => getListingKitPreview(task.id),
      enabled: resultQueries.some((query) => query.data?.status === "completed"),
      refetchOnWindowFocus: true,
    })),
  });

  const gateStatuses = useMemo(
    () =>
      tasks.map((task, index) => ({
        task,
        result: resultQueries[index]?.data,
        preview: previewQueries[index]?.data,
      })),
    [previewQueries, resultQueries, tasks],
  );

  const draftEligible = gateStatuses.filter((item) => deriveGateState(item).canSaveDraft);
  const publishEligible = gateStatuses.filter((item) => deriveGateState(item).canPublish);
  const visibleStatuses = gateStatuses.filter((item) => {
    const gate = deriveGateState(item);

    switch (activeFilter) {
      case "draft":
        return gate.canSaveDraft;
      case "publish":
        return gate.canPublish;
      case "blocked":
        return gate.taskStatus === "completed" && !gate.canSaveDraft;
      case "submitted":
        return Boolean(gate.lastSubmissionStatus);
      default:
        return true;
    }
  });

  if (tasks.length === 0) {
    return null;
  }

  async function refreshTask(taskId: string) {
    await Promise.all([
      client.invalidateQueries({ queryKey: listingKitKeys.taskResult(taskId) }),
      client.invalidateQueries({ queryKey: listingKitKeys.preview(taskId) }),
    ]);
  }

  async function handleBatchSubmit(action: BatchSubmitAction) {
    const eligible = action === "publish" ? publishEligible : draftEligible;
    if (eligible.length === 0) {
      setError(
        action === "publish"
          ? "没有可正式发布的任务。"
          : "没有可保存草稿的任务。",
      );
      setMessage("");
      return;
    }

    const remaining = eligible.filter(
      (item) => submissionResults[action][item.task.id]?.status !== "succeeded",
    );
    const skippedCount = eligible.length - remaining.length;
    if (remaining.length === 0) {
      setError("");
      setMessage(
        action === "publish"
          ? `已跳过 ${skippedCount} 个已成功发布的 SHEIN 任务。`
          : `已跳过 ${skippedCount} 个已成功保存的 SHEIN 草稿。`,
      );
      return;
    }

    setError("");
    setMessage("");
    if (action === "publish") {
      setIsPublishing(true);
    } else {
      setIsSavingDrafts(true);
    }

    try {
      const failures: Array<{ item: GateStatus; message: string }> = [];
      let successCount = 0;

      for (const item of remaining) {
        try {
          await submitTask(item.task.id, {
            platform: "shein",
            action,
          });
          successCount += 1;
          setSubmissionResults((current) => ({
            ...current,
            [action]: {
              ...current[action],
              [item.task.id]: { status: "succeeded" },
            },
          }));
        } catch (submitError) {
          const failureMessage =
            submitError instanceof Error
              ? submitError.message
              : "SHEIN 批量提交失败。";
          failures.push({ item, message: failureMessage });
          setSubmissionResults((current) => ({
            ...current,
            [action]: {
              ...current[action],
              [item.task.id]: { error: failureMessage, status: "failed" },
            },
          }));
        } finally {
          await refreshTask(item.task.id);
        }
      }

      setMessage(
        buildBatchSubmitMessage(action, successCount, failures.length, skippedCount),
      );
      setError(
        failures.length > 0
          ? failures
              .map(({ item, message: failureMessage }) => `${item.task.title}：${failureMessage}`)
              .join("；")
          : "",
      );
    } finally {
      setIsPublishing(false);
      setIsSavingDrafts(false);
    }
  }

  return (
    <Card className="border-zinc-300 bg-zinc-50/80 p-5">
      <div className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN 批量提交检查
            </p>
            <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
              只提交真正可用的任务
            </h2>
            <p className="mt-2 text-sm leading-6 text-zinc-600">
              “保存草稿”允许带提醒的任务；“正式发布”只允许完全通过提交前检查的任务。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2 text-xs font-semibold text-zinc-700">
              可保存草稿：{draftEligible.length}
            </div>
            <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2 text-xs font-semibold text-zinc-700">
              可正式发布：{publishEligible.length}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-3">
          <Button
            disabled={isSavingDrafts || draftEligible.length === 0}
            onClick={() => handleBatchSubmit("save_draft")}
            variant="secondary"
          >
            {isSavingDrafts ? "正在保存草稿..." : "保存可用草稿"}
          </Button>
          <Button
            disabled={isPublishing || publishEligible.length === 0}
            onClick={() => handleBatchSubmit("publish")}
          >
            {isPublishing ? "正在发布..." : "发布可用任务"}
          </Button>
        </div>

        <div className="flex flex-wrap gap-2">
          {[
            ["all", `全部 ${gateStatuses.length}`],
            ["draft", `可保存草稿 ${draftEligible.length}`],
            ["publish", `可发布 ${publishEligible.length}`],
            [
              "blocked",
              `已阻断 ${
                gateStatuses.filter((item) => {
                  const gate = deriveGateState(item);
                  return gate.taskStatus === "completed" && !gate.canSaveDraft;
                }).length
              }`,
            ],
            [
              "submitted",
              `已提交 ${
                gateStatuses.filter((item) => deriveGateState(item).lastSubmissionStatus)
                  .length
              }`,
            ],
          ].map(([filter, label]) => (
            <Button
              className={`h-auto rounded-xl px-3 py-2 text-xs ${
                activeFilter === filter ? "text-white" : "text-zinc-700"
              }`}
              key={filter}
              onClick={() => setActiveFilter(filter as GateFilter)}
              type="button"
              variant={activeFilter === filter ? "default" : "outline"}
            >
              {label}
            </Button>
          ))}
        </div>

        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
        {message ? (
          <div className="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {message}
          </div>
        ) : null}

        <div className="grid gap-3">
          {visibleStatuses.map((item) => {
            const gate = deriveGateState(item);
            return (
              <div
                className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-zinc-200 bg-white/80 px-4 py-3"
                key={item.task.id}
              >
                <div className="space-y-1">
                  <div className="text-sm font-semibold text-zinc-950">
                    {item.task.title}
                  </div>
                  <div className="text-xs text-zinc-500">{item.task.id}</div>
                  <div className="flex flex-wrap gap-2 text-xs">
                    <span className="rounded-lg bg-zinc-100 px-2 py-1 font-medium text-zinc-700">
                      任务：{statusLabel(gate.taskStatus)}
                    </span>
                    <span className="rounded-lg bg-zinc-100 px-2 py-1 font-medium text-zinc-700">
                      提交检查：{statusLabel(gate.readiness)}
                    </span>
                    {gate.canPublish ? (
                      <span className="rounded-lg bg-emerald-100 px-2 py-1 font-medium text-emerald-700">
                        可发布
                      </span>
                    ) : gate.canSaveDraft ? (
                      <span className="rounded-lg bg-amber-100 px-2 py-1 font-medium text-amber-700">
                        可保存草稿
                      </span>
                    ) : null}
                    {gate.lastSubmissionStatus ? (
                      <span className="rounded-lg bg-zinc-100 px-2 py-1 font-medium text-zinc-700">
                        最近：{gate.lastSubmissionAction ?? "submit"} / {statusLabel(gate.lastSubmissionStatus)}
                      </span>
                    ) : null}
                  </div>
                  {gate.lastSubmissionError ? (
                    <div className="text-xs text-rose-600">{gate.lastSubmissionError}</div>
                  ) : null}
                </div>

                <div className="flex flex-wrap gap-2">
                  <Link href={`/listing-kits/${item.task.id}/status`}>
                    <Button variant="ghost">状态</Button>
                  </Link>
                  <Link href={`/listing-kits/${item.task.id}/workspace`}>
                    <Button variant="secondary">工作区</Button>
                  </Link>
                </div>
              </div>
            );
          })}
          {visibleStatuses.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-zinc-200 bg-white/70 px-4 py-6 text-sm text-zinc-500">
              当前筛选下没有任务。
            </div>
          ) : null}
        </div>
      </div>
    </Card>
  );
}

function buildBatchSubmitMessage(
  action: BatchSubmitAction,
  successCount: number,
  failedCount: number,
  skippedCount: number,
) {
  const base =
    action === "publish"
      ? `已发布 ${successCount} 个 SHEIN 任务`
      : `已保存 ${successCount} 个 SHEIN 草稿`;
  const suffixes = [
    failedCount > 0 ? `失败 ${failedCount} 个` : "",
    skippedCount > 0 ? `跳过 ${skippedCount} 个` : "",
  ].filter(Boolean);

  return suffixes.length > 0
    ? `${base}，${suffixes.join("，")}。`
    : `${base}。`;
}
