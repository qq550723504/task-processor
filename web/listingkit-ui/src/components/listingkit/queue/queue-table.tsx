"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { presentScenePresetCompact } from "@/components/listingkit/shared/scene-preset-presentation";
import {
  deriveQueueItemAction,
  type QueueItemAction,
} from "@/components/listingkit/queue/queue-actions";
import {
  presentRecoveryDescriptor,
  presentRetryHint,
} from "@/components/listingkit/shared/hint-presentation";
import { derivePrimaryRecoveryDescriptor } from "@/components/listingkit/queue/queue-recovery";
import {
  presentQueueReviewStatus,
  presentQueueState,
} from "@/components/listingkit/shared/status-presentation";
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
      <Table className="min-w-full">
        <TableHeader className="bg-zinc-50">
          <TableRow className="text-xs uppercase tracking-[0.2em] hover:bg-transparent">
            <TableHead>Platform</TableHead>
            <TableHead>Slot</TableHead>
            <TableHead>State</TableHead>
            <TableHead>Quality</TableHead>
            <TableHead>Preview</TableHead>
            <TableHead>Review</TableHead>
            <TableHead>Scene</TableHead>
            <TableHead>Retry Hint</TableHead>
            <TableHead>Recovery</TableHead>
            <TableHead>Action</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody className="bg-white">
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
              <TableRow key={`${item.platform}-${item.slot}-${item.generation_task}`}>
                <TableCell className="text-sm capitalize text-zinc-900">
                  {item.platform}
                </TableCell>
                <TableCell className="text-sm capitalize text-zinc-700">
                  {item.slot}
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
                  <Badge variant={state.tone}>{state.label}</Badge>
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
                  {item.quality_grade_label ?? item.quality_grade}
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
                  {item.render_preview_available ? (
                    <Badge variant="success">Ready</Badge>
                  ) : (
                    <Badge variant="neutral">Pending</Badge>
                  )}
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
                  <Badge variant={reviewStatus.tone}>{reviewStatus.label}</Badge>
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
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
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
                  <div className="space-y-1">
                    <div className="font-medium text-zinc-900">{retryHint.label}</div>
                    <div className="text-xs leading-5 text-zinc-500">
                      {retryHint.description}
                    </div>
                  </div>
                </TableCell>
                <TableCell className="text-sm text-zinc-700">
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
                </TableCell>
                <TableCell>
                  <Button variant="secondary" onClick={() => onAction(item, action)}>
                    {action.label}
                  </Button>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </Card>
  );
}
