import type { RecoverySummary } from "@/lib/types/listingkit";

function titleCaseWords(value: string) {
  return value
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

const actionCtaKindLabels: Record<string, string> = {
  review: "去检查",
  retry: "重试",
  refresh: "刷新",
  monitor: "继续查看",
};

const recoveryCtaKindLabels: Record<string, string> = {
  review: "检查恢复项",
  retry: "立即重试",
  refresh: "刷新状态",
  monitor: "稍后再看",
};

const severityLabels: Record<string, string> = {
  high: "高优先级",
  medium: "中优先级",
  low: "低优先级",
};

const urgencyLabels: Record<string, string> = {
  now: "立即处理",
  soon: "尽快处理",
  later: "稍后处理",
};

const resolvedActionTitleLabels: Record<string, string> = {
  "Review Previews": "检查预览结果",
};

const resolvedActionSummaryLabels: Record<string, string> = {
  "Review the current section and preview focus.":
    "先检查当前分区和预览焦点，再继续处理。",
};

const recoveryTitleLabels: Record<string, string> = {
  "Use fallback review": "使用兜底结果继续检查",
  "Retry generation": "重新生成当前内容",
  "Refresh task state": "刷新任务状态",
  "Wait for generation": "等待生成继续推进",
};

const recoverySummaryLabels: Record<string, string> = {
  "A fallback result is available and should be reviewed first.":
    "当前有一份兜底结果可用，建议先检查后再决定是否重试。",
  "A fallback result is available and should be reviewed before retrying.":
    "当前有一份兜底结果可用，建议先检查后再决定是否重试。",
  "The failed generation step can be retried now.":
    "当前失败的生成步骤可以直接重试。",
  "Reload the latest revision before deciding on the next action.":
    "先刷新到最新任务状态，再决定下一步操作。",
  "More generation output is expected before this item can move forward.":
    "需要等待更多生成结果后，这一项才能继续推进。",
};

export function presentActionCtaKind(kind?: string) {
  if (!kind) {
    return "去检查";
  }

  return actionCtaKindLabels[kind] ?? titleCaseWords(kind);
}

export function presentResolvedActionTitle(title?: string) {
  if (!title) {
    return "当前建议";
  }

  return resolvedActionTitleLabels[title.trim()] ?? title;
}

export function presentResolvedActionSummary(summary?: string | null) {
  if (!summary) {
    return summary;
  }

  return resolvedActionSummaryLabels[summary.trim()] ?? summary;
}

export function presentRecoveryCtaKind(kind?: string) {
  if (!kind) {
    return "检查恢复项";
  }

  return recoveryCtaKindLabels[kind] ?? titleCaseWords(kind);
}

export function presentRecoveryTitle(title?: string) {
  if (!title) {
    return "需要继续处理";
  }

  return recoveryTitleLabels[title.trim()] ?? title;
}

export function presentRecoverySummaryText(summary?: string | null) {
  if (!summary) {
    return summary;
  }

  return recoverySummaryLabels[summary.trim()] ?? summary;
}

export function presentRecoveryMetaLabel(
  severity?: string,
  urgency?: string,
) {
  const severityLabel =
    severityLabels[severity ?? "medium"] ?? titleCaseWords(severity ?? "medium");
  const urgencyLabel =
    urgencyLabels[urgency ?? "now"] ?? titleCaseWords(urgency ?? "now").toLowerCase();

  return `${severityLabel} / ${urgencyLabel}`;
}

export function presentRecoverySummary(summary?: RecoverySummary | null) {
  if (!summary) {
    return undefined;
  }

  return {
    title: presentRecoveryTitle(summary.title),
    summary: presentRecoverySummaryText(summary.summary),
    metaLabel: presentRecoveryMetaLabel(summary.severity, summary.urgency),
    ctaLabel: presentRecoveryCtaKind(summary.cta_kind),
  };
}
