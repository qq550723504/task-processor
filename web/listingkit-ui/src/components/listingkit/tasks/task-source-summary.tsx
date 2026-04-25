"use client";

import type { TaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import { Card } from "@/components/shared/card";

function summarizeSource(draft?: Partial<TaskCreateDraft> | null) {
  const hasProductUrl = Boolean(draft?.productUrl?.trim());
  const imageCount =
    draft?.imageUrls
      ?.split(/\r?\n/)
      .map((value) => value.trim())
      .filter(Boolean).length ?? 0;

  if (hasProductUrl && imageCount > 0) {
    return {
      title: "Created from product URL and image URLs",
      summary: `${imageCount} image URL${imageCount === 1 ? "" : "s"} plus a product listing URL were submitted for this task.`,
    };
  }

  if (hasProductUrl) {
    return {
      title: "Created from product URL",
      summary:
        "This task started from a product listing URL. A 1688 link is supported in this flow.",
    };
  }

  if (imageCount > 0) {
    return {
      title: "Created from image URLs",
      summary: `${imageCount} image URL${imageCount === 1 ? "" : "s"} were submitted for this task.`,
    };
  }

  return null;
}

export function TaskSourceSummary({
  draft,
}: {
  draft?: Partial<TaskCreateDraft> | null;
}) {
  const source = summarizeSource(draft);
  if (!source) {
    return null;
  }

  return (
    <Card className="p-6">
      <div className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
          Task source
        </p>
        <h2 className="text-lg font-semibold text-zinc-950">{source.title}</h2>
        <p className="text-sm leading-6 text-zinc-600">{source.summary}</p>
      </div>
    </Card>
  );
}

