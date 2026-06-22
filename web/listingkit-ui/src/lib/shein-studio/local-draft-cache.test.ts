import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  clearLocalSheinStudioDraftSnapshot,
  loadLocalSheinStudioDraftSnapshot,
  loadLocalSheinStudioDraftSnapshotDetail,
  saveLocalSheinStudioDraftSnapshot,
} from "@/lib/shein-studio/local-draft-cache";

const STORAGE_KEY = "listingkit:shein-studio:recent-draft";

describe("local SHEIN Studio draft cache", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-22T00:00:00.000Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    window.localStorage.clear();
  });

  it("loads legacy raw draft snapshots", () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        prompt: "legacy draft",
        updatedAt: "2026-06-21T00:00:00.000Z",
      }),
    );

    expect(loadLocalSheinStudioDraftSnapshot()).toMatchObject({
      prompt: "legacy draft",
      updatedAt: "2026-06-21T00:00:00.000Z",
    });
    expect(loadLocalSheinStudioDraftSnapshotDetail()).toMatchObject({
      batchId: undefined,
      draft: expect.objectContaining({ prompt: "legacy draft" }),
    });
  });

  it("saves normalized draft payloads with a trimmed batch id", () => {
    saveLocalSheinStudioDraftSnapshot(
      {
        prompt: "new draft",
        styleCount: "2",
      },
      { batchId: " batch-1 " },
    );

    expect(loadLocalSheinStudioDraftSnapshotDetail()).toMatchObject({
      batchId: "batch-1",
      draft: expect.objectContaining({
        prompt: "new draft",
        styleCount: "2",
        updatedAt: "2026-06-22T00:00:00.000Z",
      }),
    });
  });

  it("ignores invalid cached JSON without throwing", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    window.localStorage.setItem(STORAGE_KEY, "{bad json");

    expect(loadLocalSheinStudioDraftSnapshot()).toBeNull();
    expect(loadLocalSheinStudioDraftSnapshotDetail()).toBeNull();
    expect(warn).toHaveBeenCalledWith(
      "shein studio local draft snapshot parse failed",
      expect.any(String),
    );
  });

  it("clears the cached snapshot", () => {
    saveLocalSheinStudioDraftSnapshot({
      prompt: "new draft",
      updatedAt: "2026-06-21T00:00:00.000Z",
    });

    clearLocalSheinStudioDraftSnapshot();

    expect(window.localStorage.getItem(STORAGE_KEY)).toBeNull();
    expect(loadLocalSheinStudioDraftSnapshotDetail()).toBeNull();
  });
});
