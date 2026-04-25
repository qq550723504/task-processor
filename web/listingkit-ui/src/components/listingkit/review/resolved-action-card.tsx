"use client";

import { ArrowRight } from "lucide-react";

import { presentActionCtaKind } from "@/components/listingkit/shared/action-presentation";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type { ResolvedActionSummary } from "@/lib/types/listingkit";

export function ResolvedActionCard({
  summary,
  onSelect,
}: {
  summary?: ResolvedActionSummary | null;
  onSelect?: (summary: ResolvedActionSummary) => void;
}) {
  if (!summary) {
    return null;
  }

  return (
    <Card className="p-5">
      <div className="flex min-w-0 flex-col items-stretch gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
            Primary Action
          </p>
          <h2 className="break-words text-xl font-semibold text-zinc-950">
            {summary.title}
          </h2>
          {summary.summary ? (
            <p className="max-w-2xl break-words text-sm leading-6 text-zinc-600">
              {summary.summary}
            </p>
          ) : null}
        </div>
        <Button
          className="w-full shrink-0 gap-2 sm:w-auto"
          onClick={() => (summary ? onSelect?.(summary) : undefined)}
        >
          {presentActionCtaKind(summary.cta_kind)}
          <ArrowRight className="h-4 w-4" />
        </Button>
      </div>
    </Card>
  );
}
