import { afterEach, describe, expect, it, vi } from "vitest";

import {
  approveSheinStudioBatchDesigns,
  createSheinStudioBatchTasks,
  generateSheinStudioBatch,
  getSheinStudioBatchDetail,
  parseSheinStudioBatchDetailResponse,
  retrySheinStudioBatchItems,
} from "@/lib/api/shein-studio-batches";

describe("parseSheinStudioBatchDetailResponse", () => {
  it("maps itemized batch detail responses", () => {
    expect(
      parseSheinStudioBatchDetailResponse({
        batch: {
          id: "batch-1",
          status: "draft",
          prompt: "botanical",
          style_count: "3",
          shein_store_id: 7,
          grouped_image_mode: "shared_by_size",
          variation_intensity: "strong",
          artwork_model: "gpt-image-2",
          transparent_background: true,
          selection: {
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
              baseline_status: "ready",
              baseline_reason: "",
              shein_store_id: "869",
              eligible: true,
            },
          ],
          selected_sds_images: [
            { imageUrl: "https://cdn.example.com/sds-1.png", color: "black" },
          ],
          created_at: "2026-06-01T10:00:00Z",
          updated_at: "2026-06-01T10:05:00Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batch_id: "batch-1",
              target_group_key: "size:1200x1200",
              target_group_label: "1200 x 1200",
              status: "review_ready",
              selection_count: 1,
              created_at: "2026-06-01T10:00:00Z",
              updated_at: "2026-06-01T10:05:00Z",
            },
            designs: [
              {
                id: "design-1",
                batch_id: "batch-1",
                item_id: "item-1",
                source_attempt_id: "attempt-1",
                target_group_key: "size:1200x1200",
                target_group_label: "1200 x 1200",
                image_url: "https://cdn.example.com/design-1.png",
                review_status: "approved",
                review_note: "looks good",
                created_at: "2026-06-01T10:01:00Z",
                updated_at: "2026-06-01T10:05:00Z",
              },
            ],
          },
        ],
      }),
    ).toEqual({
      batch: {
        id: "batch-1",
        status: "draft",
        prompt: "botanical",
        styleCount: "3",
        sheinStoreId: 7,
        variationIntensity: "strong",
        artworkModel: "gpt-image-2",
        transparentBackground: true,
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [
          {
            imageUrl: "https://cdn.example.com/sds-1.png",
            color: "black",
            variantSku: undefined,
          },
        ],
        selectionVariantId: 100,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
          printableWidth: undefined,
          printableHeight: undefined,
          templateImageUrl: undefined,
          maskImageUrl: undefined,
          blankDesignUrl: undefined,
          mockupImageUrl: undefined,
          mockupImageUrls: undefined,
          sizeReferenceImageUrls: undefined,
          selectedVariantIds: undefined,
          variants: undefined,
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
              printableWidth: undefined,
              printableHeight: undefined,
              templateImageUrl: undefined,
              maskImageUrl: undefined,
              blankDesignUrl: undefined,
              mockupImageUrl: undefined,
              mockupImageUrls: undefined,
              sizeReferenceImageUrls: undefined,
              selectedVariantIds: undefined,
              variants: undefined,
            },
            baselineKey: undefined,
            baselineStatus: "ready",
            baselineReason: "",
            baselineReasonCode: undefined,
            sheinStoreId: "869",
            eligible: true,
            eligibilityReason: undefined,
          },
        ],
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
            lastError: undefined,
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
              role: undefined,
              roleLabel: undefined,
              productImageUrls: undefined,
              createdAt: "2026-06-01T10:01:00Z",
              updatedAt: "2026-06-01T10:05:00Z",
            },
          ],
        },
      ],
    });
  });

  it("rejects invalid itemized batch detail responses", () => {
    expect(() =>
      parseSheinStudioBatchDetailResponse({
        batch: {
          id: 123,
        },
      }),
    ).toThrow("ListingKit API returned an unexpected studio batch detail response");
  });

  it("keeps the batch shein store id numeric when the backend omits it", () => {
    expect(
      parseSheinStudioBatchDetailResponse({
        batch: {
          id: "batch-1",
          status: "draft",
          prompt: "botanical",
          style_count: "3",
          created_at: "2026-06-01T10:00:00Z",
          updated_at: "2026-06-01T10:05:00Z",
        },
        items: [],
      }),
    ).toMatchObject({
      batch: {
        id: "batch-1",
        status: "draft",
        prompt: "botanical",
        styleCount: "3",
        sheinStoreId: 0,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [],
    });
  });

  it("accepts string shein store ids from the batch detail endpoint", () => {
    expect(
      parseSheinStudioBatchDetailResponse({
        batch: {
          id: "batch-1",
          status: "draft",
          prompt: "botanical",
          style_count: "3",
          shein_store_id: "870",
          created_at: "2026-06-01T10:00:00Z",
          updated_at: "2026-06-01T10:05:00Z",
        },
        items: [],
      }),
    ).toMatchObject({
      batch: {
        id: "batch-1",
        status: "draft",
        prompt: "botanical",
        styleCount: "3",
        sheinStoreId: 870,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [],
    });
  });
});

