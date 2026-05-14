import type {
  ListingKitTaskFacetDescriptor,
  ListingKitTaskListItem,
  ListingKitTaskListSummary,
  ListingKitTaskListTaxonomy,
} from "@/lib/types/listingkit";

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

export const SHEIN_WORK_QUEUE_OPTIONS = [
  { value: "", label: "全部工作队列" },
  { value: "generation_queue", label: "生成队列" },
  { value: "generation_failed_queue", label: "生成失败队列" },
  { value: "repair_queue", label: "修复队列" },
  { value: "review_queue", label: "复核队列" },
  { value: "submit_ready_queue", label: "待提交队列" },
  { value: "draft_queue", label: "草稿队列" },
  { value: "submit_failed_queue", label: "提交失败队列" },
  { value: "published_queue", label: "已发布队列" },
];

export const SHEIN_ACTION_QUEUE_OPTIONS = [
  { value: "", label: "全部处理动作" },
  { value: "store_auth_queue", label: "店铺授权处理" },
  { value: "classification_queue", label: "类目处理" },
  { value: "attributes_queue", label: "属性处理" },
  { value: "variant_queue", label: "规格处理" },
  { value: "media_queue", label: "图片处理" },
  { value: "pricing_queue", label: "价格处理" },
  { value: "final_review_queue", label: "最终确认" },
  { value: "source_review_queue", label: "来源复核" },
  { value: "payload_rebuild_queue", label: "载荷重建" },
  { value: "manual_review_queue", label: "人工备注复核" },
  { value: "submit_ready_action_queue", label: "直接提交" },
];

const HOME_WORK_QUEUE_PRIORITY = [
  "submit_ready_queue",
  "repair_queue",
  "review_queue",
  "submit_failed_queue",
  "draft_queue",
  "generation_queue",
  "generation_failed_queue",
  "published_queue",
];

const HOME_ACTION_QUEUE_PRIORITY = [
  "submit_ready_action_queue",
  "final_review_queue",
  "pricing_queue",
  "media_queue",
  "attributes_queue",
  "classification_queue",
  "variant_queue",
  "source_review_queue",
  "manual_review_queue",
  "payload_rebuild_queue",
  "store_auth_queue",
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

export function descriptorOptions(
  items: ListingKitTaskFacetDescriptor[] | undefined,
  fallback: Array<{ value: string; label: string }>,
  emptyLabel: string,
) {
  if (!items?.length) {
    return fallback;
  }
  return [
    { value: "", label: emptyLabel },
    ...items.map((item) => ({
      value: item.key,
      label: item.label || item.key,
    })),
  ];
}

function descriptorLabel(
  key: string | undefined,
  items: ListingKitTaskFacetDescriptor[] | undefined,
  fallback: Array<{ value: string; label: string }>,
) {
  if (!key) {
    return "";
  }
  return (
    items?.find((item) => item.key === key)?.label ||
    fallback.find((item) => item.value === key)?.label ||
    key
  );
}

export function facetDescriptorLabel(
  key: string | undefined,
  items: ListingKitTaskFacetDescriptor[] | undefined,
  fallback: Array<{ value: string; label: string }>,
) {
  return descriptorLabel(key, items, fallback);
}

export function sheinWorkQueueLabel(
  key?: string,
  taxonomy?: ListingKitTaskListTaxonomy,
) {
  return descriptorLabel(key, taxonomy?.shein_work_queues, SHEIN_WORK_QUEUE_OPTIONS);
}

export function sheinActionQueueLabel(
  key?: string,
  taxonomy?: ListingKitTaskListTaxonomy,
) {
  return descriptorLabel(
    key,
    taxonomy?.shein_action_queues,
    SHEIN_ACTION_QUEUE_OPTIONS,
  );
}

export function queueTone(severity?: string) {
  switch (severity) {
    case "positive":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "negative":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-zinc-200 bg-zinc-50 text-zinc-600";
  }
}

export function taxonomySeverity(
  key: string | undefined,
  items: ListingKitTaskFacetDescriptor[] | undefined,
) {
  if (!key) {
    return undefined;
  }
  return items?.find((item) => item.key === key)?.severity;
}

export function buildSummaryEntries(
  counts: Record<string, number> | undefined,
  descriptors: ListingKitTaskFacetDescriptor[] | undefined,
) {
  if (!counts) {
    return [];
  }
  return Object.entries(counts)
    .filter(([, count]) => count > 0)
    .map(([key, count]) => {
      const descriptor = descriptors?.find((item) => item.key === key);
      return {
        key,
        label: descriptor?.label || key,
        count,
        severity: descriptor?.severity,
      };
    })
    .sort((left, right) => right.count - left.count);
}

export function buildHomeWorkQueueSummaryEntries(
  counts: Record<string, number> | undefined,
  descriptors: ListingKitTaskFacetDescriptor[] | undefined,
) {
  const entries = buildSummaryEntries(counts, descriptors);
  const priority = new Map(
    HOME_WORK_QUEUE_PRIORITY.map((key, index) => [key, index]),
  );

  return entries.sort((left, right) => {
    const leftPriority = priority.get(left.key) ?? Number.MAX_SAFE_INTEGER;
    const rightPriority = priority.get(right.key) ?? Number.MAX_SAFE_INTEGER;
    if (leftPriority !== rightPriority) {
      return leftPriority - rightPriority;
    }
    return right.count - left.count;
  });
}

export function buildHomeActionQueueSummaryEntries(
  counts: Record<string, number> | undefined,
  descriptors: ListingKitTaskFacetDescriptor[] | undefined,
) {
  const entries = buildSummaryEntries(counts, descriptors);
  const priority = new Map(
    HOME_ACTION_QUEUE_PRIORITY.map((key, index) => [key, index]),
  );

  return entries.sort((left, right) => {
    const leftPriority = priority.get(left.key) ?? Number.MAX_SAFE_INTEGER;
    const rightPriority = priority.get(right.key) ?? Number.MAX_SAFE_INTEGER;
    if (leftPriority !== rightPriority) {
      return leftPriority - rightPriority;
    }
    return right.count - left.count;
  });
}

export function buildFacetSummarySections(
  summary: ListingKitTaskListSummary | undefined,
  taxonomy: ListingKitTaskListTaxonomy | undefined,
) {
  if (!summary) {
    return [];
  }

  return [
    {
      title: "工作队列",
      filterKey: "shein_work_queue" as const,
      entries: buildSummaryEntries(
        summary.shein_work_queue_counts,
        taxonomy?.shein_work_queues,
      ).slice(0, 4),
    },
    {
      title: "处理动作",
      filterKey: "shein_action_queue" as const,
      entries: buildSummaryEntries(
        summary.shein_action_queue_counts,
        taxonomy?.shein_action_queues,
      ).slice(0, 4),
    },
    {
      title: "阻断项",
      filterKey: "shein_blocker_key" as const,
      entries: buildSummaryEntries(
        summary.shein_blocker_counts,
        taxonomy?.shein_blockers,
      ).slice(0, 4),
    },
    {
      title: "待确认",
      filterKey: "shein_warning_key" as const,
      entries: buildSummaryEntries(
        summary.shein_warning_counts,
        taxonomy?.shein_warnings,
      ).slice(0, 4),
    },
  ].filter((section) => section.entries.length > 0);
}
