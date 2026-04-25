"use client";

import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";

const schema = z.object({
  platform: z.string(),
  slot: z.string(),
  quality_grade: z.string(),
  preview_capability: z.string(),
  review_status: z.string(),
  render_preview_available: z.boolean(),
});

export type QueueFilterValue = z.infer<typeof schema>;

export function QueueFiltersBar({
  value,
  onApply,
}: {
  value: QueueFilterValue;
  onApply: (value: QueueFilterValue) => void;
}) {
  const form = useForm<QueueFilterValue>({
    resolver: zodResolver(schema),
    defaultValues: value,
    values: value,
  });

  return (
    <Card className="p-4">
      <form
        className="grid gap-4 lg:grid-cols-6"
        onSubmit={form.handleSubmit((nextValue) => onApply(nextValue))}
      >
        <label className="space-y-2 text-sm text-zinc-700">
          <span>Platform</span>
          <select className="w-full rounded-xl border border-zinc-200 px-3 py-2" {...form.register("platform")} aria-label="Platform">
            <option value="">All</option>
            <option value="amazon">Amazon</option>
            <option value="shein">Shein</option>
            <option value="temu">Temu</option>
            <option value="walmart">Walmart</option>
          </select>
        </label>
        <label className="space-y-2 text-sm text-zinc-700">
          <span>Slot</span>
          <input className="w-full rounded-xl border border-zinc-200 px-3 py-2" placeholder="main / gallery" {...form.register("slot")} />
        </label>
        <label className="space-y-2 text-sm text-zinc-700">
          <span>Quality Grade</span>
          <select className="w-full rounded-xl border border-zinc-200 px-3 py-2" {...form.register("quality_grade")} aria-label="Quality Grade">
            <option value="">All</option>
            <option value="ideal">Ideal</option>
            <option value="provisional">Provisional</option>
            <option value="missing">Missing</option>
          </select>
        </label>
        <label className="space-y-2 text-sm text-zinc-700">
          <span>Preview Capability</span>
          <select className="w-full rounded-xl border border-zinc-200 px-3 py-2" {...form.register("preview_capability")} aria-label="Preview Capability">
            <option value="">All</option>
            <option value="detail_preview">Detail</option>
            <option value="measurement_preview">Measurement</option>
            <option value="badge_preview">Badge</option>
            <option value="copy_preview">Copy</option>
            <option value="subject_preview">Subject</option>
          </select>
        </label>
        <label className="space-y-2 text-sm text-zinc-700">
          <span>Review Status</span>
          <select className="w-full rounded-xl border border-zinc-200 px-3 py-2" {...form.register("review_status")} aria-label="Review Status">
            <option value="">All</option>
            <option value="approved">Approved</option>
            <option value="deferred">Deferred</option>
            <option value="pending">Pending</option>
          </select>
        </label>
        <div className="flex items-end gap-3">
          <label className="flex h-10 items-center gap-2 rounded-xl border border-zinc-200 px-3 text-sm text-zinc-700">
            <input type="checkbox" aria-label="Has Preview" {...form.register("render_preview_available")} />
            <span>Has Preview</span>
          </label>
          <Button className="flex-1" type="submit">
            Apply Filters
          </Button>
        </div>
      </form>
    </Card>
  );
}
