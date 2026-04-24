import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  loadTaskCreateDraft,
  saveTaskCreateDraft,
} from "@/components/listingkit/task-create-draft";

describe("task create draft storage", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("saves and loads a draft by task id", () => {
    saveTaskCreateDraft("task_123", {
      text: "Women knit cardigan",
      imageUrls: "https://example.com/1.jpg",
      productUrl: "",
      platforms: ["shein"],
    });

    expect(loadTaskCreateDraft("task_123")).toEqual({
      text: "Women knit cardigan",
      imageUrls: "https://example.com/1.jpg",
      productUrl: "",
      platforms: ["shein"],
      sheinStoreId: "",
      sdsEnabled: false,
      sdsVariantId: "",
      sdsParentProductId: "",
      sdsPrototypeGroupId: "",
      sdsLayerId: "",
      sdsDesignType: "material",
      sdsFitLevel: "1",
      sdsResizeMode: "0",
      sceneCategory: "",
      sceneStyle: "",
      backgroundTone: "",
      composition: "",
      propsLevel: "",
      audienceHint: "",
      customSceneHint: "",
    });
  });

  it("returns null for malformed session storage", () => {
    window.sessionStorage.setItem(
      "listingkit:create-draft:task_123",
      "{not-json",
    );

    expect(loadTaskCreateDraft("task_123")).toBeNull();
  });
});
