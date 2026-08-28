import { fireEvent, render, screen } from "@testing-library/react";
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

  it("lets the operator add a source image without selecting it by default", () => {
    const onSaveImageControls = vi.fn();
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "generated-main",
            label: "生成主图",
            url: "https://cdn.example.com/generated-main.jpg",
            origin: "generated",
          },
        ]}
        availableImages={[
          {
            id: "source-1",
            label: "来源图 1",
            url: "https://1688.example.com/source-1.jpg",
            origin: "source",
            requiresReview: true,
          },
        ]}
        onSelect={vi.fn()}
        onSaveImageControls={onSaveImageControls}
      />,
    );

    expect(screen.getByText("最终提交 1 / 1 张")).toBeInTheDocument();
    expect(screen.getByText("可选来源图 1 张")).toBeInTheDocument();
    expect(screen.getByText("来源图 1")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "加入提交图片" }));

    expect(screen.getByText("最终提交 2 / 2 张")).toBeInTheDocument();
    expect(screen.getByText("IP 风险")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "保存图片设置" }));
    expect(onSaveImageControls).toHaveBeenCalledWith(
      expect.objectContaining({
        final_image_order: [
          "https://cdn.example.com/generated-main.jpg",
          "https://1688.example.com/source-1.jpg",
        ],
      }),
    );
  });

  it("returns an added source image to the available list when removed", () => {
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "generated-main",
            label: "生成主图",
            url: "https://cdn.example.com/generated-main.jpg",
          },
        ]}
        availableImages={[
          {
            id: "source-1",
            label: "来源图 1",
            url: "https://1688.example.com/source-1.jpg",
            origin: "source",
            requiresReview: true,
          },
        ]}
        onSelect={vi.fn()}
        onSaveImageControls={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "加入提交图片" }));
    const removeButtons = screen.getAllByRole("button", { name: "从提交中移除" });
    fireEvent.click(removeButtons[removeButtons.length - 1]);

    expect(screen.getByRole("button", { name: "加入提交图片" })).toBeInTheDocument();
    expect(screen.queryByText("IP 风险")).not.toBeInTheDocument();
  });

  it("restores a removed added source image with the global restore action", () => {
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "generated-main",
            label: "生成主图",
            url: "https://cdn.example.com/generated-main.jpg",
          },
        ]}
        availableImages={[
          {
            id: "source-1",
            label: "来源图 1",
            url: "https://1688.example.com/source-1.jpg",
            origin: "source",
            requiresReview: true,
          },
        ]}
        onSelect={vi.fn()}
        onSaveImageControls={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "加入提交图片" }));
    fireEvent.click(screen.getAllByRole("button", { name: "从提交中移除" })[1]);
    fireEvent.click(screen.getByRole("button", { name: "恢复已移除图片" }));

    expect(screen.getByText("最终提交 2 / 2 张")).toBeInTheDocument();
    expect(screen.getByText("IP 风险")).toBeInTheDocument();
  });

  it("reveals a persisted source image after it is removed", () => {
    const sourceURL = "https://1688.example.com/source-1.jpg";
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "generated-main",
            label: "生成主图",
            url: "https://cdn.example.com/generated-main.jpg",
          },
          {
            id: "source-1",
            label: "来源图 1",
            url: sourceURL,
            origin: "source",
            requiresReview: true,
          },
        ]}
        availableImages={[
          {
            id: "source-1-available",
            label: "来源图 1",
            url: sourceURL,
            origin: "source",
            requiresReview: true,
          },
        ]}
        onSelect={vi.fn()}
        onSaveImageControls={vi.fn()}
      />,
    );

    expect(screen.queryByRole("button", { name: "加入提交图片" })).not.toBeInTheDocument();
    expect(screen.queryByText("可选来源图 1 张")).not.toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: "从提交中移除" })[1]);

    expect(screen.getByRole("button", { name: "加入提交图片" })).toBeInTheDocument();
    expect(screen.getByText("可选来源图 1 张")).toBeInTheDocument();
  });

  it("clears a source deletion when the image is added again", () => {
    const onSaveImageControls = vi.fn();
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "generated-main",
            label: "生成主图",
            url: "https://cdn.example.com/generated-main.jpg",
          },
        ]}
        availableImages={[
          {
            id: "source-1",
            label: "来源图 1",
            url: "https://1688.example.com/source-1.jpg",
            origin: "source",
            requiresReview: true,
          },
        ]}
        onSelect={vi.fn()}
        onSaveImageControls={onSaveImageControls}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "加入提交图片" }));
    fireEvent.click(screen.getAllByRole("button", { name: "从提交中移除" })[1]);
    fireEvent.click(screen.getByRole("button", { name: "加入提交图片" }));
    fireEvent.click(screen.getByRole("button", { name: "保存图片设置" }));

    expect(onSaveImageControls).toHaveBeenCalledWith(
      expect.objectContaining({
        final_image_order: [
          "https://cdn.example.com/generated-main.jpg",
          "https://1688.example.com/source-1.jpg",
        ],
        deleted_image_urls: [],
      }),
    );
  });

  it("keeps an added source image in the working order after reordering", () => {
    const onSaveImageControls = vi.fn();
    render(
      <SheinDataImageGallery
        images={[
          {
            id: "generated-main",
            label: "生成主图",
            url: "https://cdn.example.com/generated-main.jpg",
          },
          {
            id: "generated-gallery",
            label: "生成图库图",
            url: "https://cdn.example.com/generated-gallery.jpg",
          },
        ]}
        availableImages={[
          {
            id: "source-1",
            label: "来源图 1",
            url: "https://1688.example.com/source-1.jpg",
            origin: "source",
            requiresReview: true,
          },
        ]}
        onSelect={vi.fn()}
        onSaveImageControls={onSaveImageControls}
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "下移" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "加入提交图片" }));

    expect(screen.getByText("最终提交 3 / 3 张")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "保存图片设置" }));
    expect(onSaveImageControls).toHaveBeenCalledWith(
      expect.objectContaining({
        final_image_order: [
          "https://cdn.example.com/generated-gallery.jpg",
          "https://cdn.example.com/generated-main.jpg",
          "https://1688.example.com/source-1.jpg",
        ],
      }),
    );
  });
});
