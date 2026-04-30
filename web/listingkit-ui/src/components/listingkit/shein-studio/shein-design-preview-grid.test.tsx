import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";

vi.mock("next/image", () => ({
  default: (props: React.ImgHTMLAttributes<HTMLImageElement>) => <img {...props} />,
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-lightbox", () => ({
  SheinDesignLightbox: () => null,
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-review-note", () => ({
  SheinDesignReviewNote: () => <div>review note</div>,
}));

describe("SheinDesignPreviewGrid", () => {
  it("shows cancel approval and continue generating actions for selected designs", () => {
    const onToggle = vi.fn();
    const onRegenerate = vi.fn();
    const onBackToGenerate = vi.fn();

    render(
      <SheinDesignPreviewGrid
        createActionDisabledReason={undefined}
        designs={[{ id: "design-1", imageUrl: "https://example.com/design-1.png" }]}
        onBackToGenerate={onBackToGenerate}
        onCreateReviewTasks={vi.fn()}
        onRegenerate={onRegenerate}
        onToggle={onToggle}
        selectedIds={["design-1"]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "取消批准" }));
    expect(onToggle).toHaveBeenCalledWith("design-1");

    fireEvent.click(screen.getByRole("button", { name: "继续生成款式图" }));
    expect(onBackToGenerate).toHaveBeenCalledTimes(1);
  });
});
