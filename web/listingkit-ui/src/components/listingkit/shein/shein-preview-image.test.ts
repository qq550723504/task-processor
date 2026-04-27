import { collectSheinPreviewImages } from "@/components/listingkit/shein/shein-preview-image";
import type { SheinPreviewPayload } from "@/lib/types/listingkit";

describe("collectSheinPreviewImages", () => {
  it("prioritizes final SHEIN preview images over source images", () => {
    const shein: SheinPreviewPayload = {
      source_product: {
        image_urls: ["http://local/source.png"],
      },
      request_draft: {
        image_info: {
          main_image: "https://cdn.sdspod.com/out/request-main.jpg",
          source: ["http://local/source.png"],
        },
      },
      preview_product: {
        image_info: {
          image_info_list: [
            { image_url: "https://cdn.sdspod.com/out/final-main.jpg" },
            { image_url: "https://cdn.sdspod.com/out/final-gallery.jpg" },
          ],
        },
      },
    };

    const images = collectSheinPreviewImages(shein, {
      mockup_image_urls: ["https://cdn.sdspod.com/out/final-main.jpg"],
    });

    expect(images.map((image) => image.url)).toEqual([
      "https://cdn.sdspod.com/out/final-main.jpg",
      "https://cdn.sdspod.com/out/final-gallery.jpg",
      "https://cdn.sdspod.com/out/request-main.jpg",
    ]);
    expect(images[0]?.label).toBe("Preview product image 1");
  });

  it("uses SDS mockups when SHEIN preview payload is not available yet", () => {
    const images = collectSheinPreviewImages(null, {
      mockup_image_urls: [
        "https://cdn.sdspod.com/out/main.jpg",
        "https://cdn.sdspod.com/out/gallery.jpg",
      ],
    });

    expect(images.map((image) => image.label)).toEqual([
      "SDS mockup 1",
      "SDS mockup 2",
    ]);
  });

  it("uses source images only when no SHEIN or SDS rendered image exists", () => {
    const images = collectSheinPreviewImages({
      source_product: {
        image_urls: ["http://local/source.png"],
      },
    });

    expect(images.map((image) => image.url)).toEqual(["http://local/source.png"]);
    expect(images[0]?.label).toBe("Source product 1");
  });
});
