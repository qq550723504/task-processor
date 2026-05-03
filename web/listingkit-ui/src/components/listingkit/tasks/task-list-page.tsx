"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, Boxes, Clock, LoaderCircle, Plus, RefreshCw } from "lucide-react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { EmptyState } from "@/components/shared/empty-state";
import { SheinSettingsCard } from "@/components/listingkit/shein/shein-settings-card";
import { useListingKitTasks } from "@/lib/query/use-task-list";
import {
  sheinSubmissionStatusLabel,
  sheinWorkflowStatusLabel,
} from "@/lib/shein-studio/shein-submission-display";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

const STATUS_OPTIONS = [
  { value: "", label: "全部任务状态" },
  { value: "pending", label: "待处理" },
  { value: "processing", label: "处理中" },
  { value: "completed", label: "已完成" },
  { value: "needs_review", label: "待审核" },
  { value: "failed", label: "失败" },
];

const PLATFORM_OPTIONS = [
  { value: "", label: "全部平台" },
  { value: "shein", label: "SHEIN" },
  { value: "amazon", label: "Amazon" },
  { value: "temu", label: "Temu" },
];

const SHEIN_WORKFLOW_OPTIONS = [
  { value: "", label: "全部 SHEIN 状态" },
  { value: "pending_confirmation", label: "待确认" },
  { value: "ready_to_submit", label: "可提交" },
  { value: "publish_failed", label: "发布失败" },
  { value: "published", label: "已发布" },
  { value: "draft_saved", label: "草稿已保存" },
];

const primaryLinkClass =
  "inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800";
const secondaryLinkClass =
  "inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100";

