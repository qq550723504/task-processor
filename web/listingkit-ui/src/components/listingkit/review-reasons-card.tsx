"use client";

import { Card } from "@/components/shared/card";
import { extractTaskReviewReasons } from "@/components/listingkit/task-review-reasons";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function ReviewReasonsCard({
  task,
  limit = 4,
}: {
  task?: ListingKitTaskResult | null;
  limit?: number;
}) {
  if (task?.status !== "needs_review") {
    return null;
  }

  const reasons = extractTaskReviewReasons(task);
  if (reasons.length === 0) {
    return null;
  }

  const visibleReasons = reasons.slice(0, limit);
  const hiddenCount = Math.max(reasons.length - visibleReasons.length, 0);

  return (
    <Card className="border-amber-200 bg-amber-50/60 p-5">
      <div className="space-y-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
            Review focus
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            This task completed with review blockers. Resolve or confirm these items before final approval.
          </p>
        </div>
        <ul className="space-y-2">
          {visibleReasons.map((reason) => (
            <li
              className="rounded-2xl border border-amber-200 bg-white/70 px-4 py-3 text-sm leading-6 text-zinc-700"
              key={reason}
            >
              {reason}
            </li>
          ))}
        </ul>
        {hiddenCount > 0 ? (
          <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">
            {hiddenCount} more review reason{hiddenCount === 1 ? "" : "s"} in task details
          </p>
        ) : null}
      </div>
    </Card>
  );
}
