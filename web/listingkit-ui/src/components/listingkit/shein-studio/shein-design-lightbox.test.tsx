import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/image", () => ({
  default: (props: React.ImgHTMLAttributes<HTMLImageElement>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img alt={props.alt ?? ""} {...props} />
  ),
}));

import { SheinDesignLightbox } from "@/components/listingkit/shein-studio/shein-design-lightbox";

describe("SheinDesignLightbox", () => {
  it("normalizes uploaded original image URLs before rendering the original view", () => {
    render(
      <SheinDesignLightbox
        activeView="design"
        design={{
          id: "design-1",
          imageUrl:
            "/api/v1/listing-kits/uploads/files/final-image.png",
          originalImageUrl:
            "/api/v1/listing-kits/uploads/files/original-image.png",
        }}
        onClose={vi.fn()}
        onViewChange={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "查看生成原图" }));

    expect(screen.getByAltText("生成款式预览")).toHaveAttribute(
      "src",
      "/api/listing-kits/uploads/files/original-image.png",
    );
  });
});
