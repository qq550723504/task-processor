import { describe, expect, it } from "vitest";

import {
  buildSheinStudioGenerateRequest,
  buildSheinStudioSelectedVariants,
  getItemizedBatchPendingTaskDesignIDs,
  getSheinStudioCreateActionDisabledReason,
  hasInFlightItemizedBatchGeneration,
  mergeSheinStudioDraftState,
  pickActiveSheinStudioGroup,
  projectGroupToWorkbench,
  projectHydratedBatchToWorkbench,
  projectDefaultSelectedSDSImages,
  projectSavedBatchToWorkbench,
  projectWorkbenchTraceContext,
  resolveCurrentSheinStudioSavedBatch,
  projectWorkbenchStateToSavedBatch,
  sheinStudioBusyMessage,
  summarizeSheinStudioSelection,
  toggleSelectedDesignId,
  toggleItemizedBatchDesignApproval,
  updateFlatDesignReviewNote,
  updateItemizedBatchDesignReviewNote,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";

describe("shein studio workbench model", () => {
  it("summarizes explicit SDS variants before falling back to selected ids", () => {
    const summary = summarizeSheinStudioSelection({
      productId: 1,
      parentProductId: 1,
      variantId: 10,
      variants: [
        { variantId: 10, color: "Black", size: "M" },
        { variantId: 11, color: "White", size: "L" },
      ],
      selectedVariantIds: [10, 11, 12],
      prototypeGroupId: 2,
      layerId: "layer",
      productName: "tee",
      variantLabel: "M / Black",
      printableWidth: 1200,
      printableHeight: 1600,
    });

    expect(summary.printableAreaLabel).toBe("1200 × 1600px");
    expect(summary.selectedVariants).toHaveLength(2);
    expect(summary.selectedColorCount).toBe(2);
    expect(summary.selectedSizeCount).toBe(2);
  });

  it("builds a single fallback variant from the selected SDS variant", () => {
    expect(
      buildSheinStudioSelectedVariants({
        productId: 1,
        parentProductId: 1,
        variantId: 10,
        prototypeGroupId: 2,
        layerId: "layer",
        productName: "tee",
        variantLabel: "M / Black",
      }),
    ).toEqual([{ variantId: 10, size: "M / Black", color: "默认" }]);
  });

  it("merges a gallery handoff into an existing draft once", () => {
    const state = mergeSheinStudioDraftState({
      draft: {
        prompt: "saved prompt",
        styleCount: "2",
        variationIntensity: "medium",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        sheinStoreId: "869",
        imageStrategy: "hybrid",
        groups: [],
        selectedSdsImages: [{ imageUrl: "https://example.com/sds.jpg" }],
        renderSizeImagesWithSds: true,
        designs: [{ id: "design-1", imageUrl: "https://example.com/a.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-05-10T00:00:00.000Z",
      },
      galleryDesign: {
        id: "gallery-1",
        imageUrl: "https://example.com/gallery.png",
      },
      galleryPrompt: "gallery prompt",
    });

    expect(state.prompt).toBe("saved prompt");
    expect(state.designs.map((item) => item.id)).toEqual(["design-1", "gallery-1"]);
    expect(state.selectedIds).toEqual(["design-1", "gallery-1"]);
    expect(state.hasCustomizedSdsSelection).toBe(true);
    expect(state.importedGalleryDesign).toBe(true);
  });

  it("picks the most recently updated group when no explicit active group is provided", () => {
    const group = pickActiveSheinStudioGroup([
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
        sheinStoreId: "42",
        imageStrategy: "hybrid",
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
      {
        id: "group-2",
        name: "Group 2",
        currentPrompt: "prompt b",
        promptHistory: [],
        primarySelection: {
          productId: 1,
          parentProductId: 1,
          variantId: 101,
          prototypeGroupId: 200,
          layerId: "layer-2",
          productName: "hoodie",
          variantLabel: "L / white",
        },
        groupedSelections: [],
        sheinStoreId: "88",
        imageStrategy: "sds_official",
        groupedImageMode: "per_product",
        selectedSdsImages: [],
        renderSizeImagesWithSds: false,
        productImageCount: "3",
        productImagePrompt: "product prompt",
        productImagePrompts: [],
        artworkModel: "",
        transparentBackground: true,
        variationIntensity: "strong",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T01:00:00Z",
      },
    ]);

    expect(group?.id).toBe("group-2");
  });

  it("projects a group into workbench editor fields", () => {
    const projection = projectGroupToWorkbench({
      id: "group-2",
      name: "Group 2",
      currentPrompt: "prompt b",
      promptHistory: [],
      primarySelection: {
        productId: 1,
        parentProductId: 1,
        variantId: 101,
        prototypeGroupId: 200,
        layerId: "layer-2",
        productName: "hoodie",
        variantLabel: "L / white",
      },
      groupedSelections: [],
      sheinStoreId: "88",
      imageStrategy: "sds_official",
      groupedImageMode: "per_product",
      selectedSdsImages: [],
      renderSizeImagesWithSds: false,
      productImageCount: "3",
      productImagePrompt: "product prompt",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: true,
      variationIntensity: "strong",
      designs: [{ id: "design-2", imageUrl: "https://example.com/2.png" }],
      selectedIds: ["design-2"],
      createdTasks: [],
      updatedAt: "2026-05-26T01:00:00Z",
    });

    expect(projection.prompt).toBe("prompt b");
    expect(projection.groupedImageMode).toBe("per_product");
    expect(projection.productImageCount).toBe("3");
    expect(projection.selectedIds).toEqual(["design-2"]);
  });

  it("treats itemized batch detail as the primary workbench source", () => {
    const projection = projectHydratedBatchToWorkbench({
      savedBatch: {
        id: "batch-1",
        name: "Saved Batch",
        prompt: "stale saved prompt",
        styleCount: "9",
        variationIntensity: "light",
        productImageCount: "5",
        artworkModel: "legacy-model",
        transparentBackground: false,
        sheinStoreId: "42",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-06-01T10:00:00Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "itemized prompt",
          styleCount: "2",
          sheinStoreId: 869,
          variationIntensity: "strong",
          artworkModel: "nanobanana",
          transparentBackground: true,
          groupedImageMode: "per_product",
          selectedSdsImages: [{ imageUrl: "https://example.com/sds.jpg" }],
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          groupedSelections: [],
          createdAt: "2026-06-01T10:00:00Z",
          updatedAt: "2026-06-01T10:10:00Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "group-1",
              status: "review_ready",
              selectionCount: 1,
              createdAt: "2026-06-01T10:00:00Z",
              updatedAt: "2026-06-01T10:10:00Z",
            },
            designs: [
              {
                id: "design-1",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-1",
                targetGroupKey: "group-1",
                imageUrl: "https://example.com/design-1.png",
                reviewStatus: "approved",
                createdAt: "2026-06-01T10:00:00Z",
                updatedAt: "2026-06-01T10:10:00Z",
              },
            ],
          },
        ],
      },
    });

    expect(projection.prompt).toBe("itemized prompt");
    expect(projection.styleCount).toBe("2");
    expect(projection.variationIntensity).toBe("strong");
    expect(projection.artworkModel).toBe("nanobanana");
    expect(projection.transparentBackground).toBe(true);
    expect(projection.sheinStoreId).toBe("869");
    expect(projection.groupedImageMode).toBe("per_product");
    expect(projection.selection?.variantId).toBe(101);
    expect(projection.designs.map((item) => item.id)).toEqual(["design-1"]);
    expect(projection.selectedIds).toEqual(["design-1"]);
    expect(projection.itemizedBatchDetail?.batch.id).toBe("batch-1");
  });

  it("detects approved itemized designs that still need SHEIN task creation", () => {
    const pendingDesignIds = getItemizedBatchPendingTaskDesignIDs({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "itemized prompt",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:10:00Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "group-1",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:10:00Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "group-1",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-06-01T10:00:00Z",
              updatedAt: "2026-06-01T10:10:00Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "group-1",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-06-01T10:01:00Z",
              updatedAt: "2026-06-01T10:10:00Z",
            },
          ],
        },
      ],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
    });

    expect(pendingDesignIds).toEqual(["design-2"]);
  });

  it("toggles approval for one itemized batch design", () => {
    const detail = {
      batch: {
        id: "batch-1",
        status: "review_ready" as const,
        prompt: "itemized prompt",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:10:00Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "group-1",
            status: "review_ready" as const,
            selectionCount: 1,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:10:00Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "group-1",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved" as const,
              createdAt: "2026-06-01T10:00:00Z",
              updatedAt: "2026-06-01T10:10:00Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "group-1",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "unreviewed" as const,
              createdAt: "2026-06-01T10:01:00Z",
              updatedAt: "2026-06-01T10:10:00Z",
            },
          ],
        },
      ],
    };

    const next = toggleItemizedBatchDesignApproval(detail, "design-1");

    expect(next.items[0]?.designs.map((design) => design.reviewStatus)).toEqual([
      "unreviewed",
      "unreviewed",
    ]);
    expect(detail.items[0]?.designs[0]?.reviewStatus).toBe("approved");
  });

  it("updates review notes for itemized and flat designs", () => {
    const detail = {
      batch: {
        id: "batch-1",
        status: "review_ready" as const,
        prompt: "itemized prompt",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:10:00Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "group-1",
            status: "review_ready" as const,
            selectionCount: 1,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:10:00Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "group-1",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved" as const,
              createdAt: "2026-06-01T10:00:00Z",
              updatedAt: "2026-06-01T10:10:00Z",
            },
          ],
        },
      ],
    };

    expect(
      updateItemizedBatchDesignReviewNote(detail, "design-1", "needs crop").items[0]
        ?.designs[0]?.reviewNote,
    ).toBe("needs crop");
    expect(
      updateFlatDesignReviewNote(
        [
          { id: "design-1", imageUrl: "https://example.com/1.png" },
          { id: "design-2", imageUrl: "https://example.com/2.png", reviewNote: "keep" },
        ],
        "design-1",
        "needs crop",
      ),
    ).toEqual([
      {
        id: "design-1",
        imageUrl: "https://example.com/1.png",
        reviewNote: "needs crop",
      },
      { id: "design-2", imageUrl: "https://example.com/2.png", reviewNote: "keep" },
    ]);
  });

  it("toggles selected design ids", () => {
    expect(toggleSelectedDesignId(["design-1", "design-2"], "design-1")).toEqual([
      "design-2",
    ]);
    expect(toggleSelectedDesignId(["design-1"], "design-2")).toEqual([
      "design-1",
      "design-2",
    ]);
  });

  it("projects saved-batch compatibility snapshots before hydration", () => {
    const projection = projectSavedBatchToWorkbench({
      id: "batch-1",
      name: "Saved Batch",
      prompt: "saved prompt",
      styleCount: "2",
      sheinStoreId: "42",
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationJobs: [],
      legacyCompatibilitySnapshot: {
        designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
        selectedIds: ["design-1"],
        createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
        generationJobs: [{ jobId: "job-1", status: "running" }],
        generationError: "legacy-error",
      },
      updatedAt: "2026-06-01T10:00:00Z",
    });

    expect(projection.designs).toEqual([
      { id: "design-1", imageUrl: "https://example.com/design.png" },
    ]);
    expect(projection.selectedIds).toEqual(["design-1"]);
    expect(projection.createdTasks).toEqual([
      { id: "task-1", title: "Task 1", designId: "design-1" },
    ]);
    expect(projection.generationJobs).toEqual([{ jobId: "job-1", status: "running" }]);
    expect(projection.generationError).toBe("legacy-error");
  });

  it("preserves a newer saved-batch prompt over stale itemized detail prompt", () => {
    const projection = projectHydratedBatchToWorkbench({
      savedBatch: {
        id: "batch-1",
        name: "Saved Batch",
        prompt: "updated prompt",
        styleCount: "1",
        sheinStoreId: "869",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        draftUpdatedAt: "2026-06-01T10:10:00Z",
        updatedAt: "2026-06-01T10:10:00Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "draft",
          prompt: "stale itemized prompt",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-06-01T10:00:00Z",
          updatedAt: "2026-06-01T10:05:00Z",
        },
        items: [],
      },
    });

    expect(projection.prompt).toBe("updated prompt");
  });

  it("clears saved generation fallback once hydrated detail is no longer in flight", () => {
    const projection = projectHydratedBatchToWorkbench({
      savedBatch: {
        id: "batch-1",
        name: "Saved Batch",
        prompt: "saved prompt",
        styleCount: "1",
        sheinStoreId: "869",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        generationJobs: [{ jobId: "job-1", status: "running" }],
        generationError: "legacy-error",
        updatedAt: "2026-06-01T10:10:00Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "hydrated prompt",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-06-01T10:00:00Z",
          updatedAt: "2026-06-01T10:12:00Z",
        },
        items: [],
      },
    });

    expect(projection.generationJobs).toEqual([]);
    expect(projection.generationError).toBe("");
  });

  it("preserves hydrated failure errors when detail lands in a failed state", () => {
    const projection = projectHydratedBatchToWorkbench({
      savedBatch: {
        id: "batch-1",
        name: "Saved Batch",
        prompt: "saved prompt",
        styleCount: "1",
        sheinStoreId: "869",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        generationJobs: [{ jobId: "job-1", status: "failed" }],
        generationError: "generation failed",
        updatedAt: "2026-06-01T10:10:00Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "failed",
          prompt: "hydrated prompt",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-06-01T10:00:00Z",
          updatedAt: "2026-06-01T10:12:00Z",
        },
        items: [],
      },
    });

    expect(projection.generationJobs).toEqual([]);
    expect(projection.generationError).toBe("generation failed");
  });

  it("does not resurrect stale compatibility errors after they have been cleared", () => {
    const projection = projectSavedBatchToWorkbench({
      id: "batch-1",
      name: "Saved Batch",
      prompt: "saved prompt",
      styleCount: "2",
      sheinStoreId: "42",
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationJobs: [],
      generationError: "",
      legacyCompatibilitySnapshot: {
        generationError: "old failure",
      },
      updatedAt: "2026-06-01T10:00:00Z",
    });

    expect(projection.generationError).toBe("");
  });

  it("projects the current workbench state into a saved-batch fallback shape", () => {
    const projection = projectWorkbenchStateToSavedBatch({
      id: "batch-1",
      prompt: "current prompt",
      styleCount: "2",
      variationIntensity: "strong",
      productImageCount: "6",
      productImagePrompt: "hero shot",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: true,
      sheinStoreId: "869",
      imageStrategy: "ai_generated",
      groupedImageMode: "per_product",
      selectedSdsImages: [{ imageUrl: "https://example.com/sds.jpg" }],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 101,
        prototypeGroupId: 200,
        layerId: "layer-2",
        productName: "hoodie",
        variantLabel: "L / white",
      },
      groupedSelections: [],
      groups: [],
      designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
      selectedIds: ["design-1"],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
      generationJobs: [{ jobId: "job-1", status: "running" }],
      updatedAt: "2026-06-01T10:10:00Z",
    });

    expect(projection).toMatchObject({
      id: "batch-1",
      name: "",
      prompt: "current prompt",
      selectedIds: ["design-1"],
      updatedAt: "2026-06-01T10:10:00Z",
    });
  });

  it("resolves the current saved batch before falling back to workbench state", () => {
    const savedBatch = {
      id: "batch-1",
      name: "Saved Batch",
      prompt: "saved prompt",
      styleCount: "1",
      sheinStoreId: "869",
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-06-01T10:00:00Z",
    };
    const fallback = {
      prompt: "current prompt",
      styleCount: "2",
      variationIntensity: "strong" as const,
      productImageCount: "6",
      productImagePrompt: "hero shot",
      productImagePrompts: [],
      artworkModel: "nanobanana" as const,
      transparentBackground: true,
      sheinStoreId: "869",
      imageStrategy: "ai_generated" as const,
      groupedImageMode: "per_product" as const,
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: undefined,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationJobs: [],
      updatedAt: "2026-06-01T10:10:00Z",
    };

    expect(
      resolveCurrentSheinStudioSavedBatch({
        activeBatchId: "batch-1",
        fallback,
        initialBatchId: "",
        savedBatches: [savedBatch],
      }),
    ).toBe(savedBatch);
    expect(
      resolveCurrentSheinStudioSavedBatch({
        activeBatchId: "",
        fallback,
        initialBatchId: "batch-2",
        savedBatches: [savedBatch],
      }),
    ).toMatchObject({
      id: "batch-2",
      prompt: "current prompt",
      styleCount: "2",
    });
  });

  it("projects default SDS image selection for hybrid generation", () => {
    expect(
      projectDefaultSelectedSDSImages({
        availableSdsImages: [
          {
            imageUrl: "https://img.ltwebstatic.com/images3_spmp/2026/01/01/mockup.jpg",
            kind: "mockup",
            label: "Mockup",
          },
          {
            imageUrl: "https://example.com/size-reference.jpg",
            kind: "size_reference",
            label: "Size",
          },
        ],
        currentSelectedSdsImages: [],
        hasCustomizedSdsSelection: false,
        imageStrategy: "hybrid",
        renderSizeImagesWithSds: true,
      }),
    ).toEqual([
      {
        imageUrl: "https://img.ltwebstatic.com/images3_spmp/2026/01/01/mockup.jpg",
        color: undefined,
        variantSku: undefined,
      },
      {
        imageUrl: "https://example.com/size-reference.jpg",
        color: undefined,
        variantSku: undefined,
      },
    ]);
  });

  it("does not replace customized SDS image selection", () => {
    expect(
      projectDefaultSelectedSDSImages({
        availableSdsImages: [
          {
            imageUrl: "https://img.ltwebstatic.com/images3_spmp/2026/01/01/mockup.jpg",
            kind: "mockup",
            label: "Mockup",
          },
        ],
        currentSelectedSdsImages: [
          { imageUrl: "https://example.com/custom.jpg" },
        ],
        hasCustomizedSdsSelection: true,
        imageStrategy: "hybrid",
        renderSizeImagesWithSds: true,
      }),
    ).toBeNull();
  });

  it("projects trace context with one-based queue positions", () => {
    expect(
      projectWorkbenchTraceContext({
        batchQueueMode: "generate",
        queuedBatchIds: ["batch-1", "batch-2"],
        queuedBatchIndex: 1,
        traceBatchId: "batch-2",
      }),
    ).toEqual({
      batchId: "batch-2",
      queueMode: "generate",
      queueIndex: 2,
      queueTotal: 2,
    });
  });

  it("omits queue trace fields outside queue mode", () => {
    expect(
      projectWorkbenchTraceContext({
        batchQueueMode: null,
        queuedBatchIds: ["batch-1"],
        queuedBatchIndex: 0,
        traceBatchId: "",
      }),
    ).toEqual({
      batchId: undefined,
      queueMode: undefined,
      queueIndex: undefined,
      queueTotal: undefined,
    });
  });

  it("returns the create-task disabled reason from the first blocking condition", () => {
    expect(
      getSheinStudioCreateActionDisabledReason({
        selectedIds: ["design-1"],
      }),
    ).toBe("请先选择 SDS 商品变体。生成 SHEIN 资料前需要锁定商品模板。");

    expect(
      getSheinStudioCreateActionDisabledReason({
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 10,
          prototypeGroupId: 2,
          layerId: "layer",
          productName: "tee",
          variantLabel: "M / Black",
        },
        galleryRatioCheck: { status: "blocking", message: "比例不匹配" },
        selectedIds: ["design-1"],
      }),
    ).toBe("比例不匹配");
  });

  it("builds generation requests with transparent-background model override", () => {
    expect(
      buildSheinStudioGenerateRequest({
        artworkModel: "nanobanana",
        prompt: " retro cherries ",
        printableWidth: 1000,
        printableHeight: 1200,
        productReferenceImageUrls: ["https://example.com/reference.jpg"],
        styleCount: 2,
        transparentBackground: true,
        variationIntensity: "strong",
      }),
    ).toEqual({
      prompt: "retro cherries\n\nprintable size: 1000x1200px.",
      count: 2,
      variationIntensity: "strong",
      printableWidth: 1000,
      printableHeight: 1200,
      productReferenceImageUrls: ["https://example.com/reference.jpg"],
      imageModel: "gpt-image-2",
      transparentBackground: true,
    });
  });

  it("leaves image model empty so backend default can apply", () => {
    expect(
      buildSheinStudioGenerateRequest({
        artworkModel: "   ",
        prompt: "retro cherries",
        styleCount: 1,
        transparentBackground: false,
        variationIntensity: "medium",
      }),
    ).toEqual({
      prompt: "retro cherries",
      count: 1,
      variationIntensity: "medium",
      printableWidth: undefined,
      printableHeight: undefined,
      productReferenceImageUrls: undefined,
      imageModel: undefined,
      transparentBackground: false,
    });
  });

  it("does not append the SDS size twice when the prompt already contains it", () => {
    expect(
      buildSheinStudioGenerateRequest({
        artworkModel: "nanobanana",
        prompt: "retro cherries printable size: 1000x1200px.",
        printableWidth: 1000,
        printableHeight: 1200,
        styleCount: 1,
        transparentBackground: false,
        variationIntensity: "medium",
      }),
    ).toEqual({
      prompt: "retro cherries printable size: 1000x1200px.",
      count: 1,
      variationIntensity: "medium",
      printableWidth: 1000,
      printableHeight: 1200,
      productReferenceImageUrls: undefined,
      imageModel: "nanobanana",
      transparentBackground: false,
    });
  });

  it("prioritizes busy messages by active operation", () => {
    expect(
      sheinStudioBusyMessage({
        isCreatingTasks: true,
        isGenerating: true,
        regeneratingId: "design-1",
      }),
    ).toBe("正在生成款式图");
    expect(
      sheinStudioBusyMessage({
        isCreatingTasks: true,
        isGenerating: false,
        regeneratingId: "design-1",
      }),
    ).toBe("正在重新生成图片");
  });

  it("treats partially failed batches without active items as not in flight", () => {
    expect(
      hasInFlightItemizedBatchGeneration({
        batch: {
          id: "batch-1",
          status: "partially_failed",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T10:00:00Z",
          updatedAt: "2026-05-26T10:05:00Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "size:1000x1000",
              status: "failed",
              selectionCount: 1,
              createdAt: "2026-05-26T10:00:00Z",
              updatedAt: "2026-05-26T10:05:00Z",
            },
            designs: [],
          },
        ],
      }),
    ).toBe(false);
  });

  it("treats partially failed batches with pending items as still in flight", () => {
    expect(
      hasInFlightItemizedBatchGeneration({
        batch: {
          id: "batch-1",
          status: "partially_failed",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T10:00:00Z",
          updatedAt: "2026-05-26T10:05:00Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "size:1000x1000",
              status: "pending",
              selectionCount: 1,
              createdAt: "2026-05-26T10:00:00Z",
              updatedAt: "2026-05-26T10:05:00Z",
            },
            designs: [],
          },
        ],
      }),
    ).toBe(true);
  });
});
