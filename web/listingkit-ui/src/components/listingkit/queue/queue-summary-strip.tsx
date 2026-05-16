"use client";

import { Card } from "@/components/ui/card";
import type { QueueSummary } from "@/lib/types/listingkit";

const metrics = [
  { key: "total_items", label: "Total" },
  { key: "previewable_items", label: "Previewable" },
  { key: "retryable_items", label: "Retryable" },
  { key: "approved_sections", label: "Approved" },
  { key: "review_pending_sections", label: "Pending Review" },
] as const;

export function QueueSummaryStrip({ summary }: { summary?: QueueSummary | null }) {
  if (!summary) {
    return null;
  }

  return (
    <div className="grid gap-3 md:grid-cols-5">
      {metrics.map((metric) => (
        <Card className="p-4" key={metric.key}>
          <p className="text-xs uppercase tracking-[0.18em] text-zinc-500">
            {metric.label}
          </p>
          <p className="mt-2 text-2xl font-semibold text-zinc-950">
            {summary[metric.key]}
          </p>
        </Card>
      ))}
    </div>
  );
}
