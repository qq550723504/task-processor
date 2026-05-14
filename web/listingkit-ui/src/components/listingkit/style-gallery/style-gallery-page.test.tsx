import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import {
  StyleGalleryPage,
  formatImageDimensions,
} from "@/components/listingkit/style-gallery/style-gallery-page";
import { STYLE_GALLERY_HANDOFF_STORAGE_KEY } from "@/lib/style-gallery/gallery-handoff";

describe("formatImageDimensions", () => {
  it("formats loaded image dimensions", () => {
    expect(formatImageDimensions(1024, 1536)).toBe("1024 x 1536px");
  });

  it("hides missing image dimensions", () => {
    expect(formatImageDimensions(0, 1536)).toBe("");
    expect(formatImageDimensions(1024)).toBe("");
  });
});

describe("StyleGalleryPage", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("stores the selected gallery image before entering the POD flow", () => {
    render(
      <StyleGalleryPage
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
            taskLinked: 0,
          },
          generatedAt: "2026-05-10T00:00:00.000Z",
          total: 1,
        }}
      />,
    );

    const image = screen.getByAltText("Retro style");
    Object.defineProperty(image, "naturalWidth", { configurable: true, value: 1024 });
    Object.defineProperty(image, "naturalHeight", { configurable: true, value: 1536 });
    fireEvent.load(image);

    expect(screen.getByText("ListingKit 款式图库")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "从 POD 生成" })).toHaveAttribute(
      "href",
      "/listing-kits/sds",
    );

    const action = screen.getByRole("link", { name: "用于生成任务" });
    expect(action).toHaveAttribute("href", "/listing-kits/sds");
    action.addEventListener("click", (event) => event.preventDefault());
    fireEvent.click(action);

    expect(
      JSON.parse(
        window.localStorage.getItem(STYLE_GALLERY_HANDOFF_STORAGE_KEY) ?? "{}",
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

  it("filters loaded gallery images by dimension preset", () => {
    render(
      <StyleGalleryPage
        initialGallery={{
          items: [
            {
              id: "square",
              imageUrl: "https://example.com/square.png",
              source: "studio_saved",
              sourceLabel: "已保存 AI 图",
              title: "Square style",
            },
            {
              id: "portrait",
              imageUrl: "https://example.com/portrait.png",
              source: "studio_saved",
              sourceLabel: "已保存 AI 图",
              title: "Portrait style",
            },
            {
              id: "landscape",
              imageUrl: "https://example.com/landscape.png",
              source: "studio_saved",
              sourceLabel: "已保存 AI 图",
              title: "Landscape style",
            },
          ],
          summary: {
            publishedInputs: 0,
            studioLegacy: 0,
            studioSaved: 3,
            taskLinked: 0,
          },
          generatedAt: "2026-05-10T00:00:00.000Z",
          total: 3,
        }}
      />,
    );

    const square = screen.getByAltText("Square style");
    Object.defineProperty(square, "naturalWidth", { configurable: true, value: 1000 });
    Object.defineProperty(square, "naturalHeight", { configurable: true, value: 1000 });
    fireEvent.load(square);

    const portrait = screen.getByAltText("Portrait style");
    Object.defineProperty(portrait, "naturalWidth", { configurable: true, value: 900 });
    Object.defineProperty(portrait, "naturalHeight", { configurable: true, value: 1200 });
    fireEvent.load(portrait);

    const landscape = screen.getByAltText("Landscape style");
    Object.defineProperty(landscape, "naturalWidth", { configurable: true, value: 1400 });
    Object.defineProperty(landscape, "naturalHeight", { configurable: true, value: 900 });
    fireEvent.load(landscape);

    fireEvent.change(screen.getByLabelText("尺寸筛选"), {
      target: { value: "portrait" },
    });

    expect(screen.getByText("Portrait style")).toBeInTheDocument();
    expect(screen.queryByText("Square style")).not.toBeInTheDocument();
    expect(screen.queryByText("Landscape style")).not.toBeInTheDocument();
  });
});
