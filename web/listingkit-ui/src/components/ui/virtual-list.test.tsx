import { render, screen } from "@testing-library/react";
import { afterEach, vi } from "vitest";

import { VirtualList } from "@/components/ui/virtual-list";

afterEach(() => vi.restoreAllMocks());

describe("VirtualList", () => {
  it("renders a bounded window instead of every item", async () => {
    vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(160);
    vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(320);
    const items = Array.from({ length: 100 }, (_, index) => `任务 ${index + 1}`);
    render(
      <VirtualList
        ariaLabel="任务列表"
        estimateSize={32}
        height={160}
        items={items}
      >
        {(item) => <span>{item}</span>}
      </VirtualList>,
    );

    const renderedItems = await screen.findAllByRole("listitem");
    expect(renderedItems.length).toBeGreaterThan(0);
    expect(renderedItems.length).toBeLessThan(100);
    expect(screen.getByText("任务 1")).toBeInTheDocument();
  });
});
