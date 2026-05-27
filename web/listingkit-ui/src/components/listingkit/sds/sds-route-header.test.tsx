import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SdsRouteHeader } from "@/components/listingkit/sds/sds-route-header";

describe("SdsRouteHeader", () => {
  it("renders title, description, eyebrow, and links", () => {
    render(
      <SdsRouteHeader
        description="选择商品后再进入批次工作台。"
        eyebrow="第 1 步 · 新建批次"
        links={[
          { href: "/listing-kits/sds", label: "返回最近批次首页" },
          { href: "/listing-kits/sds/new", label: "返回新建批次并选品" },
        ]}
        title="选择底版商品和子 SKU"
      />,
    );

    expect(
      screen.getByRole("heading", { name: "选择底版商品和子 SKU" }),
    ).toBeInTheDocument();
    expect(screen.getByText("第 1 步 · 新建批次")).toBeInTheDocument();
    expect(screen.getByText("选择商品后再进入批次工作台。")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "返回最近批次首页" }),
    ).toHaveAttribute("href", "/listing-kits/sds");
    expect(
      screen.getByRole("link", { name: "返回新建批次并选品" }),
    ).toHaveAttribute("href", "/listing-kits/sds/new");
  });

  it("omits the link row when no links are provided", () => {
    render(
      <SdsRouteHeader
        description="这里只有专注选品内容。"
        eyebrow="新建批次"
        links={[]}
        title="专注选品"
      />,
    );

    expect(screen.getByRole("heading", { name: "专注选品" })).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
