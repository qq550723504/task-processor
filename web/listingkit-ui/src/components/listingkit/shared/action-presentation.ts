import type { RecoverySummary } from "@/lib/types/listingkit";

function titleCaseWords(value: string) {
  return value
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

const actionCtaKindLabels: Record<string, string> = {
  review: "Review",
  retry: "Retry",
  refresh: "Refresh",
  monitor: "Monitor",
};

const recoveryCtaKindLabels: Record<string, string> = {
  review: "Review fallback",
  retry: "Retry now",
  refresh: "Refresh state",
  monitor: "Check later",
};

const severityLabels: Record<string, string> = {
  high: "High severity",
  medium: "Medium severity",
  low: "Low severity",
};

const urgencyLabels: Record<string, string> = {
  now: "act now",
  soon: "act soon",
  later: "check later",
};

export function presentActionCtaKind(kind?: string) {
  if (!kind) {
    return "Review";
  }

  return actionCtaKindLabels[kind] ?? titleCaseWords(kind);
}

export function presentRecoveryCtaKind(kind?: string) {
  if (!kind) {
    return "Review fallback";
  }

  return recoveryCtaKindLabels[kind] ?? titleCaseWords(kind);
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
    title: summary.title ?? "Recovery needed",
    summary: summary.summary,
    metaLabel: presentRecoveryMetaLabel(summary.severity, summary.urgency),
    ctaLabel: presentRecoveryCtaKind(summary.cta_kind),
  };
}
