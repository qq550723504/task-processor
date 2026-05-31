"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

import { extractTaskFixes, inferTaskDraftFocus } from "@/components/listingkit/tasks/task-fixes";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { TaskRevisionHistoryPanel } from "@/components/listingkit/tasks/task-revision-history-panel";
import { TaskPodExecutionCard } from "@/components/listingkit/tasks/task-pod-execution-card";
import { loadTaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import { TaskSourceSummary } from "@/components/listingkit/tasks/task-source-summary";
import { shouldAutoOpenWorkspace } from "@/components/listingkit/tasks/task-status-transition";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useExecuteAction } from "@/lib/query/use-action";
import { useRetryChildTask } from "@/lib/query/use-child-task-retry";
import {
  sheinSubmissionRemoteStatusLabel,
  sheinWorkflowStatusLabel,
} from "@/lib/shein-studio/shein-submission-display";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

function formatStatusDate(value?: string) {
  if (!value) {
    return null;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function terminalStatusTitle(task?: ListingKitTaskResult | null) {
  if (task?.status === "completed" && task?.shein_workflow_status === "published") {
    return "任务生成已完成，SHEIN 已发布";
  }
  if (task?.status === "completed" && task?.shein_workflow_status === "draft_saved") {
    return "任务生成已完成，SHEIN 草稿已保存";
  }
  if (task?.status === "completed") {
    return "任务已处理完成";
  }
  if (task?.status === "needs_review") {
    return "任务需要人工确认";
  }
  return "查看结果并继续";
}

function terminalStatusDescription(task?: ListingKitTaskResult | null) {
  if (task?.status === "completed" && task?.shein_workflow_status === "published") {
    return "生成链路已经完成，且商品资料已经发布到 SHEIN。建议进入工作台核对最终结果和提交记录。";
  }
  if (task?.status === "completed" && task?.shein_workflow_status === "draft_saved") {
    return "生成链路已经完成，且商品资料已经保存到 SHEIN 草稿箱。建议进入工作台继续复核或正式发布。";
  }
  if (task?.status === "completed" || task?.status === "needs_review") {
    return "建议先查看工作台和结果，再决定继续提交还是回退修改。";
  }
  return "建议先进入工作台或队列查看失败详情，再决定是否重新创建任务。";
}

export function TaskStatusScreen({
  taskId,
  task,
}: {
  taskId: string;
  task?: ListingKitTaskResult | null;
}) {
  const router = useRouter();
  const layerAction = useExecuteAction(taskId, {});
  const childTaskRetry = useRetryChildTask(taskId);
  const isTerminal =
    task?.status === "completed" ||
    task?.status === "failed" ||
    task?.status === "needs_review";
  const taskDraft = useMemo(() => loadTaskCreateDraft(taskId), [taskId]);
  const taskFixes = extractTaskFixes(task);
  const taskDraftFocus = inferTaskDraftFocus(task);
  const [autoOpenEnabled, setAutoOpenEnabled] = useState(true);
  const taskDraftIssues = useMemo(() => {
    const issues: Array<"text" | "imageUrls" | "productUrl"> = [];
    if (
      taskFixes.some(
        (fix) =>
          fix.includes("链接") ||
          fix.includes("1688") ||
          fix.toLowerCase().includes("url"),
      )
    ) {
      issues.push("productUrl");
    }
    if (taskFixes.some((fix) => fix.includes("图片"))) {
      issues.push("imageUrls");
    }
    if (taskFixes.some((fix) => fix.includes("描述") || fix.includes("字符") || fix.includes("文本"))) {
      issues.push("text");
    }
    return issues;
  }, [taskFixes]);

  useEffect(() => {
    if (!shouldAutoOpenWorkspace(task) || !autoOpenEnabled) {
      return;
    }

    const timeout = window.setTimeout(() => {
      router.push(`/listing-kits/${taskId}/workspace`);
    }, 1500);

    return () => window.clearTimeout(timeout);
  }, [autoOpenEnabled, router, task, taskId]);

  if (!task) {
    return null;
  }

  const handleRunStandardProductTemporal = () => {
    layerAction.mutate({
      action_key: "run_standard_product_temporal",
    });
  };

  const handleRunPlatformAdaptTemporal = () => {
    layerAction.mutate({
      action_key: "run_platform_adapt_temporal",
      target: {
        action_key: "run_platform_adapt_temporal",
        queue_query: {
          platform: "all",
        },
      },
    });
  };

  const handleRetrySDSDesignSync = () => {
    childTaskRetry.mutate({
      kind: "sds_design_sync",
    });
  };

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <Card className="p-6">
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            任务状态
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-zinc-950">
            查看当前任务进度
          </h1>
          <p className="text-sm leading-6 text-zinc-600">
            创建任务后，可以先在这里查看当前进度，再决定进入工作台、队列或返回修改。
          </p>
          {task.shein_workflow_status || task.shein_submission_remote_status ? (
            <div className="flex flex-wrap gap-2 pt-2">
              {task.shein_workflow_status ? (
                <span className="inline-flex rounded-full border border-orange-200 bg-orange-50 px-2.5 py-1 text-xs font-medium text-orange-700">
                  SHEIN {sheinWorkflowStatusLabel(task.shein_workflow_status)}
                </span>
              ) : null}
              {task.shein_submission_remote_status ? (
                <span className="inline-flex rounded-full border border-sky-200 bg-sky-50 px-2.5 py-1 text-xs font-medium text-sky-700">
                  {sheinSubmissionRemoteStatusLabel(task.shein_submission_remote_status)}
                </span>
              ) : null}
            </div>
          ) : null}
          <div className="grid gap-3 pt-2 sm:grid-cols-2 xl:grid-cols-3">
            <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                任务 ID
              </p>
              <p className="mt-1 break-all text-sm text-zinc-700">{taskId}</p>
            </div>
            {formatStatusDate(task.result?.updated_at ?? task.completed_at) ? (
              <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  最近更新
                </p>
                <p className="mt-1 text-sm text-zinc-700">
                  {formatStatusDate(task.result?.updated_at ?? task.completed_at)}
                </p>
              </div>
            ) : null}
            {formatStatusDate(task.created_at) ? (
              <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  创建时间
                </p>
                <p className="mt-1 text-sm text-zinc-700">
                  {formatStatusDate(task.created_at)}
                </p>
              </div>
            ) : null}
          </div>
        </div>
      </Card>

      <TaskStatusPanel
        task={task}
        onRetryChildTask={handleRetrySDSDesignSync}
        retryingChildTaskKind={childTaskRetry.isPending ? "sds_design_sync" : null}
      />
      <Card className="p-6">
        <div className="space-y-4">
          <div className="space-y-1">
            <h2 className="text-lg font-semibold text-zinc-950">分层 Temporal 执行</h2>
            <p className="text-sm leading-6 text-zinc-600">
              标准商品层和平台适配层现在可以分别手动触发。标准层会产出稳定的标准商品快照，平台层会基于这个快照继续做 SHEIN 等平台适配。
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <Button
              className="w-full sm:w-auto"
              onClick={handleRunStandardProductTemporal}
              type="button"
              variant="secondary"
              disabled={layerAction.isPending}
            >
              运行标准商品层
            </Button>
            <Button
              className="w-full sm:w-auto"
              onClick={handleRunPlatformAdaptTemporal}
              type="button"
              variant="secondary"
              disabled={layerAction.isPending}
            >
              运行平台适配层
            </Button>
          </div>
        </div>
      </Card>
      <TaskRevisionHistoryPanel taskId={taskId} />
      <TaskPodExecutionCard task={task} />
      <TaskSourceSummary draft={taskDraft} />
      <TaskProgressNotice task={task} />

      {task.status === "failed" && taskFixes.length > 0 ? (
        <Card className="p-6">
          <div className="space-y-4">
            <h2 className="text-lg font-semibold text-zinc-950">建议先处理这些问题</h2>
            <ul className="space-y-2 text-sm leading-6 text-zinc-700">
              {taskFixes.map((fix) => (
                <li
                  className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3"
                  key={fix}
                >
                  {fix}
                </li>
              ))}
            </ul>
            <div>
              <Button
                variant="secondary"
                onClick={() =>
                  router.push(
                    `/listing-kits/new?fromTask=${taskId}${taskDraftFocus ? `&focus=${taskDraftFocus}` : ""}${taskDraftIssues.length > 0 ? `&issues=${taskDraftIssues.join(",")}` : ""}`,
                  )
                }
              >
                基于当前内容重新创建任务
              </Button>
            </div>
          </div>
        </Card>
      ) : null}

      {isTerminal ? (
        <Card className="p-6">
          <div className="space-y-4">
            <h2 className="text-lg font-semibold text-zinc-950">{terminalStatusTitle(task)}</h2>
            <p className="text-sm leading-6 text-zinc-600">{terminalStatusDescription(task)}</p>
            {task.status === "completed" || task.status === "needs_review" ? (
              <div className="flex flex-wrap items-center gap-3">
                <p className="text-sm leading-6 text-zinc-500">
                  {autoOpenEnabled
                    ? "1.5 秒后会自动进入工作台，你也可以先留在这里查看状态。"
                    : "已暂停自动跳转。"}
                </p>
                {autoOpenEnabled ? (
                  <Button
                    variant="secondary"
                    onClick={() => setAutoOpenEnabled(false)}
                    type="button"
                  >
                    取消自动跳转
                  </Button>
                ) : null}
              </div>
            ) : null}
            <div className="flex flex-wrap gap-3">
              <Button onClick={() => router.push(`/listing-kits/${taskId}/workspace`)}>
                打开工作台
              </Button>
              <Button
                variant="secondary"
                onClick={() => router.push(`/listing-kits/${taskId}/queue`)}
              >
                打开队列
              </Button>
            </div>
          </div>
        </Card>
      ) : null}
    </div>
  );
}
