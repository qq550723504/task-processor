"use client";

import { useRouter } from "next/navigation";

import { Button } from "@/components/shared/button";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";

export function SheinSavedBatchesPanel({
  batches,
  onDelete,
  onLoad,
}: {
  batches: SheinStudioSavedBatch[];
  onDelete: (batchID: string) => void;
  onLoad: (batch: SheinStudioSavedBatch) => void;
}) {
  const router = useRouter();

  if (batches.length === 0) {
    return null;
  }

  return (
    <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
      <div className="flex items-center justify-between gap-3">
        <div className="text-sm font-semibold text-zinc-900">Saved batches</div>
        <div className="text-xs text-zinc-500">{batches.length} saved</div>
      </div>
      <div className="mt-3 grid gap-3">
        {batches.map((batch) => (
          <div
            className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3"
            key={batch.id}
          >
            <div className="space-y-1">
              <div className="text-sm font-semibold text-zinc-950">{batch.name}</div>
              <div className="text-xs text-zinc-500">
                {batch.selection?.productName || "No product"} · {batch.designs.length}{" "}
                styles · {new Date(batch.updatedAt).toLocaleString()}
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                onClick={() => router.push(`/listing-kits/shein/batches/${batch.id}`)}
                tone="ghost"
              >
                Open batch
              </Button>
              <Button onClick={() => onLoad(batch)} tone="secondary">
                Load
              </Button>
              <Button onClick={() => onDelete(batch.id)} tone="ghost">
                Delete
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
