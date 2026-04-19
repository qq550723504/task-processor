"use client";

import { cn } from "@/lib/utils/cn";
import type { ReviewSection } from "@/lib/types/listingkit";

export function ReviewSectionTabs({
  sections,
  selectedKey,
  onSelect,
}: {
  sections?: ReviewSection[];
  selectedKey?: string;
  onSelect: (section: ReviewSection) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {(sections ?? []).map((section) => (
        <button
          key={section.section_key}
          type="button"
          className={cn(
            "rounded-full px-3 py-2 text-sm font-medium transition",
            selectedKey === section.section_key
              ? "bg-zinc-950 text-white"
              : "bg-zinc-100 text-zinc-700 hover:bg-zinc-200",
          )}
          onClick={() => onSelect(section)}
        >
          {section.title ?? section.capability_label ?? section.capability}
        </button>
      ))}
    </div>
  );
}
