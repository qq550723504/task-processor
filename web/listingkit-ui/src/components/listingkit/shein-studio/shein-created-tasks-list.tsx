"use client";

import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import type {
  SheinStudioCreatedTask,
  SheinStudioFailedTask,
  SheinStudioRejectedTask,
} from "@/lib/types/shein-studio";

export function SheinCreatedTasksList({
  failedTasks = [],
  rejectedTasks = [],
  reusedTasks = [],
  tasks,
}: {
  failedTasks?: SheinStudioFailedTask[];
  rejectedTasks?: SheinStudioRejectedTask[];
  reusedTasks?: SheinStudioCreatedTask[];
  tasks: SheinStudioCreatedTask[];
}) {
  const router = useRouter();
  const visibleTasks = [
    ...tasks.map((task) => ({ ...task, outcome: task.outcome ?? "created" })),
    ...reusedTasks.map((task) => ({ ...task, outcome: "reused" as const })),
  ];

  if (
    visibleTasks.length === 0 &&
    rejectedTasks.length === 0 &&
    failedTasks.length === 0
  ) {
    return null;
  }

  return (
    <div className="space-y-3">
      {visibleTasks.length > 0 ? (
        <div className="rounded-[1.25rem] border border-emerald-200/70 bg-emerald-50/90 px-4 py-4 dark:border-emerald-500/30 dark:bg-emerald-950/25">
          <div className="text-sm font-semibold text-emerald-900">
            SHEIN 资料任务已创建或复用
          </div>
          <p className="mt-1 text-sm leading-6 text-emerald-800 dark:text-emerald-100/90">
            打开工作区确认 SHEIN 资料、价格和提交状态；如果需要处理生成/审核队列，可进入队列页。
          </p>
          <div className="mt-3 grid gap-3">
            {visibleTasks.map((task) => (
              <div
                className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-emerald-200/80 bg-background/90 px-4 py-3 dark:border-emerald-500/20 dark:bg-card/95"
                key={`${task.outcome}:${task.id}`}
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="text-sm font-semibold text-foreground">
                      {task.title}
                    </div>
                    <span className="rounded-full border border-emerald-200 bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-800">
                      {task.outcome === "reused" ? "已复用" : "新建"}
                    </span>
                    {task.source ? (
                      <span className="rounded-full border border-sky-200 bg-sky-50 px-2 py-0.5 text-[11px] font-medium text-sky-700">
                        {taskSourceLabel(task.source)}
                      </span>
                    ) : null}
                    {task.status ? (
                      <span className="rounded-full border border-zinc-200 bg-white px-2 py-0.5 text-[11px] font-medium text-zinc-600">
                        {task.status}
                      </span>
                    ) : null}
                  </div>
                  <div className="text-xs text-muted-foreground">{task.id}</div>
                  <TaskMetadataLine task={task} />
                </div>
                <div className="flex gap-2">
                  <Button
                    onClick={() => router.push(`/listing-kits/${task.id}/status`)}
                    variant="secondary"
                  >
                    状态
                  </Button>
                  <Button
                    onClick={() =>
                      router.push(`/listing-kits/${task.id}/queue?platform=shein`)
                    }
                    variant="secondary"
                  >
                    队列
                  </Button>
                  <Button
                    onClick={() =>
                      router.push(
                        `/listing-kits/${task.id}/workspace?platform=shein&section_key=general_review`,
                      )
                    }
                    variant="ghost"
                  >
                    审核 SHEIN 资料
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : null}
      {rejectedTasks.length > 0 ? (
        <TaskOutcomePanel
          items={rejectedTasks}
          title="未创建的候选"
          tone="amber"
          summary="这些候选被后端业务门禁拒绝，处理原因后可再次创建。"
        />
      ) : null}
      {failedTasks.length > 0 ? (
        <TaskOutcomePanel
          items={failedTasks}
          title="创建失败的候选"
          tone="rose"
          summary="这些候选在执行创建时失败，通常可在排查错误后重试。"
        />
      ) : null}
    </div>
  );
}

function taskSourceLabel(source: string) {
  switch (source) {
    case "batch_created":
      return "批次创建";
    case "legacy_session_backfilled":
      return "旧任务回填";
    case "rejected":
      return "后端拒绝";
    default:
      return source;
  }
}

function TaskMetadataLine({ task }: { task: SheinStudioCreatedTask }) {
  const parts = [
    task.designId ? `design ${task.designId}` : "",
    task.itemId ? `item ${task.itemId}` : "",
    task.selectionId ? `selection ${task.selectionId}` : "",
    task.compatibilityFingerprint
      ? `fingerprint ${task.compatibilityFingerprint}`
      : "",
    task.submissionState ? `submission ${task.submissionState}` : "",
    task.lastSubmissionAction ? `last ${task.lastSubmissionAction}` : "",
  ].filter(Boolean);
  if (parts.length === 0) {
    return null;
  }
  return (
    <div className="max-w-3xl text-xs leading-5 text-muted-foreground">
      {parts.join(" · ")}
    </div>
  );
}

function TaskOutcomePanel({
  items,
  summary,
  title,
  tone,
}: {
  items: Array<SheinStudioRejectedTask | SheinStudioFailedTask>;
  summary: string;
  title: string;
  tone: "amber" | "rose";
}) {
  const toneClass =
    tone === "amber"
      ? "border-amber-200 bg-amber-50/90 text-amber-950"
      : "border-rose-200 bg-rose-50/90 text-rose-950";
  return (
    <div className={`rounded-[1.25rem] border px-4 py-4 ${toneClass}`}>
      <div className="text-sm font-semibold">{title}</div>
      <p className="mt-1 text-sm leading-6 opacity-85">{summary}</p>
      <div className="mt-3 grid gap-3">
        {items.map((item, index) => (
          <div
            className="rounded-2xl border border-white/80 bg-white/85 px-4 py-3 text-zinc-900 shadow-sm"
            key={`${item.designId}:${item.selectionId ?? ""}:${index}`}
          >
            <div className="text-sm font-semibold">
              {item.title?.trim() || item.designId}
            </div>
            <div className="mt-1 text-sm leading-6">
              {item.reasonCode ? (
                <span className="font-semibold">{item.reasonCode}：</span>
              ) : null}
              {item.message?.trim() || "后端没有返回详细原因。"}
            </div>
            <div className="mt-2 text-xs leading-5 text-zinc-500">
              {[
                item.itemId ? `item ${item.itemId}` : "",
                item.selectionId ? `selection ${item.selectionId}` : "",
                item.compatibilityFingerprint
                  ? `fingerprint ${item.compatibilityFingerprint}`
                  : "",
                item.status ? `status ${item.status}` : "",
              ]
                .filter(Boolean)
                .join(" · ")}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
