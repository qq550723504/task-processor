import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import SdsNewPage from "@/app/listing-kits/sds/new/page";

vi.mock("@/components/listingkit/sds/sds-product-browser", () => ({
  SDSProductBrowser: () => <div>sds product browser</div>,
}));

describe("/listing-kits/sds/new page", () => {
  it("renders the dedicated new-batch selection route", () => {
    render(<SdsNewPage />);

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
});
