import {
  collectSheinPreviewImageGroups,
  collectSheinPreviewImages,
} from "@/components/listingkit/shein/shein-preview-image";
import type { SheinPreviewPayload } from "@/lib/types/listingkit";

describe("collectSheinPreviewImages", () => {
  it("prioritizes final SHEIN preview images over source images", () => {
    const shein: SheinPreviewPayload = {
      source_product: {
        image_urls: ["http://local/source.png"],
      },
      draft_payload: {
        image_info: {
          main_image: "https://cdn.sdspod.com/out/request-main.jpg",
          source: ["http://local/source.png"],
        },
      },
      preview_payload: {
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

  it("uses final review images as the authoritative submit image list", () => {
    const shein: SheinPreviewPayload = {
      final_review: {
        images: [
          { url: "https://cdn.sdspod.com/out/final-main.jpg", role: "main" },
          { url: "https://cdn.sdspod.com/out/final-gallery.jpg", role: "gallery" },
        ],
      },
      preview_payload: {
        image_info: {
          image_info_list: [
            { image_url: "http://local/stale-ai-main.png" },
            { image_url: "http://local/stale-ai-gallery.png" },
          ],
        },
      },
    };

    const groups = collectSheinPreviewImageGroups(shein, {
      mockup_image_urls: [
        "https://cdn.sdspod.com/out/final-main.jpg",
        "https://cdn.sdspod.com/out/final-gallery.jpg",
      ],
    });

    expect(groups.productImages.map((image) => image.url)).toEqual([
      "https://cdn.sdspod.com/out/final-main.jpg",
      "https://cdn.sdspod.com/out/final-gallery.jpg",
    ]);
    expect(groups.mockupImages).toEqual([]);
  });

  it("separates SHEIN product images from SDS mockup renderings", () => {
    const shein: SheinPreviewPayload = {
      preview_payload: {
        image_info: {
          image_info_list: [
            { image_url: "http://local/product-main.png" },
            { image_url: "http://local/product-gallery.png" },
          ],
        },
      },
    };

    const groups = collectSheinPreviewImageGroups(shein, {
      mockup_image_urls: [
        "https://cdn.sdspod.com/out/mockup-main.jpg",
        "https://cdn.sdspod.com/out/mockup-gallery.jpg",
      ],
    });

    expect(groups.productImages.map((image) => image.url)).toEqual([
      "http://local/product-main.png",
      "http://local/product-gallery.png",
    ]);
    expect(groups.mockupImages.map((image) => image.url)).toEqual([
      "https://cdn.sdspod.com/out/mockup-main.jpg",
      "https://cdn.sdspod.com/out/mockup-gallery.jpg",
    ]);
  });

  it("does not repeat SDS mockups in the reference group once they are final product images", () => {
    const shein: SheinPreviewPayload = {
      preview_payload: {
        image_info: {
          image_info_list: [
            { image_url: "https://cdn.sdspod.com/out/mockup-main.jpg" },
            { image_url: "https://cdn.sdspod.com/out/mockup-gallery.jpg" },
          ],
        },
      },
    };

    const groups = collectSheinPreviewImageGroups(shein, {
      mockup_image_urls: [
        "https://cdn.sdspod.com/out/mockup-main.jpg",
        "https://cdn.sdspod.com/out/mockup-gallery.jpg",
      ],
    });

    expect(groups.productImages.map((image) => image.url)).toEqual([
      "https://cdn.sdspod.com/out/mockup-main.jpg",
      "https://cdn.sdspod.com/out/mockup-gallery.jpg",
    ]);
    expect(groups.mockupImages).toEqual([]);
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
