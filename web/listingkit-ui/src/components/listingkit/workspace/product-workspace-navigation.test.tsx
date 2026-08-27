import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProductWorkspaceNavigation } from "@/components/listingkit/workspace/product-workspace-navigation";
import {
  buildProductWorkspaceCanonicalNavigation,
  buildProductWorkspacePlatformNavigation,
} from "@/components/listingkit/workspace/product-workspace-model";

describe("ProductWorkspaceNavigation", () => {
  it("renders product structure, platform context, and history without execution identity", () => {
    render(
      <ProductWorkspaceNavigation
        canonicalItems={buildProductWorkspaceCanonicalNavigation("overview")}
        platformItems={buildProductWorkspacePlatformNavigation(
          [{ platform: "shein", label: "SHEIN", status: "attention" }],
          "shein",
        )}
        onSelect={vi.fn()}
        onSelectHistory={vi.fn()}
      />,
    );

    expect(screen.getByText("商品资料")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "概览" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "图片" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "基础信息" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "SKU" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "规格" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "属性" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "描述" })).toBeInTheDocument();

    expect(screen.getByText("平台资料")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /SHEIN/ })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "历史" })).toBeInTheDocument();

    expect(screen.queryByText(/tenant/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Task|Temporal|Queue|任务 ID/i)).not.toBeInTheDocument();
  });

  it("routes selections through callbacks instead of owning workflow behavior", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const onSelectHistory = vi.fn();

    render(
      <ProductWorkspaceNavigation
        canonicalItems={buildProductWorkspaceCanonicalNavigation("overview")}
        platformItems={buildProductWorkspacePlatformNavigation(
          [{ platform: "shein", label: "SHEIN", status: "attention" }],
          "shein",
        )}
        onSelect={onSelect}
        onSelectHistory={onSelectHistory}
      />,
    );

    await user.click(screen.getByRole("button", { name: "属性" }));
    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({ key: "attributes" }));

    await user.click(screen.getByRole("button", { name: /SHEIN/ }));
    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ key: "platform:shein", platform: "shein" }),
    );

    await user.click(screen.getByRole("button", { name: "历史" }));
    expect(onSelectHistory).toHaveBeenCalledTimes(1);
  });
});
