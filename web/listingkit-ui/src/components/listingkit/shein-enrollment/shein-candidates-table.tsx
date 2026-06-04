"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { SheinActivityCandidateRecord } from "@/lib/types/listingkit/shein-enrollment";

export function SheinCandidatesTable({
  enrolling,
  items,
  onApprove,
  onEnroll,
  onReject,
}: {
  enrolling: boolean;
  items: SheinActivityCandidateRecord[];
  onApprove: (candidateId: number) => Promise<void>;
  onEnroll: (candidateIds: number[], activityKey: string) => Promise<void>;
  onReject: (candidateId: number) => Promise<void>;
}) {
  const [activityKey, setActivityKey] = useState("");
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const selected = useMemo(() => new Set(selectedIds), [selectedIds]);

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white p-4 lg:flex-row lg:items-center">
        <Input
          aria-label="活动 key"
          className="lg:w-64"
          onChange={(event) => setActivityKey(event.target.value)}
          placeholder="可选：活动 key，不填则由后端生成"
          value={activityKey}
        />
        <Button
          disabled={enrolling || selectedIds.length === 0}
          onClick={() => void onEnroll(selectedIds, activityKey.trim())}
          type="button"
        >
          {enrolling ? "报名中..." : "报名活动"}
        </Button>
      </div>

      {items.length === 0 ? (
        <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
          当前活动类型下暂无候选商品。
        </div>
      ) : null}

      <div className="grid gap-3">
        {items.map((item) => (
          <div key={item.id} className="rounded-2xl border border-zinc-200 bg-white p-4">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-start">
              <label className="flex items-start gap-3">
                <input
                  aria-label={`选择 ${item.skc_name || item.id}`}
                  checked={selected.has(item.id ?? 0)}
                  onChange={(event) => {
                    const currentId = item.id ?? 0;
                    setSelectedIds((current) =>
                      event.target.checked
                        ? [...current, currentId]
                        : current.filter((candidateId) => candidateId !== currentId),
                    );
                  }}
                  type="checkbox"
                />
                <div>
                  <p className="font-medium text-zinc-950">{item.skc_name || "-"}</p>
                  <p className="mt-1 text-sm text-zinc-500">
                    {item.eligibility_reason || "未提供候选原因"}
                  </p>
                  <p className="mt-1 text-xs text-zinc-500">
                    状态 {item.review_status || "-"} · 成本 {item.effective_cost_price ?? "-"} ·
                    利润率 {item.calculated_profit_rate ?? "-"}
                  </p>
                </div>
              </label>
              <div className="flex flex-wrap gap-2 lg:ml-auto">
                <Button
                  disabled={!item.id}
                  onClick={() => void onApprove(item.id ?? 0)}
                  size="sm"
                  type="button"
                  variant="outline"
                >
                  通过
                </Button>
                <Button
                  disabled={!item.id}
                  onClick={() => void onReject(item.id ?? 0)}
                  size="sm"
                  type="button"
                  variant="outline"
                >
                  驳回
                </Button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
