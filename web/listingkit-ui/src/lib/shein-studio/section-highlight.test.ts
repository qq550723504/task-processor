import { afterEach, describe, expect, it, vi } from "vitest";

import {
  dispatchSheinStudioSectionFocus,
  resolveSheinStudioSectionFocusAction,
  SHEIN_STUDIO_SECTION_FOCUS_EVENT,
} from "@/lib/shein-studio/section-highlight";

describe("section highlight helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("maps semantic focus actions to stable section ids", () => {
    expect(
      resolveSheinStudioSectionFocusAction({ action: "recent-batches" }),
    ).toBe("shein-studio-recent-batches");
    expect(
      resolveSheinStudioSectionFocusAction({ action: "product-picker" }),
    ).toBe("shein-studio-product-picker");
    expect(
      resolveSheinStudioSectionFocusAction({ sectionId: "custom-section" }),
    ).toBe("custom-section");
  });

  it("dispatches a semantic section focus event", () => {
    const dispatchSpy = vi.spyOn(window, "dispatchEvent");

    dispatchSheinStudioSectionFocus({ action: "product-picker" });

    expect(dispatchSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        type: SHEIN_STUDIO_SECTION_FOCUS_EVENT,
        detail: { action: "product-picker" },
      }),
    );
  });

  it("quietly skips dispatching when window is unavailable", () => {
    const originalWindow = window;
    vi.stubGlobal("window", undefined);

    expect(() =>
      dispatchSheinStudioSectionFocus({ action: "recent-batches" }),
    ).not.toThrow();

    vi.stubGlobal("window", originalWindow);
  });
});
