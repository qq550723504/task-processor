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
            输入更完整，任务首次成功率通常会更高。
          </p>
        </div>
      </div>
      <div className="text-sm font-medium text-zinc-700">
        {item.current} / {item.recommended} {ready ? "已满足" : "建议值"}
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
            输入建议
          </h2>
          <p className="text-sm leading-6 text-zinc-600">
            这些是当前流程里最常见的通过门槛。若你有商品链接，也可以用它补强来源信息。
          </p>
        </div>

        <div className="space-y-3">
          <GuidanceRow
            item={{
              label: "至少提供 3 张图片",
              current: imageCount,
              recommended: 3,
            }}
          />
          <GuidanceRow
            item={{
              label: "标题或文案建议达到 50 个字符",
              current: textLength,
              recommended: 50,
            }}
          />
          <GuidanceRow
            item={{
              label: "有商品链接会更稳",
              current: hasProductUrl ? 1 : 0,
              recommended: 1,
            }}
          />
        </div>
      </div>
    </Card>
  );
}
