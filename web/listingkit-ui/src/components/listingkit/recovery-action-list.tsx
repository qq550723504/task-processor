"use client";

import { presentRecoveryDescriptor } from "@/components/listingkit/hint-presentation";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type { RecoveryDescriptor } from "@/lib/types/listingkit";

export function RecoveryActionList({
  descriptors,
  onSelect,
}: {
  descriptors?: RecoveryDescriptor[];
  onSelect: (descriptor: RecoveryDescriptor) => void;
}) {
  if (!descriptors?.length) {
    return null;
  }

  return (
    <Card className="p-4">
      <div className="space-y-3">
        <h3 className="text-base font-semibold text-zinc-950">
          Recommended Recovery
        </h3>
        {descriptors.map((descriptor, index) => {
          const copy = presentRecoveryDescriptor(descriptor);

          return (
            <div
              className="flex items-center justify-between gap-3 rounded-xl border border-zinc-200 px-3 py-3"
              key={`${descriptor.role}-${descriptor.platform}-${descriptor.slot}-${index}`}
            >
              <div className="space-y-1">
                <p className="text-sm font-medium text-zinc-950">{copy.title}</p>
                <p className="text-sm leading-6 text-zinc-600">
                  {copy.description}
                </p>
                <p className="text-xs uppercase tracking-[0.16em] text-zinc-500">
                  {copy.metaLabel}
                </p>
              </div>
              <Button tone="secondary" onClick={() => onSelect(descriptor)}>
                {copy.ctaLabel}
              </Button>
            </div>
          );
        })}
      </div>
    </Card>
  );
}
