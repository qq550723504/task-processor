"use client";

import { useMemo, useState } from "react";

import { formatSheinPriceSnapshot } from "@/components/listingkit/shein-enrollment/shein-price-snapshot";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { SheinActivityCandidateRecord } from "@/lib/types/listingkit/shein-enrollment";

export function SheinCandidatesTable({
  enrollmentDisabled,
  enrollmentDisabledReason,
  enrolling,
  items,
  onEnroll,
  onReject,
}: {
  enrollmentDisabled?: boolean;
  enrollmentDisabledReason?: string;
  enrolling: boolean;
  items: SheinActivityCandidateRecord[];
  onEnroll: (candidateIds: number[], activityKey: string) => Promise<void>;
  onReject: (candidateId: number) => Promise<void>;
}) {
  const [activityKey, setActivityKey] = useState("");
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const selected = useMemo(() => new Set(selectedIds), [selectedIds]);
  const executableIds = useMemo(
    () =>
      items
        .filter((item) => item.id && isExecutableEnrollmentCandidate(item))
        .map((item) => item.id ?? 0),
    [items],
  );
  const selectedExecutableIds = useMemo(
    () => selectedIds.filter((id) => executableIds.includes(id)),
    [executableIds, selectedIds],
  );
  const allExecutableSelected =
    executableIds.length > 0 && executableIds.every((id) => selected.has(id));

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
          disabled={enrolling || executableIds.length === 0}
          onClick={() => {
            setSelectedIds(allExecutableSelected ? [] : executableIds);
          }}
          type="button"
          variant="outline"
        >
          {allExecutableSelected ? "取消全选" : "全选"}
        </Button>
        <Button
          disabled={enrolling || selectedExecutableIds.length === 0 || enrollmentDisabled}
          onClick={() => void onEnroll(selectedExecutableIds, activityKey.trim())}
          type="button"
        >
          {enrolling ? "报名中..." : "报名活动"}
        </Button>
        <span className="text-xs text-zinc-500">
          已选 {selectedExecutableIds.length} / {executableIds.length} 个可报名
        </span>
        {enrollmentDisabled && enrollmentDisabledReason ? (
          <span className="text-xs text-amber-700">{enrollmentDisabledReason}</span>
        ) : null}
      </div>

      {items.length === 0 ? (
        <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
          当前活动类型下暂无候选商品。
        </div>
      ) : null}

      <div className="grid gap-3">
        {items.map((item) => {
          const executable = isExecutableEnrollmentCandidate(item);
          return (
            <div key={item.id} className="rounded-2xl border border-zinc-200 bg-white p-4">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start">
                <label className="flex items-start gap-3">
                  <input
                    aria-label={`选择 ${item.skc_name || item.id}`}
                    checked={executable && selected.has(item.id ?? 0)}
                    disabled={!executable || !item.id}
                    onChange={(event) => {
                      const currentId = item.id ?? 0;
                      if (!currentId) {
                        return;
                      }
                      setSelectedIds((current) =>
                        event.target.checked
                          ? Array.from(new Set([...current, currentId]))
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
                      售价 {formatSheinPriceSnapshot(item.price_snapshot)} · 利润率{" "}
                      {item.calculated_profit_rate ?? "-"}
                    </p>
                    {!executable ? (
                      <p className="mt-1 text-xs text-amber-700">当前状态不可报名</p>
                    ) : null}
                    {item.review_status === "failed" && item.last_enrollment_error ? (
                      <p className="mt-1 text-xs text-red-600">
                        报名失败：{item.last_enrollment_error}
                      </p>
                    ) : null}
                  </div>
                </label>
                <div className="flex flex-wrap gap-2 lg:ml-auto">
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
          );
        })}
      </div>
    </section>
  );
}

function isExecutableEnrollmentCandidate(item: SheinActivityCandidateRecord) {
  return (
    item.review_status === "approved" ||
    item.review_status === "auto_queued" ||
    item.review_status === "pending_review"
  );
}
