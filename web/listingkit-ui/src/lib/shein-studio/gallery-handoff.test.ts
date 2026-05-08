import { describe, expect, it } from "vitest";

import {
  consumeSheinStudioGalleryHandoff,
  evaluateSDSRatioMatch,
  galleryHandoffToDesign,
  saveSheinStudioGalleryHandoff,
} from "@/lib/shein-studio/gallery-handoff";

class MemoryStorage implements Pick<Storage, "getItem" | "removeItem" | "setItem"> {
  private readonly values = new Map<string, string>();

  getItem(key: string) {
    return this.values.get(key) ?? null;
  }

  removeItem(key: string) {
    this.values.delete(key);
  }

  setItem(key: string, value: string) {
    this.values.set(key, value);
  }
}

describe("SHEIN Studio gallery handoff", () => {
  it("round-trips a gallery image and converts it into a selected design", () => {
    const storage = new MemoryStorage();
    saveSheinStudioGalleryHandoff(
      {
        createdAt: "2026-05-08T08:00:00.000Z",
        id: "gallery-1",
        imageUrl: "https://example.com/style.png",
        prompt: "retro cherries",
        source: "studio_saved",
        title: "Retro cherry style",
        width: 1024,
        height: 1536,
      },
      storage,
    );

    const handoff = consumeSheinStudioGalleryHandoff(
      storage,
      new Date("2026-05-08T08:05:00.000Z"),
    );

    expect(handoff?.id).toBe("gallery-1");
    expect(storage.getItem("listingkit:shein-studio:gallery-handoff")).toBeNull();
    expect(galleryHandoffToDesign(handoff!)).toEqual({
      id: "gallery-gallery-1",
      imageUrl: "https://example.com/style.png",
      revisedPrompt: "retro cherries",
      role: "gallery",
      roleLabel: "图库导入",
      sourceHeight: 1536,
      sourceWidth: 1024,
    });
  });

  it("ignores expired handoff payloads", () => {
    const storage = new MemoryStorage();
    saveSheinStudioGalleryHandoff(
      {
        createdAt: "2026-05-08T08:00:00.000Z",
        id: "gallery-1",
        imageUrl: "https://example.com/style.png",
        source: "studio_saved",
        title: "Retro cherry style",
      },
      storage,
    );

    expect(
      consumeSheinStudioGalleryHandoff(
        storage,
        new Date("2026-05-09T08:01:00.000Z"),
      ),
    ).toBeNull();
    expect(storage.getItem("listingkit:shein-studio:gallery-handoff")).toBeNull();
  });
});

describe("evaluateSDSRatioMatch", () => {
  it("passes close ratios, warns borderline ratios, and blocks mismatches", () => {
    expect(
      evaluateSDSRatioMatch({
        sourceWidth: 1000,
        sourceHeight: 1000,
        targetWidth: 980,
        targetHeight: 1000,
      }).status,
    ).toBe("pass");

    expect(
      evaluateSDSRatioMatch({
        sourceWidth: 1200,
        sourceHeight: 1000,
        targetWidth: 1000,
        targetHeight: 1000,
      }).status,
    ).toBe("warning");

    expect(
      evaluateSDSRatioMatch({
        sourceWidth: 1400,
        sourceHeight: 1000,
        targetWidth: 1000,
        targetHeight: 1000,
      }).status,
    ).toBe("blocking");
  });
});
