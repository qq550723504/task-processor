import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import SdsNewPage from "@/app/listing-kits/sds/new/page";

vi.mock("@/components/listingkit/sds/sds-product-browser", () => ({
  SDSProductBrowser: () => <div>sds product browser</div>,
}));

describe("/listing-kits/sds/new page", () => {
  it("renders the dedicated new-batch selection route", async () => {
    render(await SdsNewPage());

    expect(
      screen.getByRole("heading", { name: "选择底版商品和子 SKU" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "返回最近批次首页" }),
    ).toHaveAttribute("href", "/listing-kits/sds");
    expect(screen.getByText("第 1 步 · 新建批次")).toBeInTheDocument();
    expect(screen.getByText("sds product browser")).toBeInTheDocument();
    expect(
      screen.getByText("这里先专注完成选品；选好后再进入专门的批次工作台继续生成、审核和创建任务。"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "最近批次" }),
    ).not.toBeInTheDocument();
  });

  it("shows the quick single-generation hint when opened from the shortcut", async () => {
    render(
      await SdsNewPage({
        searchParams: Promise.resolve({ entry: "single" }),
      }),
    );

    expect(
      screen.getByText("当前是快速单个生成路径：选 1 个商品后就可以直接进入批次工作台开始生成。"),
    ).toBeInTheDocument();
  });
});
