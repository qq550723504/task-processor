"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

import { extractTaskFixes, inferTaskDraftFocus } from "@/components/listingkit/tasks/task-fixes";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { TaskSDSSyncCard } from "@/components/listingkit/tasks/task-sds-sync-card";
import { loadTaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import { TaskSourceSummary } from "@/components/listingkit/tasks/task-source-summary";
import { shouldAutoOpenWorkspace } from "@/components/listingkit/tasks/task-status-transition";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function TaskStatusScreen({
  taskId,
  task,
}: {
  taskId: string;
  task?: ListingKitTaskResult | null;
}) {
  const router = useRouter();
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

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <Card className="p-6">
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            ListingKit Task
          </p>
          <h1 className="text-3xl font-semibold tracking-tight text-zinc-950">
            Status {taskId}
          </h1>
          <p className="text-sm leading-6 text-zinc-600">
            Use this page as the handoff after task creation. Once the task is ready,
            continue into queue or workspace.
          </p>
        </div>
      </Card>

      <TaskStatusPanel task={task} />
      <TaskSDSSyncCard task={task} />
      <TaskSourceSummary draft={taskDraft} />
      <TaskProgressNotice task={task} />

      {task.status === "failed" && taskFixes.length > 0 ? (
        <Card className="p-6">
          <div className="space-y-4">
            <h2 className="text-lg font-semibold text-zinc-950">Recommended fixes</h2>
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
                tone="secondary"
                onClick={() =>
                  router.push(
                    `/listing-kits/new?fromTask=${taskId}${taskDraftFocus ? `&focus=${taskDraftFocus}` : ""}${taskDraftIssues.length > 0 ? `&issues=${taskDraftIssues.join(",")}` : ""}`,
                  )
                }
              >
                Create improved task
              </Button>
            </div>
          </div>
        </Card>
      ) : null}

      {isTerminal ? (
        <Card className="p-6">
          <div className="space-y-4">
            <h2 className="text-lg font-semibold text-zinc-950">
              {task.status === "completed"
                ? "Task completed"
                : task.status === "needs_review"
                  ? "Task requires review"
                  : "Inspect task output"}
            </h2>
            <p className="text-sm leading-6 text-zinc-600">
              {task.status === "completed"
                ? "Review the generated queue or go straight into the workspace."
                : task.status === "needs_review"
                  ? "Review the generated queue and workspace before approving or revising the output."
                  : "Review the queue and workspace to inspect the failure details and any partial output."}
            </p>
            {task.status === "completed" || task.status === "needs_review" ? (
              <div className="flex flex-wrap items-center gap-3">
                <p className="text-sm leading-6 text-zinc-500">
                  {autoOpenEnabled
                    ? "Opening workspace automatically in 1.5 seconds."
                    : "Auto-open paused."}
                </p>
                {autoOpenEnabled ? (
                  <Button
                    tone="secondary"
                    onClick={() => setAutoOpenEnabled(false)}
                    type="button"
                  >
                    Cancel auto-open
                  </Button>
                ) : null}
              </div>
            ) : null}
            <div className="flex flex-wrap gap-3">
              <Button onClick={() => router.push(`/listing-kits/${taskId}/workspace`)}>
                Open workspace
              </Button>
              <Button
                tone="secondary"
                onClick={() => router.push(`/listing-kits/${taskId}/queue`)}
              >
                Open queue
              </Button>
            </div>
          </div>
        </Card>
      ) : null}
    </div>
  );
}
