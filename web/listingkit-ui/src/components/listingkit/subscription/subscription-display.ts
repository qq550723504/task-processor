export const SUBSCRIPTION_MODULE_GUIDANCE: Record<string, string> = {
  store_management: "控制店铺、店铺资料与基础运营配置是否可用。",
  task_import: "控制任务导入与批量导入能力是否可用。",
  rules: "控制规则、映射和限制类配置能力是否可用。",
  operation_strategy: "控制运营策略相关配置能力是否可用。",
  studio: "控制生成任务、工作台和图片生产类能力。",
  oss_storage: "控制文件存储额度，适合按存储空间进行配额管理。",
};

export const SUBSCRIPTION_METRIC_LABELS: Record<string, { label: string; unit?: string }> = {
  design_jobs: { label: "设计任务额度", unit: "次" },
  product_image_jobs: { label: "商品图片任务额度", unit: "次" },
  storage_bytes: { label: "存储额度", unit: "字节" },
  uploaded_bytes: { label: "已上传存储用量", unit: "字节" },
  import_tasks: { label: "导入任务额度", unit: "条" },
};

export function subscriptionModuleSummary(moduleCode: string, fallback?: string) {
  return (
    SUBSCRIPTION_MODULE_GUIDANCE[moduleCode] ??
    fallback ??
    "控制该模块对应业务能力的开通状态。"
  );
}

export function subscriptionMetricDisplayName(key: string) {
  return SUBSCRIPTION_METRIC_LABELS[key]?.label ?? key;
}

export function subscriptionMetricUnit(key: string) {
  return SUBSCRIPTION_METRIC_LABELS[key]?.unit;
}

export function formatSubscriptionRecord(value?: Record<string, number>) {
  if (!value || Object.keys(value).length === 0) {
    return "-";
  }
  return Object.entries(value)
    .map(([key, count]) => `${subscriptionMetricDisplayName(key)}: ${formatSubscriptionMetricValue(key, count)}`)
    .join(", ");
}

export function formatSubscriptionMetricValue(key: string, value: number) {
  if (key === "storage_bytes" || key.endsWith("_bytes")) {
    return formatBytes(value);
  }
  return String(value);
}

export function formatSubscriptionDate(value?: string) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  const maximumFractionDigits = unitIndex === 0 ? 0 : 1;
  return `${new Intl.NumberFormat("zh-CN", { maximumFractionDigits }).format(size)} ${units[unitIndex]}`;
}

