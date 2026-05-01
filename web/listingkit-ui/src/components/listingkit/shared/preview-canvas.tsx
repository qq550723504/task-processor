"use client";

import { Card } from "@/components/shared/card";
import { EmptyState } from "@/components/shared/empty-state";
import type { PreviewSlot, ReviewPreviewResponse } from "@/lib/types/listingkit";

export function PreviewCanvas({
  preview,
  response,
  emptyState,
}: {
  preview?: PreviewSlot | null;
  response?: ReviewPreviewResponse | null;
  emptyState?: {
    title: string;
    description: string;
  } | null;
}) {
  if (preview?.asset_url) {
    return (
      <Card className="min-h-[32rem] overflow-hidden bg-[radial-gradient(circle_at_top,_rgba(199,210,254,0.45),_transparent_32%),linear-gradient(180deg,_#fafaf9,_#f4f4f5)] p-6">
        <div className="rounded-2xl border border-zinc-200 bg-white p-4 shadow-lg shadow-zinc-950/5">
          {/* Dynamic remote asset URLs are not covered by the app image config. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            alt={preview.template_label ?? preview.asset_id ?? "Listing preview"}
            className="mx-auto max-h-[32rem] w-auto rounded-xl object-contain"
            src={preview.asset_url}
          />
        </div>
      </Card>
    );
  }

  if (!preview?.preview_svg) {
    return (
      <EmptyState
        title={emptyState?.title ?? "No preview available"}
        description={
          emptyState?.description ??
          response?.revision_mismatch_reason ??
          "The selected slot does not currently expose an SVG sidecar preview."
        }
      />
    );
  }

  return (
    <Card className="min-h-[32rem] overflow-hidden bg-[radial-gradient(circle_at_top,_rgba(199,210,254,0.45),_transparent_32%),linear-gradient(180deg,_#fafaf9,_#f4f4f5)] p-6">
      <div className="rounded-2xl border border-zinc-200 bg-white p-4 shadow-lg shadow-zinc-950/5">
        <div
          className="min-h-[26rem] [&_svg]:h-auto [&_svg]:w-full"
          dangerouslySetInnerHTML={{ __html: preview.preview_svg }}
        />
      </div>
    </Card>
  );
}
