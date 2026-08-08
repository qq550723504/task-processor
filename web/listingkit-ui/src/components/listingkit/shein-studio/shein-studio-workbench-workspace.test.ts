import { describe, expect, it } from "vitest";

import {
  getSheinStudioBatchSaveValidationError,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-workspace";

describe("Shein Studio batch save validation", () => {
  it("requires exactly one reference image for hot-selling-reference mode", () => {
    expect(
      getSheinStudioBatchSaveValidationError({
        artworkGenerationMode: "hot_reference",
        hotStyleReferenceImageUrls: [],
        prompt: "",
      }),
    ).toBe("保存批次前请先提供 1 张热销款参考图。");

    expect(
      getSheinStudioBatchSaveValidationError({
        artworkGenerationMode: "hot_reference",
        hotStyleReferenceImageUrls: ["ref-1", "ref-2"],
        prompt: "",
      }),
    ).toBe("保存批次前请先提供 1 张热销款参考图。");
  });

  it("requires a theme prompt outside hot-selling-reference mode", () => {
    expect(
      getSheinStudioBatchSaveValidationError({
        artworkGenerationMode: "theme_prompt",
        hotStyleReferenceImageUrls: [],
        prompt: "  ",
      }),
    ).toBe("保存批次前请先填写主题提示词。");
  });
});
