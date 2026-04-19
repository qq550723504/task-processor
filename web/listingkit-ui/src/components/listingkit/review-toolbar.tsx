"use client";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type { ReviewToolbar as ReviewToolbarType, ToolbarAction } from "@/lib/types/listingkit";

export function ReviewToolbar({
  toolbar,
  onAction,
}: {
  toolbar?: ReviewToolbarType | null;
  onAction: (action: ToolbarAction) => void;
}) {
  if (!toolbar) {
    return null;
  }

  const actions = [
    ...(toolbar.section_actions ?? []),
    ...(toolbar.preview_actions ?? []),
  ];

  return (
    <Card className="p-4">
      <div className="space-y-4">
        <div>
          <h3 className="text-base font-semibold text-zinc-950">
            {toolbar.platform} / {toolbar.slot}
          </h3>
          <p className="text-sm leading-6 text-zinc-600">
            {toolbar.capability} · {toolbar.visual_mode}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {actions.map((action) => (
            <Button
              key={action.key}
              tone={action.kind === "workflow" ? "primary" : "secondary"}
              disabled={!action.enabled}
              onClick={() => onAction(action)}
            >
              {action.label}
            </Button>
          ))}
        </div>
      </div>
    </Card>
  );
}
