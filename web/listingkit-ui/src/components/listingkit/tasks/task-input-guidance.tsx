"use client";

import { CheckCircle2, Circle } from "lucide-react";

import { Card } from "@/components/shared/card";

type GuidanceItem = {
  label: string;
  current: number;
  recommended: number;
};

function GuidanceRow({ item }: { item: GuidanceItem }) {
  const ready = item.current >= item.recommended;

  return (
    <div className="flex items-start justify-between gap-4 rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
      <div className="flex items-start gap-3">
        {ready ? (
          <CheckCircle2 className="mt-0.5 h-4 w-4 text-emerald-600" />
        ) : (
          <Circle className="mt-0.5 h-4 w-4 text-zinc-400" />
        )}
        <div className="space-y-1">
          <div className="text-sm font-medium text-zinc-900">{item.label}</div>
          <p className="text-xs leading-5 text-zinc-600">
            Better inputs reduce immediate product-enrichment failures.
          </p>
        </div>
      </div>
      <div className="text-sm font-medium text-zinc-700">
        {item.current} / {item.recommended} {ready ? "ready" : "target"}
      </div>
    </div>
  );
}

export function TaskInputGuidance({
  imageCount,
  textLength,
  hasProductUrl = false,
}: {
  imageCount: number;
  textLength: number;
  hasProductUrl?: boolean;
}) {
  return (
    <Card className="border-zinc-200 bg-zinc-50/80 p-5">
      <div className="space-y-4">
        <div className="space-y-1">
          <h2 className="text-sm font-semibold uppercase tracking-[0.2em] text-zinc-500">
            Input quality guidance
          </h2>
          <p className="text-sm leading-6 text-zinc-600">
            These are the practical thresholds observed from the current backend
            quality gate. A 1688 product URL can also provide a stronger starting
            source.
          </p>
        </div>

        <div className="space-y-3">
          <GuidanceRow
            item={{
              label: "Need at least 3 image URLs",
              current: imageCount,
              recommended: 3,
            }}
          />
          <GuidanceRow
            item={{
              label: "Need at least 50 characters of product text",
              current: textLength,
              recommended: 50,
            }}
          />
          <GuidanceRow
            item={{
              label: "Optional product URL for source-backed generation",
              current: hasProductUrl ? 1 : 0,
              recommended: 1,
            }}
          />
        </div>
      </div>
    </Card>
  );
}
