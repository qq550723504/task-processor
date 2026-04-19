"use client";

import { Badge } from "@/components/shared/badge";
import { Card } from "@/components/shared/card";
import { cn } from "@/lib/utils/cn";
import type { ReviewSlot } from "@/lib/types/listingkit";

export function SlotNavigationList({
  slots,
  selectedSlot,
  onSelect,
}: {
  slots?: ReviewSlot[];
  selectedSlot?: string;
  onSelect: (slot: ReviewSlot) => void;
}) {
  return (
    <div className="space-y-2">
      {(slots ?? []).map((slot) => (
        <button
          key={`${slot.platform}-${slot.slot}`}
          className="w-full text-left"
          onClick={() => onSelect(slot)}
          type="button"
        >
          <Card
            className={cn(
              "p-3 transition hover:border-zinc-400",
              selectedSlot === slot.slot && "border-zinc-950",
            )}
          >
            <div className="flex items-center justify-between gap-3">
              <div>
                <h4 className="text-sm font-semibold capitalize text-zinc-950">
                  {slot.slot}
                </h4>
                <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">
                  {slot.quality_grade_label ?? slot.quality_grade ?? slot.state}
                </p>
              </div>
              {slot.render_preview_available ? (
                <Badge tone="success">Preview</Badge>
              ) : (
                <Badge tone="neutral">No Preview</Badge>
              )}
            </div>
          </Card>
        </button>
      ))}
    </div>
  );
}
