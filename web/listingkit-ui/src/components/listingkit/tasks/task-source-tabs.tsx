"use client";

import { Card } from "@/components/shared/card";
import { cn } from "@/lib/utils/cn";

export type TaskSourceTab = "imageUrls" | "productUrl";

function sourceCopy(activeTab: TaskSourceTab) {
  if (activeTab === "productUrl") {
    return "Paste a 1688 or other product URL when you want ListingKit to start from the original listing.";
  }

  return "Paste public image URLs when you want to drive generation from product images.";
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
            Source mode
          </h2>
          <p className="text-sm leading-6 text-zinc-600">{sourceCopy(activeTab)}</p>
        </div>

        <div
          aria-label="Source mode"
          className="inline-flex rounded-2xl border border-zinc-200 bg-white p-1"
          role="tablist"
        >
          {[
            { key: "productUrl", label: "1688 / Product URL" },
            { key: "imageUrls", label: "Image URLs" },
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
