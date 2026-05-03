"use client";

import { Card } from "@/components/shared/card";
import { cn } from "@/lib/utils/cn";

export type TaskSourceTab = "imageUrls" | "productUrl";

function sourceCopy(activeTab: TaskSourceTab) {
  if (activeTab === "productUrl") {
    return "适合已有商品来源时使用。粘贴 1688 或其他商品页链接，系统会按原始商品资料继续处理。";
  }

  return "适合只有图片素材时使用。支持直接粘贴公网图片链接，或先上传本地图片再继续。";
}

export function TaskSourceTabs({
  activeTab,
  onTabChange,
}: {
  activeTab: TaskSourceTab;
  onTabChange: (tab: TaskSourceTab) => void;
}) {
  return (
    <Card className="border-zinc-200 bg-zinc-50/80 p-5">
      <div className="space-y-4">
        <div className="space-y-1">
          <h2 className="text-sm font-semibold uppercase tracking-[0.2em] text-zinc-500">
            任务来源
          </h2>
          <p className="text-sm leading-6 text-zinc-600">{sourceCopy(activeTab)}</p>
        </div>

        <div
          aria-label="任务来源"
          className="inline-flex rounded-2xl border border-zinc-200 bg-white p-1"
          role="tablist"
        >
          {[
            { key: "productUrl", label: "商品链接" },
            { key: "imageUrls", label: "图片素材" },
          ].map((tab) => {
            const selected = activeTab === tab.key;
            return (
              <button
                aria-selected={selected}
                className={cn(
                  "rounded-xl px-4 py-2 text-sm font-medium transition",
                  selected
                    ? "bg-zinc-950 text-white"
                    : "text-zinc-700 hover:bg-zinc-100",
                )}
                key={tab.key}
                onClick={() => onTabChange(tab.key as TaskSourceTab)}
                role="tab"
                type="button"
              >
                {tab.label}
              </button>
            );
          })}
        </div>
      </div>
    </Card>
  );
}
