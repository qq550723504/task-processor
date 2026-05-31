import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SheinStudioProgressStrip } from "@/components/listingkit/shein-studio/shein-studio-progress-strip";

describe("SheinStudioProgressStrip", () => {
  it("uses a progressive grid instead of a tablet-only three-column layout", () => {
    const { container } = render(
      <SheinStudioProgressStrip
        createdTaskCount={1}
        generatedStyleCount={3}
        selectedStyleCount={2}
      />,
    );

    expect(screen.getByText("3 个款式")).toBeInTheDocument();
    const grid = container.querySelector(".grid.gap-3") as HTMLDivElement | null;
    expect(grid).not.toBeNull();
    expect(grid?.className).not.toContain("md:grid-cols-3");
  });
});
