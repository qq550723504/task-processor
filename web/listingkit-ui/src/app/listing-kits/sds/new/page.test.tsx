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
    expect(screen.getByText("第 1 步 · 新建批次")).toBeInTheDocument();
    expect(screen.getByText("sds product browser")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "最近批次" }),
    ).not.toBeInTheDocument();
  });
});
