import { beforeEach, describe, expect, it, vi } from "vitest";

import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  getSheinStudioSession,
  mapStudioSessionDetailToBatch,
  mapStudioSessionDetailToDraft,
  replaceSheinStudioSessionDesigns,
  upsertSheinStudioSessionBatch,
  updateSheinStudioSession,
} from "@/lib/api/shein-studio-sessions";
import { apiAsyncRequest, apiRequest } from "@/lib/api/client";

vi.mock("@/lib/api/client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api/client")>(
    "@/lib/api/client",
  );
  return {
    ...actual,
    apiAsyncRequest: vi.fn(),
    apiRequest: vi.fn(),
  };
});

const mockedApiAsyncRequest = vi.mocked(apiAsyncRequest);
const mockedApiRequest = vi.mocked(apiRequest);

describe("shein studio design metadata", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("stamps generated designs with prompt and generation model metadata", async () => {
    mockedApiAsyncRequest.mockResolvedValueOnce({
      prompt: "retro botanical clock",
      image_model: "gpt-image-2",
      transparent_background: true,
      warnings: ["fallback applied"],
      images: [
        {
          id: "design-1",
          image_url: "https://oss.example.com/design-1.png",
          revised_prompt: "revised",
        },
      ],
    });

    const result = await generateSheinStudioDesigns({
      prompt: "retro botanical clock",
      count: 1,
      variationIntensity: "light",
      imageModel: "gpt-image-2",
      transparentBackground: true,
    });

    expect(result.images[0]).toMatchObject({
      id: "design-1",
      prompt: "retro botanical clock",
      imageModel: "gpt-image-2",
      transparentBackground: true,
      variationIntensity: "light",
      revisedPrompt: "revised",
    });
    expect(result.warnings).toEqual(["fallback applied"]);
  });

  it("passes a custom image model through when transparent background is off", async () => {
    mockedApiAsyncRequest.mockResolvedValueOnce({
      prompt: "retro botanical clock",
      image_model: "custom-image-model",
      transparent_background: false,
      images: [],
    });

    await generateSheinStudioDesigns({
      prompt: "retro botanical clock",
      count: 1,
      imageModel: "custom-image-model",
      transparentBackground: false,
    });

    expect(mockedApiAsyncRequest).toHaveBeenCalledWith(
      "/studio/designs",
      expect.objectContaining({
        body: expect.objectContaining({
          image_model: "custom-image-model",
        }),
      }),
    );
  });

  it("omits image model when using backend default model", async () => {
    mockedApiAsyncRequest.mockResolvedValueOnce({
      prompt: "retro botanical clock",
      transparent_background: false,
      images: [],
    });

    await generateSheinStudioDesigns({
      prompt: "retro botanical clock",
      count: 1,
      imageModel: "",
      transparentBackground: false,
    });

    expect(mockedApiAsyncRequest).toHaveBeenCalledWith(
      "/studio/designs",
      expect.objectContaining({
        body: expect.objectContaining({
          image_model: undefined,
        }),
      }),
    );
  });

  it("persists design prompt and model metadata in studio sessions", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: "session-1" },
      designs: [],
    });

    await replaceSheinStudioSessionDesigns("session-1", {
      approvedDesignIds: ["design-1"],
      designs: [
        {
          id: "design-1",
          imageUrl: "https://oss.example.com/design-1.png",
          prompt: "retro botanical clock",
          imageModel: "gpt-image-2",
          transparentBackground: true,
          variationIntensity: "light",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "1200 x 1200",
        },
      ],
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/sessions/session-1/designs",
      expect.objectContaining({
        body: expect.objectContaining({
          designs: [
            expect.objectContaining({
              prompt: "retro botanical clock",
              image_model: "gpt-image-2",
              transparent_background: true,
              variation_intensity: "light",
              target_group_key: "size:1200x1200",
              target_group_label: "1200 x 1200",
            }),
          ],
        }),
      }),
    );
  });

  it("loads design prompt and model metadata from studio sessions", () => {
    const draft = mapStudioSessionDetailToDraft({
      session: {
        id: "session-1",
        prompt: "fallback session prompt",
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            current_prompt: "prompt a",
            prompt_history: [
              {
                prompt: "prompt old",
                grouped_image_mode: "shared_by_size",
                created_at: "2026-05-26T00:00:00Z",
              },
            ],
            primary_selection: {
              product_id: 1,
              parent_product_id: 1,
              variant_id: 100,
              prototype_group_id: 200,
              layer_id: "layer-1",
              product_name: "tee",
              variant_label: "M / black",
            },
            grouped_selections: [
              {
                selection_id: "1:200:101:layer-2:101",
                selection: {
                  product_id: 1,
                  parent_product_id: 1,
                  variant_id: 101,
                  prototype_group_id: 200,
                  layer_id: "layer-2",
                  product_name: "hoodie",
                  variant_label: "L / white",
                },
                baseline_status: "baseline_cached",
                baseline_reason: "基础模板已缓存，等待进一步校验",
                shein_store_id: "store-9",
                eligible: true,
              },
            ],
            shein_store_id: "store-9",
            image_strategy: "sds_official",
            grouped_image_mode: "shared_by_size",
            selected_sds_images: [],
            render_size_images_with_sds: true,
            product_image_count: "5",
            product_image_prompt: "",
            product_image_prompts: [],
            artwork_model: "",
            transparent_background: false,
            variation_intensity: "medium",
            approved_design_ids: [],
            created_tasks: [],
            designs: [],
            updated_at: "2026-05-26T00:00:00Z",
          },
        ],
        grouped_selections: [
          {
            selection_id: "1:200:101:layer-2:101",
            selection: {
              product_id: 1,
              parent_product_id: 1,
              variant_id: 101,
              prototype_group_id: 200,
              layer_id: "layer-2",
              product_name: "hoodie",
              variant_label: "L / white",
                },
                baseline_status: "blocked",
                baseline_reason: "图层信息无效",
                baseline_reason_code: "layer_missing",
                shein_store_id: "store-9",
                eligible: true,
              },
        ],
      },
      designs: [
        {
          id: "design-1",
          image_url: "https://oss.example.com/design-1.png",
          prompt: "retro botanical clock",
          image_model: "gpt-image-2",
          transparent_background: true,
          variation_intensity: "light",
          target_group_key: "size:1200x1200",
          target_group_label: "1200 x 1200",
        },
      ],
    });

    expect(draft?.designs[0]).toMatchObject({
      prompt: "retro botanical clock",
      imageModel: "gpt-image-2",
      transparentBackground: true,
      variationIntensity: "light",
      targetGroupKey: "size:1200x1200",
      targetGroupLabel: "1200 x 1200",
    });
    expect(draft?.groupedSelections).toEqual([
      expect.objectContaining({
        selectionId: "1:200:101:layer-2:101",
        sheinStoreId: "store-9",
        baselineStatus: "blocked",
        baselineReason: "图层信息无效",
        baselineReasonCode: "layer_missing",
        selection: expect.objectContaining({
          variantId: 101,
          productName: "hoodie",
        }),
      }),
    ]);
    expect(draft?.groups).toEqual([
      expect.objectContaining({
        id: "group-1",
        currentPrompt: "prompt a",
        promptHistory: [
          {
            prompt: "prompt old",
            groupedImageMode: "shared_by_size",
            createdAt: "2026-05-26T00:00:00Z",
          },
        ],
      }),
    ]);
  });

  it("sends grouped selections when updating a studio session", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: "session-1" },
      designs: [],
    });

    await updateSheinStudioSession("session-1", {
      groupedImageMode: "per_product",
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
          baselineReasonCode: "cache_unavailable",
          sheinStoreId: "store-9",
          eligible: true,
        },
      ],
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/sessions/session-1",
      expect.objectContaining({
        body: expect.objectContaining({
          grouped_image_mode: "per_product",
          grouped_selections: [
            expect.objectContaining({
              selection_id: "1:200:101:layer-2:101",
              baseline_status: "ready",
              baseline_reason_code: "cache_unavailable",
              shein_store_id: "store-9",
            }),
          ],
        }),
      }),
    );
  });

  it("sends grouped workspaces when updating a studio session", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: "session-1" },
      designs: [],
    });

    await updateSheinStudioSession("session-1", {
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
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/sessions/session-1",
      expect.objectContaining({
        body: expect.objectContaining({
          grouped_selections: undefined,
          groups: [
            expect.objectContaining({
              id: "group-1",
              current_prompt: "prompt a",
            }),
          ],
        }),
      }),
    );
  });

  it("sends grouped selections when saving a studio batch", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: "batch-1" },
      designs: [],
    });

    await upsertSheinStudioSessionBatch({
      prompt: "retro botanical clock",
      styleCount: "2",
      groupedImageMode: "shared_by_size",
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
          baselineReasonCode: "cache_unavailable",
          sheinStoreId: "store-9",
          eligible: true,
        },
      ],
      approvedDesignIds: [],
      createdTasks: [],
      designs: [],
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/batches",
      expect.objectContaining({
        body: expect.objectContaining({
          grouped_image_mode: "shared_by_size",
          grouped_selections: [
            expect.objectContaining({
              selection_id: "1:200:101:layer-2:101",
              baseline_status: "ready",
              baseline_reason_code: "cache_unavailable",
              shein_store_id: "store-9",
            }),
          ],
        }),
      }),
    );
  });

  it("sends grouped workspaces when saving a studio batch", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: "batch-1" },
      designs: [],
    });

    await upsertSheinStudioSessionBatch({
      prompt: "retro botanical clock",
      styleCount: "2",
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
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
      approvedDesignIds: [],
      createdTasks: [],
      designs: [],
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/batches",
      expect.objectContaining({
        body: expect.objectContaining({
          grouped_selections: undefined,
          groups: [
            expect.objectContaining({
              id: "group-1",
              current_prompt: "prompt a",
            }),
          ],
        }),
      }),
    );
  });

  it("maps empty-prompt batch containers from session detail", () => {
    const batch = mapStudioSessionDetailToBatch({
      session: {
        id: "batch-1",
        batch_name: "批次12",
        prompt: "",
        status: "selecting",
        selection: {
          product_id: 212094,
          parent_product_id: 212094,
          variant_id: 212095,
          prototype_group_id: 26098,
          layer_id: "857829356162756609",
          product_name: "带刻度方形挂钟25*25（包邮仅限美国直发）",
          variant_label: "10×10inch(25×25cm) · White",
          printable_width: 1500,
          printable_height: 1500,
          selected_variant_ids: [212095],
        },
        grouped_selections: [
          {
            selection_id: "212094:26098:212095:857829356162756609:212095",
            selection: {
              product_id: 212094,
              parent_product_id: 212094,
              variant_id: 212095,
              prototype_group_id: 26098,
              layer_id: "857829356162756609",
              product_name: "带刻度方形挂钟25*25（包邮仅限美国直发）",
              variant_label: "10×10inch(25×25cm) · White",
              printable_width: 1500,
              printable_height: 1500,
              selected_variant_ids: [212095],
            },
            baseline_status: "baseline_cached",
            baseline_reason: "基础模板已缓存，等待进一步校验",
            baseline_reason_code: "cache_unavailable",
            shein_store_id: "",
            eligible: true,
          },
        ],
      },
      designs: [],
    });

    expect(batch).toMatchObject({
      id: "batch-1",
      name: "批次12",
      prompt: "",
      selection: expect.objectContaining({
        variantId: 212095,
      }),
      groupedSelections: [
        expect.objectContaining({
          baselineStatus: "baseline_cached",
          baselineReason: "基础模板已缓存，等待进一步校验",
          baselineReasonCode: "cache_unavailable",
          selection: expect.objectContaining({
            variantId: 212095,
          }),
        }),
      ],
    });
  });

  it("rejects studio session responses without a string session id", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: 123 },
      designs: [],
    });

    await expect(getSheinStudioSession("session-1")).rejects.toMatchObject({
      message: "ListingKit API returned an unexpected studio session response",
      status: 502,
    });
  });

  it("rejects studio session designs without a string design id", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: "session-1" },
      designs: [{ id: 123, image_url: "https://oss.example.com/design.png" }],
    });

    await expect(getSheinStudioSession("session-1")).rejects.toMatchObject({
      message: "ListingKit API returned an unexpected studio session response",
      status: 502,
    });
  });
});
