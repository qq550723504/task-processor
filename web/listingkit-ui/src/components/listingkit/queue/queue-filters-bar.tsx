"use client";

import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";

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
        <Label className="space-y-2 text-sm text-zinc-700">
          <span>Platform</span>
          <Select {...form.register("platform")} aria-label="Platform">
            <option value="">All</option>
            <option value="amazon">Amazon</option>
            <option value="shein">Shein</option>
            <option value="temu">Temu</option>
            <option value="walmart">Walmart</option>
          </Select>
        </Label>
        <Label className="space-y-2 text-sm text-zinc-700">
          <span>Slot</span>
          <Input placeholder="main / gallery" {...form.register("slot")} />
        </Label>
        <Label className="space-y-2 text-sm text-zinc-700">
          <span>Quality Grade</span>
          <Select {...form.register("quality_grade")} aria-label="Quality Grade">
            <option value="">All</option>
            <option value="ideal">Ideal</option>
            <option value="provisional">Provisional</option>
            <option value="missing">Missing</option>
          </Select>
        </Label>
        <Label className="space-y-2 text-sm text-zinc-700">
          <span>Preview Capability</span>
          <Select {...form.register("preview_capability")} aria-label="Preview Capability">
            <option value="">All</option>
            <option value="detail_preview">Detail</option>
            <option value="measurement_preview">Measurement</option>
            <option value="badge_preview">Badge</option>
            <option value="copy_preview">Copy</option>
            <option value="subject_preview">Subject</option>
          </Select>
        </Label>
        <Label className="space-y-2 text-sm text-zinc-700">
          <span>Review Status</span>
          <Select {...form.register("review_status")} aria-label="Review Status">
            <option value="">All</option>
            <option value="approved">Approved</option>
            <option value="deferred">Deferred</option>
            <option value="pending">Pending</option>
          </Select>
        </Label>
        <div className="flex items-end gap-3">
          <Label className="flex h-10 items-center gap-2 rounded-md border border-input px-3 text-sm text-foreground">
            <Checkbox aria-label="Has Preview" {...form.register("render_preview_available")} />
            <span>Has Preview</span>
          </Label>
          <Button className="flex-1" type="submit">
            Apply Filters
          </Button>
        </div>
      </form>
    </Card>
  );
}
