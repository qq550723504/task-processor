"use client";

import {
  presentActionCtaKind,
  presentResolvedActionSummary,
  presentResolvedActionTitle,
  presentRecoverySummary,
} from "@/components/listingkit/shared/action-presentation";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import type {
  AssetGenerationOverview,
  ReviewSummary,
} from "@/lib/types/listingkit";

const overviewMetrics = [
  {
    key: "previewable_items" as const,
    label: "可预览",
  },
  {
    key: "retryable_count" as const,
    label: "可重试",
  },
  {
    key: "approved_sections" as const,
    label: "已通过",
  },
  {
    key: "review_pending_sections" as const,
    label: "待处理",
  },
] as const;

const reviewMetrics = [
  { key: "approved_sections" as const, label: "已通过" },
  { key: "deferred_sections" as const, label: "已延后" },
  { key: "pending_sections" as const, label: "待处理" },
] as const;

export function WorkspaceOverviewPanel({
  overview,
  reviewSummary,
}: {
  overview?: AssetGenerationOverview | null;
  reviewSummary?: ReviewSummary | null;
}) {
  const visibleOverviewMetrics = overviewMetrics.filter(
    (metric) => (overview?.[metric.key] ?? 0) > 0,
  );
  const visibleReviewMetrics = reviewMetrics.filter(
    (metric) => (reviewSummary?.[metric.key] ?? 0) > 0,
  );
  const resolvedActionSummary = overview?.resolved_action_summary;
  const recoverySummary = overview?.recovery_summary;
  const hasResolvedAction = Boolean(resolvedActionSummary);
  const hasRecoverySummary = Boolean(recoverySummary);

  if (
    visibleOverviewMetrics.length === 0 &&
    visibleReviewMetrics.length === 0 &&
    !hasResolvedAction &&
    !hasRecoverySummary
  ) {
    return null;
  }

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-7">
      {visibleOverviewMetrics.map((metric) => (
        <Card className="p-4" key={metric.key}>
          <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
            {metric.label}
          </p>
          <p className="mt-2 text-2xl font-semibold text-foreground">
            {overview?.[metric.key] ?? 0}
          </p>
        </Card>
      ))}
      {visibleReviewMetrics.map((metric) => (
        <Card className="p-4" key={`review-${metric.key}`}>
          <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
            {metric.label}
          </p>
          <p className="mt-2 text-2xl font-semibold text-foreground">
            {reviewSummary?.[metric.key] ?? 0}
          </p>
        </Card>
      ))}
      {hasResolvedAction ? (
        <Card className="p-4 sm:col-span-2 xl:col-span-2 2xl:col-span-4">
          <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
            当前建议
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <p className="text-lg font-semibold text-foreground">
              {presentResolvedActionTitle(resolvedActionSummary?.title)}
            </p>
            <Badge
              className="rounded-full px-2 py-1 text-[10px] uppercase tracking-[0.16em]"
              variant="neutral"
            >
              {presentActionCtaKind(resolvedActionSummary?.cta_kind)}
            </Badge>
          </div>
          {resolvedActionSummary?.summary ? (
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              {presentResolvedActionSummary(resolvedActionSummary.summary)}
            </p>
          ) : null}
        </Card>
      ) : null}
      {hasRecoverySummary ? (
        (() => {
          const recovery = presentRecoverySummary(recoverySummary);
          if (!recovery) {
            return null;
          }

          return (
            <Card className="border-amber-200 bg-amber-50/60 p-4 dark:border-amber-900 dark:bg-amber-950/30 sm:col-span-2 xl:col-span-2 2xl:col-span-3">
              <p className="text-xs uppercase tracking-[0.18em] text-amber-700">
                恢复重点
              </p>
              <p className="mt-2 text-lg font-semibold text-foreground">
                {recovery.title}
              </p>
              {recovery.summary ? (
                <p className="mt-2 text-sm leading-6 text-foreground">
                  {recovery.summary}
                </p>
              ) : null}
              <div className="mt-3 flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
                <span>{recovery.metaLabel}</span>
                <Badge
                  className="rounded-full px-2 py-1 text-[10px] tracking-[0.16em]"
                  variant="warning"
                >
                  {recovery.ctaLabel}
                </Badge>
              </div>
            </Card>
          );
        })()
      ) : null}
    </div>
  );
}
