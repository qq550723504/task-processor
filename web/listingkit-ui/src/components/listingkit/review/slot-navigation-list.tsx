"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils/cn";
import type { ReviewSlot } from "@/lib/types/listingkit";

function slotNavigationKey(slot: ReviewSlot, index: number) {
  return [
    slot.platform ?? "unknown-platform",
    slot.slot ?? "unknown-slot",
    slot.asset_id ?? slot.purpose ?? slot.quality_grade ?? `idx-${index}`,
  ].join(":");
}

export function SlotNavigationList({
  slots,
  selectedSlot,
  selectedAssetId,
  onSelect,
}: {
  slots?: ReviewSlot[];
  selectedSlot?: string;
  selectedAssetId?: string;
  onSelect: (slot: ReviewSlot) => void;
}) {
  if ((slots?.length ?? 0) <= 1) {
    return null;
  }

  return (
    <div className="space-y-2">
      {(slots ?? []).map((slot, index) => (
        <Button
          key={slotNavigationKey(slot, index)}
          variant="ghost"
          className="h-auto w-full justify-start p-0 text-left"
          onClick={() => onSelect(slot)}
          type="button"
        >
          <Card
            className={cn(
              "p-3 transition hover:border-zinc-400",
              selectedAssetId
                ? selectedAssetId === slot.asset_id && "border-zinc-950"
                : selectedSlot === slot.slot && "border-zinc-950",
            )}
          >
            <div className="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0">
                <h4 className="break-words text-sm font-semibold text-zinc-950">
                  {slot.template_label ?? slot.purpose ?? slot.slot}
                </h4>
                <p className="break-words text-xs uppercase tracking-[0.16em] text-zinc-500">
                  {[slot.slot, slot.purpose].filter(Boolean).join(" / ")}
                </p>
                <p className="break-words text-xs uppercase tracking-[0.16em] text-zinc-500">
                  {slot.quality_grade_label ?? slot.quality_grade ?? slot.state}
                </p>
              </div>
              <div className="shrink-0 self-start">
                {slot.render_preview_available ? (
                  <Badge variant="success">Preview</Badge>
                ) : (
                  <Badge variant="neutral">No Preview</Badge>
                )}
              </div>
            </div>
          </Card>
        </Button>
      ))}
    </div>
  );
}
