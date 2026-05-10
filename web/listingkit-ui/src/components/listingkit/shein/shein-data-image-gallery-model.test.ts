import { describe, expect, it } from "vitest";

import {
  buildImageRoleOverrides,
  hasSavedImageRole,
  moveItem,
  normalizeImageRole,
  roleLabel,
  suggestImageRoles,
} from "@/components/listingkit/shein/shein-data-image-gallery-model";

const images = [
  { id: "main", label: "Main image", url: "https://example.com/main.jpg" },
  { id: "size", label: "Size chart", url: "https://example.com/size.jpg" },
  { id: "gallery", label: "Gallery", url: "https://example.com/gallery.jpg" },
];

describe("shein data image gallery model", () => {
  it("marks a gallery role as main when it is the selected main url", () => {
    expect(
      buildImageRoleOverrides(
        ["https://example.com/main.jpg", "https://example.com/gallery.jpg"],
        {
          "https://example.com/main.jpg": "gallery",
          "https://example.com/gallery.jpg": "swatch",
        },
        "https://example.com/main.jpg",
      ),
    ).toEqual({
      "https://example.com/main.jpg": "main",
      "https://example.com/gallery.jpg": "swatch",
    });
  });

  it("suggests main, size map, and swatch roles from image labels", () => {
    expect(suggestImageRoles(images, {}, images[0].url)).toEqual({
      mainUrl: "https://example.com/main.jpg",
      roles: {
        "https://example.com/main.jpg": "main",
        "https://example.com/size.jpg": "size_map",
        "https://example.com/gallery.jpg": "swatch",
      },
    });
  });

  it("detects saved role metadata", () => {
    expect(hasSavedImageRole([{ url: "a", role: "skc" }])).toBe(true);
    expect(hasSavedImageRole([{ url: "a", role: "unknown" }])).toBe(false);
  });

  it("normalizes roles and labels", () => {
    expect(normalizeImageRole("size_map")).toBe("size_map");
    expect(normalizeImageRole("unknown")).toBeUndefined();
    expect(roleLabel("skc")).toBe("SKC 图");
    expect(roleLabel("gallery")).toBe("图库");
  });

  it("moves image urls within bounds", () => {
    expect(moveItem(["a", "b", "c"], "b", -1)).toEqual(["b", "a", "c"]);
    expect(moveItem(["a", "b", "c"], "c", 1)).toEqual(["a", "b", "c"]);
  });
});
