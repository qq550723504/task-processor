"use client";

import {
  presentActionCtaKind,
  presentRecoverySummary,
} from "@/components/listingkit/shared/action-presentation";
import { Card } from "@/components/shared/card";
import type {
  AssetGenerationOverview,
  ReviewSummary,
} from "@/lib/types/listingkit";

const overviewMetrics = [
  {
    key: "previewable_items" as const,
    label: "Previewable",
  },
  {
    key: "retryable_count" as const,
    label: "Retryable",
  },
  {
    key: "approved_sections" as const,
    label: "Approved",
  },
  {
    key: "review_pending_sections" as const,
    label: "Pending",
  },
] as const;

const reviewMetrics = [
  { key: "approved_sections" as const, label: "Approved" },
  { key: "deferred_sections" as const, label: "Deferred" },
  { key: "pending_sections" as const, label: "Pending" },
] as const;

export function WorkspaceOverviewPanel({
  overview,
  reviewSummary,
}: {
  overview?: AssetGenerationOverview | null;
  reviewSummary?: ReviewSummary | null;
}) {
  if (!overview && !reviewSummary) {
    return null;
  }

  return (
    <div className="grid gap-3 md:grid-cols-4 xl:grid-cols-7">
      {overviewMetrics.map((metric) => (
        <Card className="p-4" key={metric.key}>
          <p className="text-xs uppercase tracking-[0.18em] text-zinc-500">
            {metric.label}
          </p>
          <p className="mt-2 text-2xl font-semibold text-zinc-950">
            {overview?.[metric.key] ?? 0}
          </p>
        </Card>
      ))}
      {reviewMetrics.map((metric) => (
        <Card className="p-4" key={`review-${metric.key}`}>
          <p className="text-xs uppercase tracking-[0.18em] text-zinc-500">
            {metric.label}
          </p>
          <p className="mt-2 text-2xl font-semibold text-zinc-950">
            {reviewSummary?.[metric.key] ?? 0}
          </p>
        </Card>
      ))}
      {overview?.resolved_action_summary ? (
        <Card className="p-4 md:col-span-2 xl:col-span-4">
          <p className="text-xs uppercase tracking-[0.18em] text-zinc-500">
            Recommended next step
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <p className="text-lg font-semibold text-zinc-950">
              {overview.resolved_action_summary.title}
            </p>
            <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
              {presentActionCtaKind(overview.resolved_action_summary.cta_kind)}
            </span>
          </div>
          {overview.resolved_action_summary.summary ? (
            <p className="mt-2 text-sm leading-6 text-zinc-600">
              {overview.resolved_action_summary.summary}
            </p>
          ) : null}
        </Card>
      ) : null}
      {overview?.recovery_summary ? (
        (() => {
          const recovery = presentRecoverySummary(overview.recovery_summary);
          if (!recovery) {
            return null;
          }

          return (
            <Card className="border-amber-200 bg-amber-50/60 p-4 md:col-span-2 xl:col-span-3">
              <p className="text-xs uppercase tracking-[0.18em] text-amber-700">
                Recovery focus
              </p>
              <p className="mt-2 text-lg font-semibold text-zinc-950">
                {recovery.title}
              </p>
              {recovery.summary ? (
                <p className="mt-2 text-sm leading-6 text-zinc-700">
                  {recovery.summary}
                </p>
              ) : null}
              <div className="mt-3 flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
                <span>{recovery.metaLabel}</span>
                <span className="rounded-full border border-amber-200 bg-amber-100 px-2 py-1 text-[10px] font-semibold tracking-[0.16em] text-amber-800">
                  {recovery.ctaLabel}
                </span>
              </div>
            </Card>
          );
        })()
      ) : null}
    </div>
  );
}
