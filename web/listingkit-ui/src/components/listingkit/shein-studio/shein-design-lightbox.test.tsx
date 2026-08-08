import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/image", () => ({
  default: (props: React.ImgHTMLAttributes<HTMLImageElement>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img alt={props.alt ?? ""} {...props} />
  ),
}));

import { SheinDesignLightbox } from "@/components/listingkit/shein-studio/shein-design-lightbox";
import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

describe("SheinDesignLightbox", () => {
  it.each([
    {
      label: "ordinary",
      design: {
        id: "design-ordinary",
        imageUrl: "/api/v1/listing-kits/uploads/files/ordinary.png",
        backgroundRemovalStatus: "not_requested",
        transparentBackgroundMode: "none",
      },
    },
    {
      label: "pending",
      design: {
        id: "design-pending",
        imageUrl: "/api/v1/listing-kits/uploads/files/pending-image.png",
        backgroundRemovalStatus: "pending",
        transparentBackgroundMode: "removal",
      },
    },
    {
      label: "failed",
      design: {
        id: "design-failed",
        imageUrl: "/api/v1/listing-kits/uploads/files/failed-image.png",
        backgroundRemovalStatus: "failed",
        transparentBackgroundMode: "removal",
      },
    },
  ])("labels the $label current image as original and normalizes it", ({ design }) => {
    render(
      <SheinDesignLightbox
        activeView="design"
        design={design as SheinStudioGeneratedDesign}
        onClose={vi.fn()}
        onViewChange={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "原图" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "抠图后" })).not.toBeInTheDocument();
    expect(screen.getByAltText("生成款式预览")).toHaveAttribute(
      "src",
      `/api/listing-kits/uploads/files/${design.id === "design-ordinary" ? "ordinary.png" : design.id === "design-pending" ? "pending-image.png" : "failed-image.png"}`,
    );
  });

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
          backgroundRemovalStatus: "succeeded",
          transparentBackgroundMode: "removal",
        }}
        onClose={vi.fn()}
        onViewChange={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "抠图后" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "查看原图" }));

    expect(screen.getByAltText("生成款式预览")).toHaveAttribute(
      "src",
      "/api/listing-kits/uploads/files/original-image.png",
    );

    expect(screen.getByRole("button", { name: "查看抠图后" })).toBeInTheDocument();
  });
});
