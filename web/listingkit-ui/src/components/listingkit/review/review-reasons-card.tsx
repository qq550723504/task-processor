"use client";

import Link from "next/link";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  buildTaskReviewActionLinks,
  extractTaskReviewReasons,
} from "@/components/listingkit/tasks/task-review-reasons";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function ReviewReasonsCard({
  task,
  taskId,
  limit = 4,
  onRepairSDS,
}: {
  task?: ListingKitTaskResult | null;
  taskId?: string;
  limit?: number;
  onRepairSDS?: () => void;
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
  const actionLinks = taskId ? buildTaskReviewActionLinks(taskId, task) : [];

  return (
    <Card className="border-amber-200 bg-amber-50/60 p-5">
      <div className="space-y-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
            审核重点
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            当前任务仍有待确认项。请先处理或确认这些内容，再进入最终提交。
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
        {actionLinks.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {actionLinks.map((action) => (
              <Button asChild key={action.key} size="sm" variant="secondary">
                <Link href={action.href}>{action.label}</Link>
              </Button>
            ))}
          </div>
        ) : null}
        {onRepairSDS ? (
          <div>
            <Button onClick={onRepairSDS} size="sm" type="button" variant="secondary">
              修复并重试 SDS
            </Button>
          </div>
        ) : null}
        {hiddenCount > 0 ? (
          <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">
            还有 {hiddenCount} 条待确认原因，请在任务详情中继续查看
          </p>
        ) : null}
      </div>
    </Card>
  );
}
