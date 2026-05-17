import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  listSheinStudioBatches,
  loadSheinStudioDraft,
  saveSheinStudioBatch,
  saveSheinStudioDraft,
} from "@/lib/utils/shein-studio-batches";

const ensureSheinStudioSession = vi.fn();
const getCachedStudioSessionId = vi.fn();
const mapStudioSessionDetailToDraft = vi.fn();
const listSheinStudioSessionBatches = vi.fn();
const getSheinStudioSessionBatch = vi.fn();
const upsertSheinStudioSessionBatch = vi.fn();
const deleteSheinStudioSessionBatch = vi.fn();
const replaceSheinStudioSessionDesigns = vi.fn();
const updateSheinStudioSession = vi.fn();

vi.mock("@/lib/api/shein-studio-sessions", () => ({
  ensureSheinStudioSession: (...args: unknown[]) => ensureSheinStudioSession(...args),
  getCachedStudioSessionId: (...args: unknown[]) => getCachedStudioSessionId(...args),
  listSheinStudioSessionBatches: (...args: unknown[]) => listSheinStudioSessionBatches(...args),
  getSheinStudioSessionBatch: (...args: unknown[]) => getSheinStudioSessionBatch(...args),
  mapStudioSessionDetailToDraft: (...args: unknown[]) => mapStudioSessionDetailToDraft(...args),
  upsertSheinStudioSessionBatch: (...args: unknown[]) => upsertSheinStudioSessionBatch(...args),
  deleteSheinStudioSessionBatch: (...args: unknown[]) => deleteSheinStudioSessionBatch(...args),
  replaceSheinStudioSessionDesigns: (...args: unknown[]) =>
    replaceSheinStudioSessionDesigns(...args),
  updateSheinStudioSession: (...args: unknown[]) => updateSheinStudioSession(...args),
}));

describe("shein studio storage api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    ensureSheinStudioSession.mockReset();
    getCachedStudioSessionId.mockReset();
    listSheinStudioSessionBatches.mockReset();
    getSheinStudioSessionBatch.mockReset();
    mapStudioSessionDetailToDraft.mockReset();
    upsertSheinStudioSessionBatch.mockReset();
    deleteSheinStudioSessionBatch.mockReset();
    replaceSheinStudioSessionDesigns.mockReset();
    updateSheinStudioSession.mockReset();
  });

  it("loads draft from server api", async () => {
    ensureSheinStudioSession.mockResolvedValue({ session: { id: "session-1" } });
    mapStudioSessionDetailToDraft.mockReturnValue({ prompt: "retro cherries" });

    const draft = await loadSheinStudioDraft({
      productId: 1,
      parentProductId: 1,
      variantId: 100,
      prototypeGroupId: 200,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
    });

    expect(draft?.prompt).toBe("retro cherries");
  });

  it("saves batch snapshots through server api", async () => {
    upsertSheinStudioSessionBatch.mockResolvedValue({
      id: "batch-1",
      name: "retro cherries",
      prompt: "retro cherries",
      styleCount: "3",
      sheinStoreId: "",
      selectionVariantId: 100,
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      designs: [
        {
          id: "design-1",
          dataUrl: "data:image/png;base64,abc",
        },
      ],
      selectedIds: ["design-1"],
      createdTasks: [],
      updatedAt: "2026-04-24T00:00:00.000Z",
    });
    listSheinStudioSessionBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "retro cherries",
        prompt: "retro cherries",
        styleCount: "3",
        sheinStoreId: "",
        selectionVariantId: 100,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        designs: [
          {
            id: "design-1",
            dataUrl: "data:image/png;base64,abc",
          },
        ],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
    ]);

    const saved = await saveSheinStudioBatch({
      prompt: "retro cherries",
      styleCount: "3",
      sheinStoreId: "",
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      designs: [
        {
          id: "design-1",
          dataUrl: "data:image/png;base64,abc",
        },
      ],
      selectedIds: ["design-1"],
      createdTasks: [],
    });

    expect(saved?.prompt).toBe("retro cherries");

    const batches = await listSheinStudioBatches();
    expect(batches).toHaveLength(1);
    expect(batches[0]?.prompt).toBe("retro cherries");
  });

  it("saves draft through server api", async () => {
    getCachedStudioSessionId.mockReturnValue(undefined);
    ensureSheinStudioSession.mockResolvedValue({ session: { id: "session-1" } });
    updateSheinStudioSession.mockResolvedValue({ session: { id: "session-1" } });
    mapStudioSessionDetailToDraft.mockReturnValue({ prompt: "retro cherries" });

    const saved = await saveSheinStudioDraft({
      prompt: "retro cherries",
      styleCount: "4",
      sheinStoreId: "",
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    expect(saved?.prompt).toBe("retro cherries");
  });
});
