import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioFailedTask,
  SheinStudioRejectedTask,
} from "@/lib/types/shein-studio";
import type { GroupedSheinTaskCreationWarning } from "@/lib/shein-studio/create-review-tasks";

export function resolveTaskCreationStartValidation({
  activeSelection,
  approvedCount,
  sheinStoreId,
}: {
  activeSelection?: SDSProductVariantSelection;
  approvedCount: number;
  sheinStoreId: string;
}) {
  if (!activeSelection?.variantId) {
    return { error: "请先选择 SDS 变体。" };
  }
  if (!sheinStoreId.trim()) {
    return { error: "请先选择批次店铺。" };
  }
  if (approvedCount === 0) {
    return { error: "请至少批准 1 个款式后再创建 SHEIN 任务。" };
  }
  return null;
}

export function buildBatchTaskCreationFailureSummary(
  failedTasks: SheinStudioFailedTask[],
  rejectedTasks: SheinStudioRejectedTask[] = [],
) {
  if (failedTasks.length === 0 && rejectedTasks.length === 0) {
    return "";
  }
  const rejectedPreview = rejectedTasks
    .slice(0, 3)
    .map(
      (task) =>
        `${task.title?.trim() || task.designId}: ${
          task.reasonCode ? `${task.reasonCode} · ` : ""
        }${task.message ?? "候选不满足创建条件"}`,
    )
    .join("；");
  const failedPreview = failedTasks
    .slice(0, Math.max(0, 3 - rejectedTasks.length))
    .map(
      (task) =>
        `${task.title}: ${task.reasonCode ? `${task.reasonCode} · ` : ""}${
          task.message
        }`,
    )
    .join("；");
  const preview = [rejectedPreview, failedPreview].filter(Boolean).join("；");
  const total = rejectedTasks.length + failedTasks.length;
  const suffix = total > 3 ? ` 等 ${total} 个任务` : "";
  if (failedTasks.length === 0) {
    return `部分任务被拒绝：${preview}${suffix}`;
  }
  if (rejectedTasks.length === 0) {
    return `部分任务创建失败：${preview}${suffix}`;
  }
  return `部分任务被拒绝或创建失败：${preview}${suffix}`;
}

export function buildGroupedTaskCreationWarningSummary(
  warnings: GroupedSheinTaskCreationWarning[],
) {
  if (warnings.length === 0) {
    return "";
  }
  const labels = warnings.map((warning) => warning.label.trim()).filter(Boolean);
  const preview = labels.slice(0, 5).join("、");
  const suffix =
    labels.length > 5
      ? ` 等 ${labels.length} 款商品`
      : labels.length > 1
        ? ` 共 ${labels.length} 款商品`
        : "";
  return `有 ${warnings.length} 款商品因为没有匹配到自己的款式图而被跳过：${preview}${suffix}。这些商品不会创建错误任务，你可以回到生成区补图后再重试。`;
}

export function groupTaskCreationSelectionsByStore(
  items: GroupedSDSSelectionEligibility[],
) {
  const byStore = new Map<
    string,
    { sheinStoreId: string; items: GroupedSDSSelectionEligibility[] }
  >();
  for (const item of items) {
    const key = item.sheinStoreId.trim();
    const existing = byStore.get(key);
    if (existing) {
      existing.items.push(item);
      continue;
    }
    byStore.set(key, { sheinStoreId: key, items: [item] });
  }
  return [...byStore.values()];
}
