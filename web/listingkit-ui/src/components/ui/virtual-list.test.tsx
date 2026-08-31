import { render, screen, waitFor } from "@testing-library/react";
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

  it("positions rows using their measured heights", async () => {
    vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockImplementation(
      function (this: HTMLElement) {
        if (this.getAttribute("role") === "listitem") {
          return this.textContent === "长任务" ? 64 : 32;
        }
        return this.getAttribute("role") === "list" ? 160 : 0;
      },
    );
    vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(320);

    render(
      <VirtualList
        ariaLabel="动态高度任务列表"
        estimateSize={32}
        height={160}
        items={["长任务", "普通任务", "收尾任务"]}
        overscan={0}
      >
        {(item) => <span>{item}</span>}
      </VirtualList>,
    );

    await waitFor(() => {
      const secondItem = screen.getByText("普通任务").closest('[role="listitem"]');
      expect(secondItem).toHaveStyle({ transform: "translateY(64px)" });
    });
  });
});
