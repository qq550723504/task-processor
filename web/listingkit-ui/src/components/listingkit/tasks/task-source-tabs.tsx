"use client";

import { ImageIcon, Link2 } from "lucide-react";

import { Button } from "@/components/ui/button";
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
  embedded = false,
  onTabChange,
}: {
  activeTab: TaskSourceTab;
  embedded?: boolean;
  onTabChange: (tab: TaskSourceTab) => void;
}) {
  return (
    <section
      className={
        embedded
          ? "space-y-4"
          : "space-y-3 rounded-2xl border border-zinc-200 bg-zinc-50/70 p-4"
      }
    >
      <div className="space-y-1">
        <h2 className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
          任务来源
        </h2>
        <p className="text-sm leading-6 text-zinc-600">{sourceCopy(activeTab)}</p>
      </div>

      <div
        aria-label="任务来源"
        className="grid gap-2 sm:grid-cols-2"
        role="tablist"
      >
        {[
          { key: "productUrl", label: "商品链接", description: "已有原始商品页", icon: Link2 },
          { key: "imageUrls", label: "图片素材", description: "只有图片也能开始", icon: ImageIcon },
        ].map((tab) => {
          const selected = activeTab === tab.key;
          const Icon = tab.icon;
          const labelId = `task-source-tab-${tab.key}-label`;
          return (
            <Button
              aria-labelledby={labelId}
              aria-selected={selected}
              variant={selected ? "default" : "outline"}
              className={cn(
                "h-auto min-h-[76px] justify-start gap-3 rounded-xl px-4 py-3 text-left",
                selected ? "text-white" : "text-zinc-800",
              )}
              key={tab.key}
              onClick={() => onTabChange(tab.key as TaskSourceTab)}
              role="tab"
              type="button"
              >
              <span
                className={cn(
                  "mt-0.5 inline-flex h-8 w-8 items-center justify-center rounded-lg",
                  selected ? "bg-white/12 text-white" : "bg-zinc-100 text-zinc-700",
                )}
                aria-hidden="true"
              >
                <Icon className="h-4 w-4" />
              </span>
              <span className="min-w-0">
                <span id={labelId} className="sr-only">{tab.label}</span>
                <span aria-hidden="true" className="block text-sm font-semibold">{tab.label}</span>
                <span
                  aria-hidden="true"
                  className={cn(
                    "mt-1 block text-xs leading-5",
                    selected ? "text-zinc-300" : "text-zinc-500",
                  )}
                >
                  {tab.description}
                </span>
              </span>
            </Button>
          );
        })}
      </div>
    </section>
  );
}
