import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  getSheinStudioBatch,
  getSheinStudioHydratedBatch,
  listSheinStudioBatches,
  loadSheinStudioDraft,
  saveSheinStudioBatch,
  saveSheinStudioDraft,
} from "@/lib/utils/shein-studio-batches";

const ensureSheinStudioSession = vi.fn();
const buildStudioSessionSelectionKey = vi.fn();
const getCachedStudioSessionId = vi.fn();
const mapStudioSessionDetailToDraft = vi.fn();
const listSheinStudioSessionBatches = vi.fn();
const upsertSheinStudioSessionBatch = vi.fn();
const deleteSheinStudioSessionBatch = vi.fn();
const replaceSheinStudioSessionDesigns = vi.fn();
const updateSheinStudioSession = vi.fn();
const getSheinStudioBatchDetail = vi.fn();

vi.mock("@/lib/api/shein-studio-sessions", () => ({
  ensureSheinStudioSession: (...args: unknown[]) => ensureSheinStudioSession(...args),
  buildStudioSessionSelectionKey: (...args: unknown[]) =>
    buildStudioSessionSelectionKey(...args),
  getCachedStudioSessionId: (...args: unknown[]) => getCachedStudioSessionId(...args),
  listSheinStudioSessionBatches: (...args: unknown[]) => listSheinStudioSessionBatches(...args),
  mapStudioSessionDetailToDraft: (...args: unknown[]) => mapStudioSessionDetailToDraft(...args),
  upsertSheinStudioSessionBatch: (...args: unknown[]) => upsertSheinStudioSessionBatch(...args),
  deleteSheinStudioSessionBatch: (...args: unknown[]) => deleteSheinStudioSessionBatch(...args),
  replaceSheinStudioSessionDesigns: (...args: unknown[]) =>
    replaceSheinStudioSessionDesigns(...args),
  updateSheinStudioSession: (...args: unknown[]) => updateSheinStudioSession(...args),
}));

vi.mock("@/lib/api/shein-studio-batches", () => ({
  getSheinStudioBatchDetail: (...args: unknown[]) => getSheinStudioBatchDetail(...args),
}));

