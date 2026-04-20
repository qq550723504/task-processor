"use client";

import { Badge } from "@/components/shared/badge";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { presentScenePresetCompact } from "@/components/listingkit/scene-preset-presentation";
import {
  deriveQueueItemAction,
  type QueueItemAction,
} from "@/components/listingkit/queue-actions";
import {
  presentRecoveryDescriptor,
  presentRetryHint,
} from "@/components/listingkit/hint-presentation";
import { derivePrimaryRecoveryDescriptor } from "@/components/listingkit/queue-recovery";
import {
  presentQueueReviewStatus,
  presentQueueState,
} from "@/components/listingkit/status-presentation";
import type { QueueItem } from "@/lib/types/listingkit";

export function QueueTable({
  items,
  onAction,
}: {
  items?: QueueItem[];
  onAction: (item: QueueItem, action: QueueItemAction) => void;
}) {
  return (
    <Card className="overflow-hidden">
      <table className="min-w-full divide-y divide-zinc-200">
        <thead className="bg-zinc-50">
          <tr className="text-left text-xs uppercase tracking-[0.2em] text-zinc-500">
            <th className="px-4 py-3">Platform</th>
            <th className="px-4 py-3">Slot</th>
            <th className="px-4 py-3">State</th>
            <th className="px-4 py-3">Quality</th>
            <th className="px-4 py-3">Preview</th>
            <th className="px-4 py-3">Review</th>
            <th className="px-4 py-3">Scene</th>
            <th className="px-4 py-3">Retry Hint</th>
            <th className="px-4 py-3">Recovery</th>
            <th className="px-4 py-3">Action</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-100 bg-white">
          {(items ?? []).map((item) => {
            const action = deriveQueueItemAction(item);
            const primaryRecovery = derivePrimaryRecoveryDescriptor(
              item.resource_descriptors,
            );
            const state = presentQueueState(item);
            const reviewStatus = presentQueueReviewStatus(item);
            const retryHint = presentRetryHint(item.retry_hint);
            const recovery = presentRecoveryDescriptor(primaryRecovery);
            const scenePreset = presentScenePresetCompact(item.scene_preset);

            return (
              <tr key={`${item.platform}-${item.slot}-${item.generation_task}`}>
                <td className="px-4 py-3 text-sm capitalize text-zinc-900">
                  {item.platform}
                </td>
                <td className="px-4 py-3 text-sm capitalize text-zinc-700">
                  {item.slot}
                </td>
                <td className="px-4 py-3 text-sm text-zinc-700">
                  <Badge tone={state.tone}>{state.label}</Badge>
                </td>
                <td className="px-4 py-3 text-sm text-zinc-700">
                  {item.quality_grade_label ?? item.quality_grade}
                </td>
                <td className="px-4 py-3 text-sm text-zinc-700">
                  {item.render_preview_available ? (
                    <Badge tone="success">Ready</Badge>
                  ) : (
                    <Badge tone="neutral">Pending</Badge>
                  )}
                </td>
                <td className="px-4 py-3 text-sm text-zinc-700">
                  <Badge tone={reviewStatus.tone}>{reviewStatus.label}</Badge>
                </td>
                <td className="px-4 py-3 text-sm text-zinc-700">
                  {scenePreset ? (
                    <div className="space-y-1">
                      <div className="font-medium text-zinc-900">{scenePreset.title}</div>
                      <div className="text-xs uppercase tracking-[0.16em] text-zinc-500">
                        {scenePreset.detail ?? "scene preset"}
                      </div>
                    </div>
                  ) : (
                    "—"
                  )}
                </td>
                <td className="px-4 py-3 text-sm text-zinc-700">
                  <div className="space-y-1">
                    <div className="font-medium text-zinc-900">{retryHint.label}</div>
                    <div className="text-xs leading-5 text-zinc-500">
                      {retryHint.description}
                    </div>
                  </div>
                </td>
                <td className="px-4 py-3 text-sm text-zinc-700">
                  {primaryRecovery ? (
                    <div className="space-y-1">
                      <div className="font-medium text-zinc-900">{recovery.title}</div>
                      <div className="text-xs uppercase tracking-[0.16em] text-zinc-500">
                        {recovery.metaLabel}
                      </div>
                    </div>
                  ) : (
                    "—"
                  )}
                </td>
                <td className="px-4 py-3">
                  <Button tone="secondary" onClick={() => onAction(item, action)}>
                    {action.label}
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </Card>
  );
}
