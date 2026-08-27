import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ProductWorkspaceShell } from "@/components/listingkit/workspace/product-workspace-shell";

describe("ProductWorkspaceShell", () => {
  it("provides accessible product navigation, work, AI review, and action regions", () => {
    render(
      <ProductWorkspaceShell
        navigation={<div>navigation content</div>}
        work={<div>work content</div>}
        aiReview={<div>review content</div>}
        actions={<div>action content</div>}
      />,
    );

    expect(screen.getByRole("navigation", { name: "商品工作台导航" })).toHaveTextContent(
      "navigation content",
    );
    expect(screen.getByRole("main", { name: "商品工作区" })).toHaveTextContent("work content");
    expect(screen.getByRole("complementary", { name: "AI 审核" })).toHaveTextContent(
      "review content",
    );
    expect(screen.getByRole("region", { name: "商品操作" })).toHaveTextContent(
      "action content",
    );
  });

  it("uses a desktop three-column grid without forcing narrow layouts to overflow", () => {
    const { container } = render(
      <ProductWorkspaceShell
        navigation={<div>nav</div>}
        work={<div>work</div>}
        aiReview={<div>review</div>}
      />,
    );

    const grid = container.querySelector("[data-product-workspace-grid]");
    expect(grid).toHaveClass("min-w-0");
    expect(grid).toHaveClass("xl:grid-cols-[220px_minmax(0,1fr)_320px]");
  });
});
