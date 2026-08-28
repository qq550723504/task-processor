import {
  collectSheinPreviewImageGroups,
  collectSheinPreviewImages,
  getSelectableSheinPreviewImages,
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

  it("separates unselected source images from the default submit image list", () => {
    const groups = collectSheinPreviewImageGroups({
      final_review: {
        images: [
          {
            url: "https://cdn.example.com/generated-main.jpg",
            origin: "generated",
            selected: true,
            final: true,
          },
          {
            url: "https://1688.example.com/source-1.jpg",
            origin: "source",
            selected: false,
            final: false,
            requires_review: true,
          },
        ],
      },
    });

    expect(groups.productImages.map((image) => image.url)).toEqual([
      "https://cdn.example.com/generated-main.jpg",
    ]);
    expect(groups.availableImages.map((image) => image.url)).toEqual([
      "https://1688.example.com/source-1.jpg",
    ]);
    expect(groups.availableImages[0]).toMatchObject({
      origin: "source",
      requiresReview: true,
    });
  });

  it("keeps a persisted source image available for re-selection after reload", () => {
    const sourceURL = "https://1688.example.com/source-1.jpg";
    const groups = collectSheinPreviewImageGroups({
      source_product: {
        image_urls: [sourceURL],
      },
      final_review: {
        images: [
          {
            url: "https://cdn.example.com/generated-main.jpg",
            origin: "generated",
            selected: true,
            final: true,
          },
          {
            url: sourceURL,
            origin: "source",
            selected: true,
            final: true,
            requires_review: true,
          },
        ],
      },
    });

    expect(groups.productImages.map((image) => image.url)).toEqual([
      "https://cdn.example.com/generated-main.jpg",
      sourceURL,
    ]);
    expect(groups.availableImages.map((image) => image.url)).toEqual([sourceURL]);
    expect(groups.availableImages[0]).toMatchObject({
      origin: "source",
      requiresReview: true,
    });
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

describe("getSelectableSheinPreviewImages", () => {
  it("excludes available images already present in product images", () => {
    const selected = {
      id: "selected",
      label: "Selected source",
      url: "https://1688.example.com/source-1.jpg",
    };
    const available = [
      selected,
      {
        id: "unselected",
        label: "Unselected source",
        url: "https://1688.example.com/source-2.jpg",
      },
    ];

    expect(getSelectableSheinPreviewImages([selected], available)).toEqual([
      available[1],
    ]);
  });
});
