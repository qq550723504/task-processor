import {
  presentRecoveryCtaKind,
  presentRecoveryMetaLabel,
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
    label: "Retry generation",
    description: "The generation step can be retried immediately.",
  },
  refresh_revision: {
    label: "Refresh task revision",
    description: "Reload the latest task state before trying another action.",
  },
  review_fallback: {
    label: "Review fallback",
    description: "A fallback result is available and should be reviewed before retrying.",
  },
  wait_for_generation: {
    label: "Wait for generation",
    description: "The task is still producing assets. Retry after more output is available.",
  },
  no_retry: {
    label: "No retry needed",
    description: "No additional retry action is recommended for this item.",
  },
};

const recoveryCopy: Record<string, Omit<RecoveryDescriptorPresentation, "metaLabel">> = {
  review_fallback: {
    title: "Use fallback review",
    description: "A fallback result is available and should be reviewed before retrying.",
    ctaLabel: "Review fallback",
  },
  retry_dispatch: {
    title: "Retry generation",
    description: "The failed generation step can be retried now.",
    ctaLabel: "Retry now",
  },
  refresh_revision: {
    title: "Refresh task state",
    description: "Reload the latest revision before deciding on the next action.",
    ctaLabel: "Refresh state",
  },
  wait_for_generation: {
    title: "Wait for generation",
    description: "More generation output is expected before this item can move forward.",
    ctaLabel: "Check later",
  },
};

export function presentRetryHint(hint?: string): RetryHintPresentation {
  if (!hint) {
    return {
      label: "No retry guidance",
      description: "No retry recommendation is available for this item.",
    };
  }

  return (
    retryHintCopy[hint] ?? {
      label: sentenceCaseWords(hint),
      description: "Follow the current task guidance before retrying.",
    }
  );
}

export function presentRecoveryDescriptor(
  descriptor?: RecoveryDescriptor | null,
): RecoveryDescriptorPresentation {
  const hint = descriptor?.recovery_hint ?? "review_fallback";
  const copy = recoveryCopy[hint] ?? {
    title: titleCaseWords(hint),
    description: "Follow the current recovery guidance for this resource.",
    ctaLabel: presentRecoveryCtaKind(descriptor?.recovery_cta_kind ?? "review"),
  };

  return {
    ...copy,
    ctaLabel: descriptor?.recovery_cta_kind
      ? presentRecoveryCtaKind(descriptor.recovery_cta_kind)
      : copy.ctaLabel,
    metaLabel: presentRecoveryMetaLabel(
      descriptor?.recovery_severity,
      descriptor?.recovery_urgency,
    ),
  };
}
