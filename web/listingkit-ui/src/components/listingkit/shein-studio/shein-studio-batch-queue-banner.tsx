"use client";

import { Button } from "@/components/ui/button";
import type { SheinStudioBatchQueueMode } from "@/lib/types/shein-studio";

export function SheinStudioBatchQueueBanner({
  currentBatchName,
  currentIndex,
  guidance,
  mode,
  total,
  onExit,
  onNext,
  onSkip,
}: {
  currentBatchName: string;
  currentIndex: number;
  guidance: string;
  mode: SheinStudioBatchQueueMode;
  total: number;
  onExit: () => void;
  onNext: () => void;
  onSkip: () => void;
}) {
  return (
    <section className="rounded-[1.5rem] border border-emerald-200 bg-emerald-50/80 px-4 py-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700">
            批量处理
          </div>
          <div className="text-base font-semibold text-emerald-950">
            {mode === "generate" ? "批量继续生成" : "批量创建任务"}
          </div>
          <div className="text-sm text-emerald-900">
            {`第 ${currentIndex + 1} / ${total} 个批次`}
          </div>
          <div className="text-sm text-emerald-800">{currentBatchName}</div>
          <div className="text-sm text-emerald-700">{guidance}</div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button onClick={onNext} size="sm" type="button" variant="secondary">
            下一批次
          </Button>
          <Button onClick={onSkip} size="sm" type="button" variant="ghost">
            跳过
          </Button>
          <Button onClick={onExit} size="sm" type="button" variant="ghost">
            退出批量处理
          </Button>
        </div>
      </div>
    </section>
  );
}