describe("shein studio storage api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    ensureSheinStudioSession.mockReset();
    buildStudioSessionSelectionKey.mockReset();
    buildStudioSessionSelectionKey.mockReturnValue("");
    getCachedStudioSessionId.mockReset();
    listSheinStudioSessionBatches.mockReset();
    mapStudioSessionDetailToDraft.mockReset();
    upsertSheinStudioSessionBatch.mockReset();
    deleteSheinStudioSessionBatch.mockReset();
    replaceSheinStudioSessionDesigns.mockReset();
    updateSheinStudioSession.mockReset();
    getSheinStudioBatchDetail.mockReset();
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
      groupedSelections: [
        {
          selectionId: "1:200:101:layer-2:101",
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "store-9",
          eligible: true,
        },
      ],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00Z",
            },
          ],
          primarySelection: {
            productId: 1,
            parentProductId: 1,
            variantId: 100,
            prototypeGroupId: 200,
            layerId: "layer-1",
            productName: "tee",
            variantLabel: "M / black",
          },
          groupedSelections: [],
          sheinStoreId: "store-9",
          imageStrategy: "sds_official",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T00:00:00Z",
        },
      ],
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
      groupedSelections: [
        {
          selectionId: "1:200:101:layer-2:101",
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "store-9",
          eligible: true,
        },
      ],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00Z",
            },
          ],
          primarySelection: {
            productId: 1,
            parentProductId: 1,
            variantId: 100,
            prototypeGroupId: 200,
            layerId: "layer-1",
            productName: "tee",
            variantLabel: "M / black",
          },
          groupedSelections: [],
          sheinStoreId: "store-9",
          imageStrategy: "sds_official",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T00:00:00Z",
        },
      ],
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
    expect(upsertSheinStudioSessionBatch).toHaveBeenCalledWith(
      expect.objectContaining({
        groupedSelections: [
          expect.objectContaining({
            selectionId: "1:200:101:layer-2:101",
            sheinStoreId: "store-9",
          }),
        ],
        groups: [
          expect.objectContaining({
            id: "group-1",
            currentPrompt: "prompt a",
          }),
        ],
      }),
    );

    const batches = await listSheinStudioBatches();
    expect(batches).toHaveLength(1);
    expect(batches[0]?.prompt).toBe("retro cherries");
  });

  it("does not synthesize a default batch name on create", async () => {
    listSheinStudioSessionBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "批次1",
        prompt: "legacy one",
        styleCount: "1",
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
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "节日专题",
        prompt: "legacy two",
        styleCount: "1",
        sheinStoreId: "",
        selectionVariantId: 101,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 101,
          prototypeGroupId: 200,
          layerId: "layer-2",
          productName: "hoodie",
          variantLabel: "L / white",
        },
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
      {
        id: "batch-3",
        name: "批次7",
        prompt: "legacy seven",
        styleCount: "1",
        sheinStoreId: "",
        selectionVariantId: 102,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 102,
          prototypeGroupId: 200,
          layerId: "layer-3",
          productName: "clock",
          variantLabel: "One size / black",
        },
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
    ]);
    upsertSheinStudioSessionBatch.mockResolvedValue({
      id: "batch-8",
      name: "批次8",
      prompt: "",
      styleCount: "1",
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
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-24T00:00:00.000Z",
    });

    await saveSheinStudioBatch({
      prompt: "",
      styleCount: "1",
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
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    expect(upsertSheinStudioSessionBatch).toHaveBeenCalledWith(
      expect.objectContaining({
        prompt: "",
      }),
    );
    expect(upsertSheinStudioSessionBatch).toHaveBeenCalledWith(
      expect.not.objectContaining({
        name: expect.any(String),
      }),
    );
  });

  it("keeps a saved batch even when prompt is empty", async () => {
    upsertSheinStudioSessionBatch.mockResolvedValue({
      id: "batch-empty-prompt",
      name: "批次12",
      prompt: "",
      styleCount: "1",
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
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-24T00:00:00.000Z",
    });

    const saved = await saveSheinStudioBatch({
      prompt: "",
      styleCount: "1",
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
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    expect(saved).toEqual(
      expect.objectContaining({
        id: "batch-empty-prompt",
        name: "批次12",
        prompt: "",
      }),
    );
  });

  it("maps itemized batch detail back into the saved batch shape", async () => {
    listSheinStudioSessionBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "botanical legacy",
        prompt: "legacy prompt",
        styleCount: "5",
        sheinStoreId: "store-9",
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
        groupedSelections: [
          {
            selectionId: "1:200:101:layer-2:101",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "store-9",
            eligible: true,
          },
        ],
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            currentPrompt: "prompt a",
            promptHistory: [],
            primarySelection: {
              productId: 1,
              parentProductId: 1,
              variantId: 100,
              prototypeGroupId: 200,
              layerId: "layer-1",
              productName: "tee",
              variantLabel: "M / black",
            },
            groupedSelections: [],
            sheinStoreId: "store-9",
            imageStrategy: "sds_official",
            groupedImageMode: "shared_by_size",
            selectedSdsImages: [],
            renderSizeImagesWithSds: true,
            productImageCount: "5",
            productImagePrompt: "",
            productImagePrompts: [],
            artworkModel: "",
            transparentBackground: false,
            variationIntensity: "medium",
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-06-01T10:00:00Z",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [{ id: "task-0", title: "Existing Task", designId: "design-0" }],
        updatedAt: "2026-06-01T10:04:00Z",
      },
    ]);
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "botanical",
        styleCount: "3",
        sheinStoreId: 7,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1200x1200",
            targetGroupLabel: "1200 x 1200",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:05:00Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1200x1200",
              targetGroupLabel: "1200 x 1200",
              imageUrl: "https://cdn.example.com/design-1.png",
              reviewStatus: "approved",
              reviewNote: "looks good",
              createdAt: "2026-06-01T10:01:00Z",
              updatedAt: "2026-06-01T10:05:00Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1200x1200",
              imageUrl: "https://cdn.example.com/design-2.png",
              reviewStatus: "rejected",
              createdAt: "2026-06-01T10:02:00Z",
              updatedAt: "2026-06-01T10:05:00Z",
            },
          ],
        },
      ],
    });

    await expect(getSheinStudioBatch("batch-1")).resolves.toMatchObject({
      id: "batch-1",
      name: "botanical legacy",
      prompt: "botanical",
      styleCount: "3",
      sheinStoreId: "store-9",
      selection: expect.objectContaining({
        variantId: 100,
        productName: "tee",
      }),
      groupedSelections: [
        expect.objectContaining({
          selectionId: "1:200:101:layer-2:101",
          sheinStoreId: "store-9",
        }),
      ],
      groups: [expect.objectContaining({ id: "group-1", name: "Group 1" })],
      createdTasks: [{ id: "task-0", title: "Existing Task", designId: "design-0" }],
      sessionStatus: "review_ready",
      selectedIds: ["design-1"],
      designs: [
        expect.objectContaining({
          id: "design-1",
          imageUrl: "https://cdn.example.com/design-1.png",
          prompt: "botanical",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "1200 x 1200",
          reviewNote: "looks good",
        }),
        expect.objectContaining({
          id: "design-2",
          imageUrl: "https://cdn.example.com/design-2.png",
          prompt: "botanical",
        }),
      ],
      updatedAt: "2026-06-01T10:05:00Z",
    });
  });

  it("surfaces a hydration error when saved batch context cannot be loaded", async () => {
    listSheinStudioSessionBatches.mockRejectedValueOnce(new Error("list failed"));
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "botanical",
        styleCount: "3",
        sheinStoreId: 7,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [],
    });

    await expect(getSheinStudioHydratedBatch("batch-1")).rejects.toThrow(
      "list failed",
    );
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
      groupedSelections: [
        {
          selectionId: "1:200:101:layer-2:101",
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "store-9",
          eligible: true,
        },
      ],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00Z",
            },
          ],
          primarySelection: {
            productId: 1,
            parentProductId: 1,
            variantId: 100,
            prototypeGroupId: 200,
            layerId: "layer-1",
            productName: "tee",
            variantLabel: "M / black",
          },
          groupedSelections: [],
          sheinStoreId: "store-9",
          imageStrategy: "sds_official",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T00:00:00Z",
        },
      ],
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    expect(saved?.prompt).toBe("retro cherries");
    expect(updateSheinStudioSession).toHaveBeenCalledWith(
      "session-1",
      expect.objectContaining({
        groupedSelections: [
          expect.objectContaining({
            selectionId: "1:200:101:layer-2:101",
            sheinStoreId: "store-9",
          }),
        ],
        groups: [
          expect.objectContaining({
            id: "group-1",
            currentPrompt: "prompt a",
          }),
        ],
      }),
      expect.objectContaining({
        signal: undefined,
      }),
    );
  });
});
