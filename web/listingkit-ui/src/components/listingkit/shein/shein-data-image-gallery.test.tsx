import { render, screen } from "@testing-library/react";
import { vi } from "vitest";

import { SheinDataImageGallery } from "@/components/listingkit/shein/shein-data-image-gallery";

describe("SheinDataImageGallery", () => {
  it("shows SDS mockups in a separate reference section", () => {
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "product-main",
            label: "Preview product image 1",
            url: "http://local/product-main.png",
          },
        ]}
        mockupImages={[
          {
            id: "mockup-1",
            label: "SDS mockup 1",
            url: "https://cdn.sdspod.com/out/mockup-main.jpg",
          },
        ]}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("SHEIN 提交图片")).toBeInTheDocument();
    expect(screen.getByText("SDS Mockup 渲染参考")).toBeInTheDocument();
    expect(screen.getByText("Preview product image 1")).toBeInTheDocument();
    expect(screen.getByText("SDS mockup 1")).toBeInTheDocument();
    expect(screen.getByText("最终提交 1 / 1 张")).toBeInTheDocument();
  });

  it("shows single-variant swatch and SKC fallbacks as covered by the main image", () => {
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "product-main",
            label: "Preview product image 1",
            url: "http://local/product-main.png",
          },
        ]}
        finalImages={[
          {
            url: "http://local/product-main.png",
            role: "main",
            main: true,
          },
        ]}
        variantCount={1}
        onSelect={vi.fn()}
        onSaveImageControls={vi.fn()}
      />,
    );

    expect(screen.getAllByText("默认使用首图")).toHaveLength(2);
    expect(screen.getAllByText("未设置")).toHaveLength(2);
  });
});