describe("shein studio batches API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads itemized batch detail from the batch endpoint", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "botanical",
            style_count: "3",
            shein_store_id: 7,
            created_at: "2026-06-01T10:00:00Z",
            updated_at: "2026-06-01T10:05:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batch_id: "batch-1",
                target_group_key: "size:1200x1200",
                status: "review_ready",
                selection_count: 1,
                created_at: "2026-06-01T10:00:00Z",
                updated_at: "2026-06-01T10:05:00Z",
              },
              designs: [
                {
                  id: "design-1",
                  batch_id: "batch-1",
                  item_id: "item-1",
                  source_attempt_id: "attempt-1",
                  target_group_key: "size:1200x1200",
                  image_url: "https://cdn.example.com/design-1.png",
                  review_status: "approved",
                  created_at: "2026-06-01T10:01:00Z",
                  updated_at: "2026-06-01T10:05:00Z",
                },
              ],
            },
          ],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getSheinStudioBatchDetail("batch-1")).resolves.toMatchObject({
      batch: { id: "batch-1", status: "review_ready" },
      items: [{ item: { id: "item-1" }, designs: [{ id: "design-1", reviewStatus: "approved" }] }],
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/listing-kits/studio/batches/batch-1");
  });

  it("posts approval requests by design ids", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "botanical",
            style_count: "3",
            shein_store_id: 7,
            created_at: "2026-06-01T10:00:00Z",
            updated_at: "2026-06-01T10:05:00Z",
          },
          items: [],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      approveSheinStudioBatchDesigns("batch-1", ["design-1", "design-2"]),
    ).resolves.toMatchObject({
      batch: { id: "batch-1" },
      items: [],
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/batches/batch-1/design-approvals",
    );
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ design_ids: ["design-1", "design-2"] }),
    });
  });

  it("starts itemized batch generation from the batch endpoint", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "botanical",
            style_count: "3",
            shein_store_id: 7,
            created_at: "2026-06-01T10:00:00Z",
            updated_at: "2026-06-01T10:06:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batch_id: "batch-1",
                target_group_key: "size:1200x1200",
                status: "review_ready",
                selection_count: 1,
                created_at: "2026-06-01T10:00:00Z",
                updated_at: "2026-06-01T10:06:00Z",
              },
              designs: [
                {
                  id: "design-1",
                  batch_id: "batch-1",
                  item_id: "item-1",
                  source_attempt_id: "attempt-1",
                  target_group_key: "size:1200x1200",
                  image_url: "https://cdn.example.com/design-1.png",
                  review_status: "approved",
                  created_at: "2026-06-01T10:01:00Z",
                  updated_at: "2026-06-01T10:06:00Z",
                },
              ],
            },
          ],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(generateSheinStudioBatch("batch-1")).resolves.toMatchObject({
      batch: { id: "batch-1", status: "review_ready" },
      items: [{ item: { id: "item-1" }, designs: [{ id: "design-1" }] }],
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/batches/batch-1/generate",
    );
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({}),
    });
  });

  it("retries failed batch items from the batch endpoint", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            status: "generating",
            prompt: "botanical",
            style_count: "3",
            shein_store_id: 7,
            created_at: "2026-06-01T10:00:00Z",
            updated_at: "2026-06-01T10:06:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batch_id: "batch-1",
                target_group_key: "size:1200x1200",
                status: "generating",
                selection_count: 1,
                created_at: "2026-06-01T10:00:00Z",
                updated_at: "2026-06-01T10:06:00Z",
              },
              designs: [],
            },
          ],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      retrySheinStudioBatchItems("batch-1", ["item-1", "item-2"]),
    ).resolves.toMatchObject({
      batch: { id: "batch-1", status: "generating" },
      items: [{ item: { id: "item-1", status: "generating" }, designs: [] }],
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/batches/batch-1/items/retry",
    );
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ item_ids: ["item-1", "item-2"] }),
    });
  });

  it("creates studio batch tasks from approved itemized designs", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            status: "tasks_created",
            prompt: "botanical",
            style_count: "3",
            shein_store_id: 7,
            created_at: "2026-06-01T10:00:00Z",
            updated_at: "2026-06-01T10:10:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batch_id: "batch-1",
                target_group_key: "size:1200x1200",
                status: "review_ready",
                selection_count: 1,
                created_at: "2026-06-01T10:00:00Z",
                updated_at: "2026-06-01T10:10:00Z",
              },
              designs: [
                {
                  id: "design-1",
                  batch_id: "batch-1",
                  item_id: "item-1",
                  source_attempt_id: "attempt-1",
                  target_group_key: "size:1200x1200",
                  image_url: "https://cdn.example.com/design-1.png",
                  review_status: "approved",
                  created_at: "2026-06-01T10:01:00Z",
                  updated_at: "2026-06-01T10:10:00Z",
                },
              ],
            },
          ],
          created_tasks: [
            {
              id: "task-1",
              title: "Task 1",
              design_id: "design-1",
            },
          ],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createSheinStudioBatchTasks("batch-1", ["design-1"]),
    ).resolves.toMatchObject({
      batch: { id: "batch-1", status: "tasks_created" },
      items: [{ item: { id: "item-1" }, designs: [{ id: "design-1" }] }],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/batches/batch-1/tasks",
    );
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ design_ids: ["design-1"] }),
    });
  });
});
