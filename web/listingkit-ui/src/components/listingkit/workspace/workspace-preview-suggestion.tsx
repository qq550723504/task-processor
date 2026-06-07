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
    <Card className="border-amber-200 bg-amber-50/70 p-4 dark:border-amber-900 dark:bg-amber-950/30">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
            预览建议
          </p>
          <h3 className="text-sm font-semibold text-foreground">
            {suggestion.title}
          </h3>
          <p className="text-sm leading-6 text-foreground">{suggestion.summary}</p>
        </div>
        <Button variant="secondary" onClick={() => onSelect(suggestion.slot)}>
          {suggestion.ctaLabel}
        </Button>
      </div>
    </Card>
  );
}
