import path from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import { resolveListingKitUILocalStoragePath } from "@/lib/server/local-storage-path";

describe("resolveListingKitUILocalStoragePath", () => {
  afterEach(() => {
    delete process.env.LISTINGKIT_UI_STORAGE_DIR;
  });

  it("uses .data under the current working directory by default", () => {
    expect(resolveListingKitUILocalStoragePath("state.json")).toBe(
      path.join(process.cwd(), ".data", "state.json"),
    );
  });

  it("uses LISTINGKIT_UI_STORAGE_DIR when configured", () => {
    process.env.LISTINGKIT_UI_STORAGE_DIR = path.join("tmp", "listingkit-ui");

    expect(resolveListingKitUILocalStoragePath("state.json")).toBe(
      path.join("tmp", "listingkit-ui", "state.json"),
    );
  });
});
