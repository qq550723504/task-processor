import { beforeEach, describe, expect, it, vi } from "vitest";

import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  mapStudioBatchDraftDetailToBatch,
  mapStudioBatchDraftDetailToDraft,
  upsertSheinStudioBatchDraft,
} from "@/lib/api/shein-studio-batch-drafts";
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

  it("loads design prompt and model metadata from studio batch drafts", () => {
    const draft = mapStudioBatchDraftDetailToDraft({
      batch: {
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

  it("sends grouped selections when saving a studio batch", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      batch: { id: "batch-1" },
      designs: [],
    });

    await upsertSheinStudioBatchDraft({
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

  it("ignores grouped workspaces when saving a studio batch because the backend only persists grouped selections", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      batch: { id: "batch-1" },
      designs: [],
    });

    await upsertSheinStudioBatchDraft({
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
        }),
      }),
    );
  });

  it("keeps top-level grouped selections when saving a legacy batch with empty groups", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      batch: { id: "batch-1" },
      designs: [],
    });

    await upsertSheinStudioBatchDraft({
      id: "batch-1",
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
      groups: [],
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
      approvedDesignIds: [],
      createdTasks: [],
      designs: [],
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/batches",
      expect.objectContaining({
        body: expect.objectContaining({
          grouped_selections: [
            expect.objectContaining({
              selection_id: "1:200:101:layer-2:101",
              shein_store_id: "store-9",
            }),
          ],
        }),
      }),
    );
  });

  it("maps empty-prompt batch containers from session detail", () => {
    const batch = mapStudioBatchDraftDetailToBatch({
      batch: {
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
      groupedSelections: [],
    });
  });

  it("maps generation jobs from batch draft detail into the draft", () => {
    const draft = mapStudioBatchDraftDetailToDraft({
      batch: {
        id: "session-1",
        prompt: "retro cherries",
        status: "generating",
        generation_job_id: "job-primary",
        generation_jobs: [
          {
            job_id: "job-primary",
            target_group_key: "primary",
            target_group_label: "当前商品",
            status: "running",
          },
          {
            job_id: "job-group-1",
            target_group_key: "group-1",
            target_group_label: "分组商品 1",
            status: "running",
          },
        ],
        updated_at: "2026-05-30T00:00:00Z",
      },
      designs: [],
    });

    expect(draft).toMatchObject({
      generationJobId: "job-primary",
      batchStatus: "generating",
      generationJobs: [
        {
          jobId: "job-primary",
          targetGroupKey: "primary",
          targetGroupLabel: "当前商品",
          status: "running",
        },
        {
          jobId: "job-group-1",
          targetGroupKey: "group-1",
          targetGroupLabel: "分组商品 1",
          status: "running",
        },
      ],
    });
  });

  it("normalizes legacy created tasks that use design_id or omit design ids", () => {
    const batch = mapStudioBatchDraftDetailToBatch({
      batch: {
        id: "batch-legacy",
        batch_name: "历史批次",
        prompt: "legacy prompt",
        approved_design_ids: ["design-1", "design-2"],
        created_tasks: [
          {
            id: "task-1",
            title: "Style 1",
            design_id: "design-1",
          },
          {
            id: "task-2",
            title: "Style 2",
          },
        ],
      },
      designs: [
        {
          id: "design-1",
        },
        {
          id: "design-2",
        },
      ],
    });

    expect(batch?.createdTasks).toEqual([
      { id: "task-1", title: "Style 1", designId: "design-1" },
      { id: "task-2", title: "Style 2", designId: "design-2" },
    ]);
  });

  it("does not synthesize a fallback batch name when updating an existing batch", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      batch: {
        id: "batch-1",
        batch_name: "半圆旗帜",
        prompt: "new prompt",
        style_count: "1",
        selection: {
          product_id: 1,
          parent_product_id: 1,
          variant_id: 2,
          prototype_group_id: 3,
          layer_id: "layer-1",
          product_name: "Curtain",
          variant_label: "Blue",
        },
      },
      designs: [],
    });

    await upsertSheinStudioBatchDraft({
      id: "batch-1",
      prompt: "new prompt",
      styleCount: "1",
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 2,
        prototypeGroupId: 3,
        layerId: "layer-1",
        productName: "Curtain",
        variantLabel: "Blue",
      },
      approvedDesignIds: [],
      createdTasks: [],
      designs: [],
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/batches",
      expect.objectContaining({
        body: expect.objectContaining({
          id: "batch-1",
          batch_name: undefined,
        }),
      }),
    );
  });

});

