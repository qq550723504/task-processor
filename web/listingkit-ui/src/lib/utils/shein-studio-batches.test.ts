import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  listSheinStudioBatches,
  loadSheinStudioDraft,
  saveSheinStudioBatch,
  saveSheinStudioDraft,
} from "@/lib/utils/shein-studio-batches";

describe("shein studio storage api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads draft from server api", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          draft: {
            prompt: "retro cherries",
            styleCount: "4",
            sheinStoreId: "12",
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
        }),
      ),
    );

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
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            batch: {
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
          }),
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            batches: [
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
            ],
          }),
        ),
      );

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
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          draft: {
            prompt: "retro cherries",
            styleCount: "4",
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
        }),
      ),
    );

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
