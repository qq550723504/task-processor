"use client";

import { Button } from "@/components/ui/button";
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
        <Button
          key={section.section_key}
          type="button"
          variant={selectedKey === section.section_key ? "default" : "secondary"}
          className={cn(
            "h-auto rounded-full px-3 py-2",
            selectedKey !== section.section_key && "text-zinc-700",
          )}
          onClick={() => onSelect(section)}
        >
          {section.title ?? section.capability_label ?? section.capability}
        </Button>
      ))}
    </div>
  );
}
