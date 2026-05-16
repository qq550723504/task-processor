"use client";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { ReviewSlot } from "@/lib/types/listingkit";

export type WorkspacePreviewSuggestion = {
  slot: ReviewSlot;
  title: string;
  summary: string;
  ctaLabel: string;
};

export function WorkspacePreviewSuggestionCard({
  suggestion,
  onSelect,
}: {
  suggestion?: WorkspacePreviewSuggestion | null;
  onSelect: (slot: ReviewSlot) => void;
}) {
  if (!suggestion) {
    return null;
  }

  return (
    <Card className="border-amber-200 bg-amber-50/70 p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
            Suggested Preview
          </p>
          <h3 className="text-sm font-semibold text-zinc-950">
            {suggestion.title}
          </h3>
          <p className="text-sm leading-6 text-zinc-700">{suggestion.summary}</p>
        </div>
        <Button variant="secondary" onClick={() => onSelect(suggestion.slot)}>
          {suggestion.ctaLabel}
        </Button>
      </div>
    </Card>
  );
}
