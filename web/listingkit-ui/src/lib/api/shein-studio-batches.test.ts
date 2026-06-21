import { afterEach, describe, expect, it, vi } from "vitest";

import {
  approveSheinStudioBatchDesigns,
  createSheinStudioBatchTasks,
  generateSheinStudioBatch,
  getSheinStudioBatchDetail,
  parseSheinStudioBatchDetailResponse,
  parseSheinStudioBatchTaskCreationResponse,
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
        status_groups: {
          items: [
            { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
            { key: "submission_failed", label: "提交失败", count: 1, ids: ["design-2"] },
          ],
          by_key: {
            submittable: { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
            submission_failed: {
              key: "submission_failed",
              label: "提交失败",
              count: 1,
              ids: ["design-2"],
            },
          },
        },
      }),
    ).toEqual({
      batch: {
        id: "batch-1",
        tenantId: undefined,
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
        draftUpdatedAt: undefined,
        updatedAt: "2026-06-01T10:05:00Z",
      },
      createdTasks: [],
      failedTasks: [],
      rejectedTasks: [],
      reusedTasks: [],
      statusGroups: {
        items: [
          { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
          { key: "submission_failed", label: "提交失败", count: 1, ids: ["design-2"] },
        ],
        byKey: {
          submittable: { key: "submittable", label: "可提交", count: 1, ids: ["item-1"] },
          submission_failed: {
            key: "submission_failed",
            label: "提交失败",
            count: 1,
            ids: ["design-2"],
          },
        },
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

  it("maps structured task outcomes from batch detail responses", () => {
    expect(
      parseSheinStudioBatchDetailResponse({
        batch: {
          id: "batch-1",
          status: "tasks_created",
          prompt: "botanical",
          style_count: "3",
          shein_store_id: 7,
          created_at: "2026-06-01T10:00:00Z",
          updated_at: "2026-06-01T10:05:00Z",
        },
        items: [],
        created_tasks: [
          {
            id: "task-1",
            title: "Task 1",
            design_id: "design-1",
            item_id: "item-1",
            selection_id: "selection-1",
            compatibility_fingerprint: "fp-1",
            status: "task_created",
            submission_state: "ready_to_submit",
            last_submission_action: "validated",
          },
        ],
        reused_tasks: [
          {
            id: "task-2",
            title: "Task 2",
            design_id: "design-2",
            item_id: "item-2",
            selection_id: "selection-2",
            compatibility_fingerprint: "fp-2",
            status: "draft_saved",
            submission_state: "draft_saved",
            last_submission_action: "save_draft",
          },
        ],
        rejected_tasks: [
          {
            design_id: "design-3",
            item_id: "item-3",
            selection_id: "selection-3",
            compatibility_fingerprint: "fp-3",
            status: "rejected",
            reason_code: "baseline_not_ready",
            message: "baseline 还没准备好",
          },
        ],
        failed_tasks: [
          {
            design_id: "design-4",
            item_id: "item-4",
            selection_id: "selection-4",
            compatibility_fingerprint: "fp-4",
            title: "Task 4",
            status: "failed",
            reason_code: "task_create_failed",
            message: "create timeout",
          },
        ],
      }),
    ).toMatchObject({
      createdTasks: [
        {
          id: "task-1",
          title: "Task 1",
          designId: "design-1",
          itemId: "item-1",
          selectionId: "selection-1",
          compatibilityFingerprint: "fp-1",
          status: "task_created",
          submissionState: "ready_to_submit",
          lastSubmissionAction: "validated",
          outcome: "created",
        },
      ],
      reusedTasks: [
        {
          id: "task-2",
          title: "Task 2",
          designId: "design-2",
          itemId: "item-2",
          selectionId: "selection-2",
          compatibilityFingerprint: "fp-2",
          status: "draft_saved",
          submissionState: "draft_saved",
          lastSubmissionAction: "save_draft",
          outcome: "reused",
        },
      ],
      rejectedTasks: [
        {
          designId: "design-3",
          itemId: "item-3",
          selectionId: "selection-3",
          compatibilityFingerprint: "fp-3",
          status: "rejected",
          reasonCode: "baseline_not_ready",
          message: "baseline 还没准备好",
          outcome: "rejected",
        },
      ],
      failedTasks: [
        {
          designId: "design-4",
          itemId: "item-4",
          selectionId: "selection-4",
          compatibilityFingerprint: "fp-4",
          title: "Task 4",
          status: "failed",
          reasonCode: "task_create_failed",
          message: "create timeout",
          outcome: "failed",
        },
      ],
    });
  });
});

describe("parseSheinStudioBatchTaskCreationResponse", () => {
  it("does not collapse a rejected-only task result into a successful creation", () => {
    expect(
      parseSheinStudioBatchTaskCreationResponse({
        batch: {
          id: "batch-1",
          status: "tasks_created",
          prompt: "botanical",
          style_count: "3",
          shein_store_id: 7,
          created_at: "2026-06-01T10:00:00Z",
          updated_at: "2026-06-01T10:05:00Z",
        },
        items: [],
        created_tasks: [],
        reused_tasks: [],
        rejected_tasks: [
          {
            design_id: "design-1",
            item_id: "item-1",
            selection_id: "selection-1",
            compatibility_fingerprint: "fp-1",
            reason_code: "baseline_not_ready",
            message: "baseline 还没准备好",
          },
        ],
      }),
    ).toMatchObject({
      createdTasks: [],
      reusedTasks: [],
      rejectedTasks: [
        {
          designId: "design-1",
          itemId: "item-1",
          selectionId: "selection-1",
          compatibilityFingerprint: "fp-1",
          reasonCode: "baseline_not_ready",
          message: "baseline 还没准备好",
          outcome: "rejected",
        },
      ],
      failedTasks: [],
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

  it("includes tenant id when loading a scoped batch detail", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            tenant_id: "tenant-9",
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
      getSheinStudioBatchDetail("batch-1", { tenantId: "tenant-9" }),
    ).resolves.toMatchObject({
      batch: { id: "batch-1", tenantId: "tenant-9" },
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/batches/batch-1?tenant_id=tenant-9",
    );
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

  it("includes tenant id when creating scoped batch tasks", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            tenant_id: "tenant-9",
            status: "tasks_creating",
            prompt: "botanical",
            style_count: "3",
            shein_store_id: 7,
            created_at: "2026-06-01T10:00:00Z",
            updated_at: "2026-06-01T10:10:00Z",
          },
          items: [],
          created_tasks: [],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await createSheinStudioBatchTasks("batch-1", ["design-1"], {
      tenantId: "tenant-9",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/studio/batches/batch-1/tasks?tenant_id=tenant-9",
    );
  });

  it("can explicitly allow partial task creation while a batch is still generating", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          batch: {
            id: "batch-1",
            status: "tasks_creating",
            prompt: "botanical",
            style_count: "3",
            shein_store_id: 7,
            created_at: "2026-06-01T10:00:00Z",
            updated_at: "2026-06-01T10:10:00Z",
          },
          items: [],
          created_tasks: [],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await createSheinStudioBatchTasks("batch-1", ["design-1"], {
      allowPartialWhileGenerating: true,
    });

    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({
        design_ids: ["design-1"],
        allow_partial_while_generating: true,
      }),
    });
  });
});
