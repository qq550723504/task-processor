import {
  presentRecoveryCtaKind,
  presentRecoveryMetaLabel,
  presentRecoverySummaryText,
  presentRecoveryTitle,
} from "@/components/listingkit/shared/action-presentation";
import type { RecoveryDescriptor } from "@/lib/types/listingkit";

type RetryHintPresentation = {
  label: string;
  description: string;
};

type RecoveryDescriptorPresentation = {
  title: string;
  description: string;
  ctaLabel: string;
  metaLabel: string;
};

function titleCaseWords(value: string) {
  return value
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function sentenceCaseWords(value: string) {
  const normalized = value
    .split(/[_\s-]+/)
    .filter(Boolean)
    .join(" ");
  if (!normalized) {
    return "Unknown";
  }
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
}

const retryHintCopy: Record<string, RetryHintPresentation> = {
  retry_dispatch: {
    label: "重新生成当前内容",
    description: "当前生成步骤可以立即重试。",
  },
  refresh_revision: {
    label: "刷新任务状态",
    description: "先刷新最新任务状态，再尝试下一步操作。",
  },
  review_fallback: {
    label: "检查兜底结果",
    description: "当前有一份兜底结果可用，建议先检查后再决定是否重试。",
  },
  wait_for_generation: {
    label: "等待生成继续推进",
    description: "任务仍在继续生成，等更多结果产出后再继续处理。",
  },
  no_retry: {
    label: "暂时无需重试",
    description: "当前这一项不建议继续重试。",
  },
};

const recoveryCopy: Record<string, Omit<RecoveryDescriptorPresentation, "metaLabel">> = {
  review_fallback: {
    title: "使用兜底结果继续检查",
    description: "当前有一份兜底结果可用，建议先检查后再决定是否重试。",
    ctaLabel: "检查恢复项",
  },
  retry_dispatch: {
    title: "重新生成当前内容",
    description: "当前失败的生成步骤可以直接重试。",
    ctaLabel: "立即重试",
  },
  refresh_revision: {
    title: "刷新任务状态",
    description: "先刷新到最新任务状态，再决定下一步操作。",
    ctaLabel: "刷新状态",
  },
  wait_for_generation: {
    title: "等待生成继续推进",
    description: "需要等待更多生成结果后，这一项才能继续推进。",
    ctaLabel: "稍后再看",
  },
};

export function presentRetryHint(hint?: string): RetryHintPresentation {
  if (!hint) {
    return {
      label: "暂无重试建议",
      description: "当前这一项还没有可执行的重试建议。",
    };
  }

  return (
    retryHintCopy[hint] ?? {
      label: sentenceCaseWords(hint),
      description: "请先按照当前任务提示处理，再决定是否重试。",
    }
  );
}

export function presentRecoveryDescriptor(
  descriptor?: RecoveryDescriptor | null,
): RecoveryDescriptorPresentation {
  const hint = descriptor?.recovery_hint ?? "review_fallback";
  const copy = recoveryCopy[hint] ?? {
    title: presentRecoveryTitle(titleCaseWords(hint)),
    description: "请先按照当前恢复提示处理这一项。",
    ctaLabel: presentRecoveryCtaKind(descriptor?.recovery_cta_kind ?? "review"),
  };

  return {
    ...copy,
    title: presentRecoveryTitle(copy.title),
    description: presentRecoverySummaryText(copy.description) ?? copy.description,
    ctaLabel: descriptor?.recovery_cta_kind
      ? presentRecoveryCtaKind(descriptor.recovery_cta_kind)
      : copy.ctaLabel,
    metaLabel: presentRecoveryMetaLabel(
      descriptor?.recovery_severity,
      descriptor?.recovery_urgency,
    ),
  };
}
