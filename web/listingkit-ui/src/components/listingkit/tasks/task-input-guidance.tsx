"use client";

import { CheckCircle2, Circle } from "lucide-react";

type GuidanceItem = {
  label: string;
  current: number;
  recommended: number;
};

function GuidanceRow({ item }: { item: GuidanceItem }) {
  const ready = item.current >= item.recommended;

  return (
    <div className="flex items-start justify-between gap-4 rounded-xl border border-border bg-background px-4 py-3">
      <div className="flex items-start gap-3">
        {ready ? (
          <CheckCircle2 className="mt-0.5 h-4 w-4 text-emerald-600" />
        ) : (
          <Circle className="mt-0.5 h-4 w-4 text-muted-foreground" />
        )}
        <div className="space-y-1">
          <div className="text-sm font-medium text-foreground">{item.label}</div>
          <p className="text-xs leading-5 text-muted-foreground">
            输入更完整，任务首次成功率通常会更高。
          </p>
        </div>
      </div>
      <div className="text-sm font-medium text-muted-foreground">
        {item.current} / {item.recommended} {ready ? "已满足" : "建议值"}
      </div>
    </div>
  );
}

export function TaskInputGuidance({
  embedded = false,
  imageCount,
  textLength,
  hasProductUrl = false,
}: {
  embedded?: boolean;
  imageCount: number;
  textLength: number;
  hasProductUrl?: boolean;
}) {
  return (
    <section
      className={
        embedded
          ? "space-y-4"
          : "space-y-4 rounded-2xl border border-border bg-muted/70 p-4"
      }
    >
      <div className="space-y-1">
        <h2 className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
          输入建议
        </h2>
        <p className="text-sm leading-6 text-muted-foreground">
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
    </section>
  );
}