function formatDate(value?: string) {
  if (!value) {
    return "未知";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function statusTone(status?: string) {
  switch (status) {
    case "completed":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "needs_review":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "failed":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "processing":
      return "border-sky-200 bg-sky-50 text-sky-700";
    default:
      return "border-zinc-200 bg-zinc-50 text-zinc-600";
  }
}

function taskStatusLabel(status?: string) {
  switch (status) {
    case "pending":
      return "待处理";
    case "processing":
      return "处理中";
    case "completed":
      return "已完成";
    case "needs_review":
      return "待审核";
    case "failed":
      return "失败";
    default:
      return status ?? "未知";
  }
}

function taskTitle(task: ListingKitTaskListItem) {
  return (
    task.product_name ||
    task.title ||
    task.task_id.slice(0, 8)
  );
}

export function TaskListPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const status = searchParams.get("status") ?? "";
  const platform = searchParams.get("platform") ?? "";
  const sheinWorkflowStatus = searchParams.get("shein_workflow_status") ?? "";
  const page = Number(searchParams.get("page") ?? "1") || 1;
  const query = {
    status: status || undefined,
    platform: platform || undefined,
    shein_workflow_status: sheinWorkflowStatus || undefined,
    page,
    page_size: 20,
  };
  const tasks = useListingKitTasks(query);
  const items = tasks.data?.items ?? [];

  const updateFilter = (key: "status" | "platform" | "shein_workflow_status", value: string) => {
    const params = sanitizedNavigationSearchParams(searchParams);
    if (value) {
      params.set(key, value);
    } else {
      params.delete(key);
    }
    params.delete("page");
    router.push(`/listing-kits${params.toString() ? `?${params.toString()}` : ""}`);
  };

  return (
    <div className="relative isolate min-h-screen overflow-hidden bg-[radial-gradient(circle_at_12%_10%,rgba(20,184,166,0.18),transparent_30%),radial-gradient(circle_at_86%_4%,rgba(251,146,60,0.16),transparent_26%),linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)] px-6 py-10">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.035)_1px,transparent_1px)] bg-[size:34px_34px]" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-col gap-6">
        <section className="grid gap-5 rounded-[2rem] border border-white/70 bg-white/78 p-6 shadow-[0_24px_90px_rgba(39,39,42,0.10)] backdrop-blur lg:grid-cols-[1fr_auto] lg:items-end">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">
              ListingKit Tasks
            </p>
            <h1 className="mt-3 text-4xl font-semibold tracking-[-0.04em] text-zinc-950">
              任务列表
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-600">
              查看最近生成、审核和提交到 SHEIN 的任务。这里直接读取后端任务仓储，不再靠你手动记 task id。
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <Button tone="secondary" onClick={() => tasks.refetch()}>
              <RefreshCw className="mr-2 h-4 w-4" />
              刷新
            </Button>
            <Link href="/listing-kits/shein" className={primaryLinkClass}>
              <Plus className="mr-2 h-4 w-4" />
              新建 SHEIN 批次
            </Link>
          </div>
        </section>

        <SheinSettingsCard />

        <Card className="border-white/70 bg-white/82 p-4">
          <div className="flex flex-wrap gap-3">
            <select
              className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
              value={status}
              onChange={(event) => updateFilter("status", event.target.value)}
            >
              {STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <select
              className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
              value={sheinWorkflowStatus}
              onChange={(event) => updateFilter("shein_workflow_status", event.target.value)}
            >
              {SHEIN_WORKFLOW_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <select
              className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
              value={platform}
              onChange={(event) => updateFilter("platform", event.target.value)}
            >
              {PLATFORM_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <div className="ml-auto flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-zinc-500">
              <Boxes className="h-4 w-4" />
              {tasks.data?.total ?? 0} 个任务
            </div>
          </div>
        </Card>

        {tasks.isLoading ? (
          <Card className="flex min-h-72 items-center justify-center border-white/70 bg-white/80">
            <LoaderCircle className="h-6 w-6 animate-spin text-zinc-500" />
          </Card>
        ) : tasks.isError ? (
          <EmptyState
            title="任务列表加载失败"
            description="后端列表接口暂时不可用，可以刷新重试。"
          />
        ) : items.length === 0 ? (
          <EmptyState
            title="暂无任务"
            description="先从 SHEIN Studio 创建一个批次，生成后会出现在这里。"
            action={
              <Link href="/listing-kits/shein" className={primaryLinkClass}>
                新建 SHEIN 批次
              </Link>
            }
          />
        ) : (
          <div className="grid gap-4">
            {items.map((task) => (
              <TaskRow key={task.task_id} task={task} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function TaskRow({ task }: { task: ListingKitTaskListItem }) {
  const workspaceHref = `/listing-kits/${task.task_id}/workspace?platform=${task.platforms?.[0] ?? "shein"}`;

  return (
    <Card className="group border-white/70 bg-white/88 p-5 shadow-[0_16px_44px_rgba(39,39,42,0.07)] transition hover:-translate-y-0.5 hover:shadow-[0_22px_60px_rgba(39,39,42,0.11)]">
      <div className="grid gap-4 lg:grid-cols-[1fr_auto] lg:items-center">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${statusTone(task.status)}`}>
              {taskStatusLabel(task.status)}
            </span>
            {task.sds_sync_status ? (
              <span className="rounded-full border border-teal-200 bg-teal-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-teal-700">
                SDS {task.sds_sync_status}
              </span>
            ) : null}
            {task.shein_workflow_status ? (
              <span className="rounded-full border border-orange-200 bg-orange-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-orange-700">
                {sheinWorkflowStatusLabel(task.shein_workflow_status)}
              </span>
            ) : null}
            {(task.platforms ?? []).map((platform) => (
              <span key={platform} className="rounded-full bg-zinc-100 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-zinc-600">
                {platform}
              </span>
            ))}
          </div>
          <h2 className="mt-3 truncate text-xl font-semibold tracking-tight text-zinc-950">
            {taskTitle(task)}
          </h2>
          <p className="mt-1 text-xs font-medium uppercase tracking-[0.16em] text-zinc-400">
            任务 ID
          </p>
          <p className="mt-1 break-all text-sm text-zinc-500">{task.task_id}</p>
          {task.variant_label ? (
            <p className="mt-1 truncate text-sm text-zinc-500">{task.variant_label}</p>
          ) : null}
          {task.error ? (
            <p className="mt-2 line-clamp-2 text-sm text-rose-600">{task.error}</p>
          ) : null}
          {task.shein_latest_submission_error ? (
            <p className="mt-2 line-clamp-2 text-sm text-rose-600">
              最近提交：发布失败。原始错误：{task.shein_latest_submission_error}
            </p>
          ) : task.shein_latest_submission_status ? (
            <p className="mt-2 text-sm text-zinc-500">
              最近提交：{sheinSubmissionStatusLabel(task.shein_latest_submission_status)}
            </p>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-3 lg:justify-end">
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-600">
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4" />
              {formatDate(task.created_at)}
            </div>
            <div className="mt-1 text-xs text-zinc-500">{task.image_count ?? 0} images</div>
          </div>
          <Link href={`/listing-kits/${task.task_id}/status`} className={secondaryLinkClass}>
            状态
          </Link>
          <Link href={workspaceHref} className={primaryLinkClass}>
            工作台
            <ArrowRight className="ml-2 h-4 w-4 transition group-hover:translate-x-0.5" />
          </Link>
        </div>
      </div>
    </Card>
  );
}
