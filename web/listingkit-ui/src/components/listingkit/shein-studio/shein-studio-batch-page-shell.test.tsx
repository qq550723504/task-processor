import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioBatchPageShell } from "@/components/listingkit/shein-studio/shein-studio-batch-page-shell";

vi.mock("@/components/listingkit/shein-studio/shein-studio-workbench", () => ({
  SheinStudioWorkbench: ({ initialBatchId }: { initialBatchId?: string }) => (
    <div>workbench {initialBatchId}</div>
  ),
}));

describe("SheinStudioBatchPageShell", () => {
  it("wraps the dedicated workbench in a responsive full-width page container", () => {
    const { container } = render(
      <SheinStudioBatchPageShell batchId="batch-123" />,
    );

    expect(screen.getByText("workbench batch-123")).toBeInTheDocument();

    const pageContainer = container.querySelector(
      ".max-w-none",
    ) as HTMLDivElement | null;
    expect(pageContainer).not.toBeNull();
    expect(pageContainer?.className).toContain("px-4");
    expect(pageContainer?.className).not.toContain("max-w-7xl");
  });
});
