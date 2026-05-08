import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import {
  SheinStyleGalleryPage,
  formatImageDimensions,
} from "@/components/listingkit/shein-studio/shein-style-gallery-page";
import { SHEIN_STUDIO_GALLERY_HANDOFF_STORAGE_KEY } from "@/lib/shein-studio/gallery-handoff";

describe("formatImageDimensions", () => {
  it("formats loaded image dimensions", () => {
    expect(formatImageDimensions(1024, 1536)).toBe("1024 x 1536px");
  });

  it("hides missing image dimensions", () => {
    expect(formatImageDimensions(0, 1536)).toBe("");
    expect(formatImageDimensions(1024)).toBe("");
  });
});

describe("SheinStyleGalleryPage", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("stores the selected gallery image before entering SHEIN Studio", () => {
    render(
      <SheinStyleGalleryPage
        initialGallery={{
          items: [
            {
              id: "style-1",
              imageUrl: "https://example.com/style.png",
              source: "studio_saved",
              sourceLabel: "已保存 AI 图",
              title: "Retro style",
              prompt: "retro cherries",
            },
          ],
          summary: {
            publishedInputs: 0,
            studioLegacy: 0,
            studioSaved: 1,
          },
          total: 1,
        }}
      />,
    );

    const image = screen.getByAltText("Retro style");
    Object.defineProperty(image, "naturalWidth", { configurable: true, value: 1024 });
    Object.defineProperty(image, "naturalHeight", { configurable: true, value: 1536 });
    fireEvent.load(image);

    const action = screen.getByRole("link", { name: "生成 SHEIN 任务" });
    action.addEventListener("click", (event) => event.preventDefault());
    fireEvent.click(action);

    expect(
      JSON.parse(
        window.localStorage.getItem(SHEIN_STUDIO_GALLERY_HANDOFF_STORAGE_KEY) ?? "{}",
      ),
    ).toMatchObject({
      id: "style-1",
      imageUrl: "https://example.com/style.png",
      prompt: "retro cherries",
      title: "Retro style",
      width: 1024,
      height: 1536,
    });
  });
});
