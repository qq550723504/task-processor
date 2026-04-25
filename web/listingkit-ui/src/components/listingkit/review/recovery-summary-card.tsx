"use client";

import { AlertTriangle } from "lucide-react";

import { presentRecoverySummary } from "@/components/listingkit/shared/action-presentation";
import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type { RecoveryDescriptor, RecoverySummary } from "@/lib/types/listingkit";

export function RecoverySummaryCard({
  summary,
  onSelect,
}: {
  summary?: RecoverySummary | null;
  onSelect?: (descriptor: RecoveryDescriptor) => void;
}) {
  if (!summary) {
    return null;
  }

  const presentation = presentRecoverySummary(summary);
  if (!presentation) {
    return null;
  }

  return (
    <Card className="border-amber-200 bg-amber-50/70 p-5">
      <div className="flex min-w-0 items-start gap-3">
        <div className="rounded-xl bg-amber-100 p-2 text-amber-700">
          <AlertTriangle className="h-4 w-4" />
        </div>
        <div className="min-w-0 space-y-1">
          <h2 className="break-words text-base font-semibold text-zinc-950">
            {presentation.title}
          </h2>
          {presentation.summary ? (
            <p className="break-words text-sm leading-6 text-zinc-700">
              {presentation.summary}
            </p>
          ) : null}
          <div className="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.2em] text-zinc-500">
            <span>{presentation.metaLabel}</span>
            <span className="rounded-full border border-amber-200 bg-amber-100 px-2 py-1 text-[10px] font-semibold tracking-[0.16em] text-amber-800">
              {presentation.ctaLabel}
            </span>
          </div>
          {summary.primary_descriptor ? (
            <div className="pt-2">
              <Button
                tone="secondary"
                onClick={() => onSelect?.(summary.primary_descriptor!)}
              >
                {presentation.ctaLabel}
              </Button>
            </div>
          ) : null}
        </div>
      </div>
    </Card>
  );
}
