import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

export const STATUS_OPTIONS = [
  { value: "", label: "全部任务状态" },
  { value: "pending", label: "待处理" },
  { value: "processing", label: "处理中" },
  { value: "completed", label: "已完成" },
  { value: "needs_review", label: "待审核" },
  { value: "failed", label: "失败" },
];

export const PLATFORM_OPTIONS = [
  { value: "", label: "全部平台" },
  { value: "shein", label: "SHEIN" },
  { value: "amazon", label: "Amazon" },
  { value: "temu", label: "Temu" },
];

export const SHEIN_WORKFLOW_OPTIONS = [
  { value: "", label: "全部 SHEIN 状态" },
  { value: "pending_confirmation", label: "待确认" },
  { value: "ready_to_submit", label: "可提交" },
  { value: "publish_failed", label: "发布失败" },
  { value: "published", label: "已发布" },
  { value: "draft_saved", label: "草稿已保存" },
];

export const primaryLinkClass =
  "inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800";

export const secondaryLinkClass =
  "inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100";

export function formatDate(value?: string) {
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

export function statusTone(status?: string) {
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

export function taskStatusLabel(status?: string) {
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

export function taskTitle(task: ListingKitTaskListItem) {
  return task.product_name || task.title || task.task_id.slice(0, 8);
}
