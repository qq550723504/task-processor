import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  approveSheinStudioBatchDesigns,
  analyzeSheinStudioReferenceStyle,
  buildDraft,
  buildHydratedBatch,
  buildReviewReadyHydratedBatchDetail,
  createDeferred,
  createSheinReviewTasks,
  createSheinStudioBatchTasks,
  deleteSheinStudioBatch,
  generateSheinStudioBatch,
  generateSheinStudioDesigns,
  getSDSBaselineReadiness,
  getSheinStudioHydratedBatch,
  groupedSelection,
  hydrateSDSVariantSelection,
  lastGenerationPanelProps,
  listSheinStudioBatches,
  loadSheinStudioDraft,
  push,
  resetSheinStudioWorkbenchHarness,
  retrySheinStudioBatchItems,
  saveSheinStudioBatch,
  saveSheinStudioDraftWithOptions,
  selection,
  setActiveSheinStudioBatchId,
  startSheinStudioBatchRun,
  useQuery,
  warmSDSBaselineForSelection,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-test-harness";
import {
  resetDedicatedBatchPromptOverrides,
  SheinStudioWorkbench,
} from "@/components/listingkit/shein-studio/shein-studio-workbench";
import {
  loadLocalSheinStudioDraftSnapshotDetail,
  saveLocalSheinStudioDraftSnapshot,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import { saveSheinStudioGalleryHandoff } from "@/lib/shein-studio/gallery-handoff";

describe("SheinStudioWorkbench", () => {
  beforeEach(() => {
    resetSheinStudioWorkbenchHarness(resetDedicatedBatchPromptOverrides);
  });

  it("defaults to one SDS main image plus size references in hybrid and SDS modes", async () => {
    hydrateSDSVariantSelection.mockResolvedValue({
      ...selection,
      mockupImageUrls: ["https://example.com/main-mockup.jpg"],
      sizeReferenceImageUrls: ["https://example.com/size-reference.jpg"],
    });
    loadSheinStudioDraft.mockResolvedValue({
      prompt: "retro cherries",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "hybrid",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-29T00:00:00.000Z",
    });

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByText("selected SDS images: 2")).toBeInTheDocument(),
    );
    expect(lastGenerationPanelProps?.selectedSdsImages).toEqual([
      {
        imageUrl: "https://example.com/main-mockup.jpg",
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

  it("loads saved groups on page entry without requiring reselecting the original product", async () => {
    saveLocalSheinStudioDraftSnapshot({
      prompt: "legacy top-level",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection: selection,
          groupedSelections: [],
          styleCount: "1",
          sheinStoreId: "1",
          imageStrategy: "ai_generated",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt a",
          promptHistory: [],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "nanobanana",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T00:00:00.000Z",
        },
        {
          id: "group-2",
          name: "Group 2",
          primarySelection: {
            ...selection,
            layerId: "layer-3",
            productName: "hoodie",
            variantId: 102,
            variantLabel: "L / white",
          },
          groupedSelections: [groupedSelection],
          styleCount: "2",
          sheinStoreId: "9",
          imageStrategy: "sds_official",
          groupedImageMode: "per_product",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt b",
          promptHistory: [],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "nanobanana",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T01:00:00.000Z",
        },
      ],
      updatedAt: "2026-05-26T01:00:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    expect(await screen.findByText("Group 2")).toBeInTheDocument();
    expect(screen.getByDisplayValue("prompt b")).toBeInTheDocument();
  });

  it("shows recent batch cards before any explicit product reselection", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        groupedSelections: [
          {
            selectionId: "sel-1",
            selection: groupedSelection.selection,
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "869",
            eligible: true,
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    expect(await screen.findByText("最近批次")).toBeInTheDocument();
    expect(screen.getByText("Retro Cherries")).toBeInTheDocument();
    expect(screen.getByText("2 款商品")).toBeInTheDocument();
  });

  it("loads a batch by id when mounted from the dedicated batch route", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [],
      },
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-1"),
    );
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    expect(screen.getByText("入口商品")).toBeInTheDocument();
    expect(screen.queryByText("入口商品状态")).not.toBeInTheDocument();
    expect(screen.getByLabelText("批次店铺")).toBeInTheDocument();
    expect(screen.getByText("saved batches visible: no")).toBeInTheDocument();
    expect(screen.queryByText("最近批次")).not.toBeInTheDocument();
  });

  it("shows itemized dedicated-batch designs in review even when the saved batch shell is empty", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "size:1000x1000",
              status: "review_ready",
              selectionCount: 1,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
            designs: [
              {
                id: "design-1",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-1",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-1.png",
                reviewStatus: "approved",
                createdAt: "2026-05-26T09:59:30.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
              {
                id: "design-2",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-2",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-2.png",
                reviewStatus: "unreviewed",
                createdAt: "2026-05-26T09:59:40.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
            ],
          },
        ],
      },
    });

    render(
      <SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("review grid: 2")).toBeInTheDocument(),
    );
    expect(screen.getByText("approved styles: 1")).toBeInTheDocument();
  });

  it("does not let a newer dedicated-batch local snapshot override hydrated detail results", async () => {
    saveLocalSheinStudioDraftSnapshot(
      {
        prompt: "stale local draft",
        styleCount: "1",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        sheinStoreId: "869",
        imageStrategy: "ai_generated",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        designs: [
          { id: "legacy-design", imageUrl: "https://example.com/legacy.png" },
        ],
        selectedIds: ["legacy-design"],
        createdTasks: [],
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      { batchId: "batch-1" },
    );
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          id: "batch-1",
          name: "Hydrated Batch",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:01:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:00:30.000Z",
                  updatedAt: "2026-05-26T10:01:00.000Z",
                },
                {
                  id: "design-2",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-2",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-2.png",
                  reviewStatus: "unreviewed",
                  createdAt: "2026-05-26T10:00:40.000Z",
                  updatedAt: "2026-05-26T10:01:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );

    render(
      <SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("review grid: 2")).toBeInTheDocument(),
    );
    expect(screen.getByText("approved styles: 1")).toBeInTheDocument();
    expect(screen.queryByText("review grid: 1")).not.toBeInTheDocument();
  });

  it("keeps dedicated batch task creation on the itemized batch endpoint after local review edits", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "review_ready",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              targetGroupKey: "size:1000x1000",
              status: "review_ready",
              selectionCount: 1,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
            designs: [
              {
                id: "design-1",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-1",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-1.png",
                reviewStatus: "approved",
                createdAt: "2026-05-26T09:59:30.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
              {
                id: "design-2",
                batchId: "batch-1",
                itemId: "item-1",
                sourceAttemptId: "attempt-2",
                targetGroupKey: "size:1000x1000",
                imageUrl: "https://example.com/design-2.png",
                reviewStatus: "unreviewed",
                createdAt: "2026-05-26T09:59:31.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
            ],
          },
        ],
      },
    });
    createSheinStudioBatchTasks.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [
        { id: "task-1", title: "Task 1", designId: "design-1" },
        { id: "task-2", title: "Task 2", designId: "design-2" },
      ],
    });
    approveSheinStudioBatchDesigns.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:05:30.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:30.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:30.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:05:30.000Z",
            },
          ],
        },
      ],
    });
    approveSheinStudioBatchDesigns.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:01:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:01:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T09:59:30.000Z",
              updatedAt: "2026-05-26T10:01:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T09:59:31.000Z",
              updatedAt: "2026-05-26T10:01:00.000Z",
            },
          ],
        },
      ],
    });

    render(
      <SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("approved styles: 1")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "toggle-design-2" }));
    fireEvent.click(screen.getByRole("button", { name: "note-design-1" }));

    await waitFor(() =>
      expect(screen.getByText("approved styles: 2")).toBeInTheDocument(),
    );
    expect(approveSheinStudioBatchDesigns).toHaveBeenCalledWith("batch-1", [
      "design-1",
      "design-2",
    ]);
    fireEvent.click(
      screen.getByRole("button", { name: "create review tasks" }),
    );

    await waitFor(() =>
      expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", [
        "design-1",
        "design-2",
      ]),
    );
    expect(createSheinReviewTasks).not.toHaveBeenCalled();
  });

  it("generates an active homepage batch through the itemized batch endpoint instead of session append", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      }),
    );
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries updated",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries updated",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Retro Cherries/ }),
    );
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          prompt: "retro cherries updated",
        }),
        { makeActive: false },
      ),
    );
    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
    expect(generateSheinStudioDesigns).not.toHaveBeenCalled();
  });

  it("starts a backend batch run from a saved homepage batch", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockResolvedValue({
      savedBatch: {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      detail: {
        batch: {
          id: "batch-1",
          status: "draft",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T09:59:00.000Z",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        items: [],
      },
    });
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:05:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "unreviewed",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:05:00.000Z",
            },
          ],
        },
      ],
    });
    createSheinStudioBatchTasks.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:00.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:04:30.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [
        { id: "task-1", title: "Task 1", designId: "design-1" },
        { id: "task-2", title: "Task 2", designId: "design-2" },
      ],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Retro Cherries/ }),
    );
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
    expect(generateSheinStudioDesigns).not.toHaveBeenCalled();
  });

  it("does not reload the dedicated batch when editing the prompt", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        name: "Retro Cherries",
        prompt: "",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    const promptInput = await screen.findByLabelText("prompt");
    const callCountBeforeEdit = getSheinStudioHydratedBatch.mock.calls.length;
    fireEvent.change(promptInput, {
      target: { value: "vintage botanical clock" },
    });

    await waitFor(() =>
      expect(
        screen.getByDisplayValue("vintage botanical clock"),
      ).toBeInTheDocument(),
    );
    await waitFor(() =>
      expect(loadLocalSheinStudioDraftSnapshotDetail()).toMatchObject({
        batchId: "batch-1",
        draft: expect.objectContaining({
          prompt: "vintage botanical clock",
        }),
      }),
    );
    expect(getSheinStudioHydratedBatch).toHaveBeenCalledTimes(
      callCountBeforeEdit,
    );
  });

  it("surfaces dedicated batch hydration failures instead of silently leaving an empty shell", async () => {
    getSheinStudioHydratedBatch.mockRejectedValue(
      new Error("Missing ZITADEL session"),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "当前批次加载失败：Missing ZITADEL session。请重新登录后再继续。",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("keeps dedicated batch prompt edits in local draft without remote autosave", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        prompt: "",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    const promptInput = await screen.findByLabelText("prompt");
    fireEvent.change(promptInput, { target: { value: "updated prompt" } });

    await waitFor(
      () =>
        expect(loadLocalSheinStudioDraftSnapshotDetail()).toMatchObject({
          batchId: "batch-1",
          draft: expect.objectContaining({
            prompt: "updated prompt",
          }),
        }),
      { timeout: 3000 },
    );
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("refreshes the dedicated batch version immediately before generate", async () => {
    getSheinStudioHydratedBatch
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            draftUpdatedAt: "2026-05-26T10:00:00.000Z",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              draftUpdatedAt: "2026-05-26T10:06:00.000Z",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            draftUpdatedAt: "2026-05-26T10:00:00.000Z",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              draftUpdatedAt: "2026-05-26T10:07:00.000Z",
              updatedAt: "2026-05-26T10:08:00.000Z",
            },
          },
        ),
      );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      draftUpdatedAt: "2026-05-26T10:07:00.000Z",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "review_ready",
        updatedAt: "2026-05-26T10:07:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:07:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:06:30.000Z",
              updatedAt: "2026-05-26T10:07:00.000Z",
            },
          ],
        },
      ],
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          updatedAt: "2026-05-26T10:07:00.000Z",
        }),
        { makeActive: false },
      ),
    );
    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("starts a backend batch run when generate is clicked on a partially failed batch", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          selection: undefined,
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "partially_failed",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "failed",
                selectionCount: 1,
                lastError: "excessive system load",
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [],
            },
          ],
        },
      ),
    );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    retrySheinStudioBatchItems.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "generating",
        updatedAt: "2026-05-26T10:07:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "generating",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:07:00.000Z",
          },
          designs: [],
        },
      ],
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    expect(
      screen.getByRole("button", { name: "重试失败批次" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/当前批次有 1 个失败项/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试失败批次" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(retrySheinStudioBatchItems).not.toHaveBeenCalled();
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("routes completed itemized task creation results back into the task list", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "review_ready",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:01:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );
    createSheinStudioBatchTasks.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "tasks_created",
        updatedAt: "2026-05-26T10:08:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:08:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:01:00.000Z",
              updatedAt: "2026-05-26T10:08:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
      failedTasks: [],
      statusGroups: {
        items: [
          {
            key: "draft_saved",
            label: "已保存草稿",
            count: 1,
            ids: ["task-1"],
          },
        ],
        byKey: {
          draft_saved: {
            key: "draft_saved",
            label: "已保存草稿",
            count: 1,
            ids: ["task-1"],
          },
        },
      },
    });

    render(
      <SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "create review tasks" }),
    );

    await waitFor(() =>
      expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", [
        "design-1",
      ]),
    );
    await waitFor(() =>
      expect(screen.getByText("created tasks: 1")).toBeInTheDocument(),
    );
  });

  it("shows a task creation recovery entry when a tasks-created itemized batch is incomplete", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "tasks_created",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:01:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
                {
                  id: "design-2",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-2",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-2.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:02:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
              ],
            },
          ],
          createdTasks: [
            { id: "task-1", title: "Task 1", designId: "design-1" },
          ],
        },
      ),
    );

    render(
      <SheinStudioWorkbench activeStep="tasks" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("created tasks: 1")).toBeInTheDocument(),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "继续创建 SHEIN 资料" }),
    );

    await waitFor(() =>
      expect(screen.getByText("review grid: 2")).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "create review tasks" }),
    ).toBeInTheDocument();
  });

  it("creates missing SHEIN tasks from the dedicated batch recovery button", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "tasks_created",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:01:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
                {
                  id: "design-2",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-2",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-2.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:02:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
              ],
            },
          ],
          createdTasks: [
            { id: "task-1", title: "Task 1", designId: "design-1" },
          ],
        },
      ),
    );
    createSheinStudioBatchTasks.mockResolvedValue({
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "tasks_created",
        updatedAt: "2026-05-26T10:08:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:08:00.000Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:01:00.000Z",
              updatedAt: "2026-05-26T10:08:00.000Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-2",
              targetGroupKey: "size:1000x1000",
              imageUrl: "https://example.com/design-2.png",
              reviewStatus: "approved",
              createdAt: "2026-05-26T10:02:00.000Z",
              updatedAt: "2026-05-26T10:08:00.000Z",
            },
          ],
        },
      ],
      createdTasks: [{ id: "task-2", title: "Task 2", designId: "design-2" }],
      reusedTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
      rejectedTasks: [],
      failedTasks: [],
      statusGroups: {
        items: [
          {
            key: "draft_saved",
            label: "已保存草稿",
            count: 2,
            ids: ["task-1", "task-2"],
          },
        ],
        byKey: {
          draft_saved: {
            key: "draft_saved",
            label: "已保存草稿",
            count: 2,
            ids: ["task-1", "task-2"],
          },
        },
      },
    });

    render(
      <SheinStudioWorkbench activeStep="tasks" initialBatchId="batch-1" />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "补建 SHEIN 资料" }),
    );
    expect(
      screen.queryByRole("button", { name: "继续生成" }),
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(createSheinStudioBatchTasks).toHaveBeenCalledWith("batch-1", [
        "design-1",
        "design-2",
      ]),
    );
    await waitFor(() =>
      expect(screen.getByText("created tasks: 2")).toBeInTheDocument(),
    );
    expect(saveSheinStudioBatch).toHaveBeenCalledWith(
      expect.objectContaining({ id: "batch-1" }),
      expect.objectContaining({ makeActive: false }),
    );
  });

  it("keeps the dedicated batch page busy when generate times out but the batch is still running", async () => {
    getSheinStudioHydratedBatch
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "draft",
              updatedAt: "2026-05-26T10:00:00.000Z",
            },
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "generating",
              updatedAt: "2026-05-26T10:07:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "generating",
                  selectionCount: 1,
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:07:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    startSheinStudioBatchRun.mockRejectedValue(new Error("run start failed"));

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(screen.getByText("run start failed")).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "generate styles" }),
    ).not.toBeDisabled();
  });

  it("keeps the dedicated batch page busy when failed-item retry times out but hydration shows retry is still running", async () => {
    getSheinStudioHydratedBatch
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "partially_failed",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "failed",
                  selectionCount: 1,
                  lastError: "excessive system load",
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "partially_failed",
              updatedAt: "2026-05-26T10:06:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "failed",
                  selectionCount: 1,
                  lastError: "excessive system load",
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:06:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      )
      .mockResolvedValueOnce(
        buildHydratedBatch(
          {
            name: "Retro Cherries",
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            batch: {
              ...buildHydratedBatch().detail.batch,
              status: "generating",
              updatedAt: "2026-05-26T10:07:00.000Z",
            },
            items: [
              {
                item: {
                  id: "item-1",
                  batchId: "batch-1",
                  targetGroupKey: "size:1000x1000",
                  status: "generating",
                  selectionCount: 1,
                  createdAt: "2026-05-26T09:59:00.000Z",
                  updatedAt: "2026-05-26T10:07:00.000Z",
                },
                designs: [],
              },
            ],
          },
        ),
      );
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      name: "Retro Cherries",
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    retrySheinStudioBatchItems.mockRejectedValue(
      new Error("ListingKit API request failed: 504"),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "重试失败批次" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(retrySheinStudioBatchItems).not.toHaveBeenCalled();
  });

  it("retries a single failed batch item without retriggering the whole failed set", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          updatedAt: "2026-05-26T10:00:00.000Z",
        },
        {
          batch: {
            ...buildHydratedBatch().detail.batch,
            status: "partially_failed",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                targetGroupLabel: "黑色 M",
                status: "failed",
                selectionCount: 1,
                lastError: "upstream timeout",
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [],
            },
            {
              item: {
                id: "item-2",
                batchId: "batch-1",
                targetGroupKey: "size:1200x1200",
                targetGroupLabel: "白色 L",
                status: "failed",
                selectionCount: 1,
                lastError: "too many requests",
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:06:00.000Z",
              },
              designs: [],
            },
          ],
        },
      ),
    );
    retrySheinStudioBatchItems.mockResolvedValue({
      ...buildHydratedBatch().detail,
      batch: {
        ...buildHydratedBatch().detail.batch,
        status: "partially_failed",
        updatedAt: "2026-05-26T10:07:00.000Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1000x1000",
            targetGroupLabel: "黑色 M",
            status: "generating",
            selectionCount: 1,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:07:00.000Z",
          },
          designs: [],
        },
        {
          item: {
            id: "item-2",
            batchId: "batch-1",
            targetGroupKey: "size:1200x1200",
            targetGroupLabel: "白色 L",
            status: "failed",
            selectionCount: 1,
            lastError: "too many requests",
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:06:00.000Z",
          },
          designs: [],
        },
      ],
    });

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "retry-item-item-1" }),
      ).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "retry-item-item-1" }));

    await waitFor(() =>
      expect(retrySheinStudioBatchItems).toHaveBeenCalledWith("batch-1", [
        "item-1",
      ]),
    );
    expect(retrySheinStudioBatchItems).not.toHaveBeenCalledWith("batch-1", [
      "item-1",
      "item-2",
    ]);
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("saves the dedicated batch back into the same batch id from the save button", async () => {
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "批次1",
      prompt: "updated prompt",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:00:00.000Z",
    });
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        prompt: "updated prompt",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("updated prompt")).toBeInTheDocument(),
    );

    const onSaveBatch = await waitFor(() => {
      const handler = lastGenerationPanelProps?.onSaveBatch as
        (() => Promise<void> | void) | undefined;
      expect(typeof handler).toBe("function");
      return handler;
    });

    saveSheinStudioBatch.mockClear();

    await act(async () => {
      await onSaveBatch?.();
    });

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          prompt: "updated prompt",
        }),
        { makeActive: false },
      ),
    );
  });

  it("does not let a local draft override the dedicated batch selection", async () => {
    saveLocalSheinStudioDraftSnapshot({
      prompt: "legacy local draft",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        name: "Retro Cherries",
        prompt: "retro cherries",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    expect(screen.getByText("入口商品")).toBeInTheDocument();
    expect(
      screen.queryByDisplayValue("legacy local draft"),
    ).not.toBeInTheDocument();
  });

  it("does not let a stale local snapshot hide newer grouped selections from the dedicated batch", async () => {
    saveLocalSheinStudioDraftSnapshot(
      {
        prompt: "stale local draft",
        styleCount: "1",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        sheinStoreId: "1",
        imageStrategy: "ai_generated",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      { batchId: "batch-1" },
    );
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          prompt: "retro cherries",
          groupedSelections: [
            {
              selectionId: "sel-hoodie",
              sheinStoreId: "869",
              selection: {
                ...selection,
                variantId: 101,
                layerId: "layer-2",
                productName: "hoodie",
                variantLabel: "L / white",
              },
              baselineStatus: "ready",
              baselineReason: "",
              eligible: true,
            },
          ],
          updatedAt: "2026-05-26T10:05:00.000Z",
        },
        {
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "retro cherries",
            styleCount: "1",
            sheinStoreId: 869,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
        },
      ),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByText("已加入 1 款")).toBeInTheDocument(),
    );
    expect(screen.queryByText("已加入 0 款")).not.toBeInTheDocument();
  });

  it("does not let a newer but incomplete local snapshot erase dedicated batch context", async () => {
    saveLocalSheinStudioDraftSnapshot(
      {
        prompt: "",
        styleCount: "1",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        sheinStoreId: "",
        imageStrategy: "ai_generated",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:06:00.000Z",
      },
      { batchId: "batch-1" },
    );
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "869全品类",
          prompt: "retro cherries",
          sheinStoreId: "869",
          groupedSelections: [groupedSelection],
          updatedAt: "2026-05-26T10:05:00.000Z",
        },
        {
          batch: {
            id: "batch-1",
            status: "draft",
            prompt: "",
            styleCount: "",
            sheinStoreId: 0,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:05:00.000Z",
          },
        },
      ),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    await waitFor(() =>
      expect(screen.getByText("已加入 1 款")).toBeInTheDocument(),
    );
    expect(screen.queryByText("已加入 0 款")).not.toBeInTheDocument();
  });

  it("shows a unified dedicated batch header without rename or delete actions", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch({
        name: "869全品类 副本",
      }),
    );
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "869全品类 副本",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    expect(
      await screen.findByRole("heading", { name: "批次工作台" }),
    ).toBeInTheDocument();
    expect(screen.getByText("BATCH WORKBENCH")).toBeInTheDocument();
    expect(screen.getByText("当前批次 · 869全品类 副本")).toBeInTheDocument();
    expect(
      screen.getByText(
        "当前正在继续处理批次 batch-1，可以在这里继续生成、审核和创建任务。",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "返回最近批次首页" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "去 SDS 选品并加入当前批次" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "继续生成剩余款式" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "前往生成区" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "重命名当前批次" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "删除当前批次" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("当前批次名称")).not.toBeInTheDocument();
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
    expect(deleteSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("uses hydrated batch truth for homepage metadata-only writes", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Stale Batch",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        generationJobs: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          id: "batch-1",
          name: "Fresh Batch",
          prompt: "retro cherries",
          designs: [
            { id: "design-1", imageUrl: "https://example.com/design-1.png" },
          ],
          selectedIds: ["design-1"],
          createdTasks: [
            { id: "task-1", title: "Task 1", designId: "design-1" },
          ],
          generationJobs: [
            {
              id: "job-1",
              status: "succeeded",
              requestedCount: 1,
              completedCount: 1,
              createdAt: "2026-05-26T10:02:00.000Z",
              updatedAt: "2026-05-26T10:03:00.000Z",
            },
          ],
          generationError: "legacy warning",
          generationJobId: "job-1",
          updatedAt: "2026-05-26T10:03:00.000Z",
        },
        {
          batch: {
            id: "batch-1",
            status: "tasks_created",
            prompt: "retro cherries",
            styleCount: "1",
            sheinStoreId: 869,
            createdAt: "2026-05-26T09:59:00.000Z",
            updatedAt: "2026-05-26T10:03:00.000Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:03:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T10:01:00.000Z",
                  updatedAt: "2026-05-26T10:03:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Renamed Batch",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [
        { id: "design-1", imageUrl: "https://example.com/design-1.png" },
      ],
      selectedIds: ["design-1"],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
      generationJobs: [
        {
          id: "job-1",
          status: "succeeded",
          requestedCount: 1,
          completedCount: 1,
          createdAt: "2026-05-26T10:02:00.000Z",
          updatedAt: "2026-05-26T10:03:00.000Z",
        },
      ],
      generationError: "legacy warning",
      generationJobId: "job-1",
      updatedAt: "2026-05-26T10:03:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("checkbox", { name: "select batch-1" }),
    );
    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-1"),
    );

    fireEvent.click(screen.getByRole("button", { name: "重命名" }));
    fireEvent.change(screen.getByLabelText("批次名称"), {
      target: { value: "Renamed Batch" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存名称" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          name: "Renamed Batch",
          updatedAt: "2026-05-26T10:03:00.000Z",
          selectedIds: ["design-1"],
          createdTasks: [
            { id: "task-1", title: "Task 1", designId: "design-1" },
          ],
          generationJobs: [
            expect.objectContaining({
              id: "job-1",
              status: "succeeded",
            }),
          ],
          generationError: "legacy warning",
          generationJobId: "job-1",
          designs: [
            expect.objectContaining({
              id: "design-1",
              imageUrl: "https://example.com/design-1.png",
            }),
          ],
        }),
        { makeActive: false },
      ),
    );
  });

  it("duplicates a recent batch through a fresh save input", async () => {
    const sourceBatch = {
      id: "batch-1",
      name: "Fresh Batch",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:03:00.000Z",
    };
    const duplicatedBatch = {
      id: "batch-2",
      name: "Fresh Batch 副本",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationJobs: [],
      updatedAt: "2026-05-26T10:04:00.000Z",
    };
    listSheinStudioBatches
      .mockResolvedValueOnce([sourceBatch])
      .mockResolvedValue([duplicatedBatch, sourceBatch]);
    getSheinStudioHydratedBatch.mockImplementation(async (batchId: string) => {
      if (batchId === "batch-1") {
        return buildHydratedBatch(sourceBatch);
      }
      return buildHydratedBatch(duplicatedBatch, {
        batch: {
          id: "batch-2",
          status: "draft",
          prompt: "retro cherries",
          styleCount: "1",
          sheinStoreId: 869,
          createdAt: "2026-05-26T10:03:30.000Z",
          updatedAt: "2026-05-26T10:04:00.000Z",
        },
      });
    });
    saveSheinStudioBatch.mockResolvedValue({
      ...duplicatedBatch,
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: "复制" }));

    await waitFor(() =>
      expect(getSheinStudioHydratedBatch).toHaveBeenCalledWith("batch-1"),
    );
    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: undefined,
          name: "Fresh Batch 副本",
          prompt: "retro cherries",
        }),
        { makeActive: false },
      ),
    );

    expect(await screen.findByText("Fresh Batch 副本")).toBeInTheDocument();
  });

  it("lets the dedicated batch page jump to SDS selection for the current batch", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(buildHydratedBatch());
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "批次1",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "去 SDS 选品并加入当前批次" }),
    );

    expect(setActiveSheinStudioBatchId).toHaveBeenCalledWith("batch-1");
    expect(push).toHaveBeenCalledWith(
      "/listing-kits/sds/new?targetBatchId=batch-1",
    );
  });

  it("lets the dedicated batch page jump back to generate step", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(buildHydratedBatch());

    render(
      <SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />,
    );

    expect(
      screen.queryByRole("button", { name: "generate styles" }),
    ).not.toBeInTheDocument();

    fireEvent.click(await screen.findByRole("button", { name: "前往生成区" }));

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "generate styles" }),
      ).toBeInTheDocument(),
    );
  });

  it("starts a server-side batch run from the dedicated batch page", async () => {
    getSheinStudioHydratedBatch.mockResolvedValue(buildHydratedBatch());
    startSheinStudioBatchRun.mockResolvedValue({
      run: {
        id: "run-dedicated-1",
      },
      items: [],
    });

    render(
      <SheinStudioWorkbench activeStep="review" initialBatchId="batch-1" />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "继续生成剩余款式" }),
    );

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-dedicated-1"),
    ).toBeInTheDocument();
  });

  it("offers a baseline recovery action on the dedicated batch page when the active selection is abnormal", async () => {
    getSDSBaselineReadiness.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "failed",
      reasonCode: "cache_unavailable",
      reason: "",
    });
    getSheinStudioHydratedBatch.mockResolvedValue(buildHydratedBatch());

    render(
      <SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />,
    );

    await waitFor(() =>
      expect(
        screen.getByText("当前 SDS 选择还没有可用的 baseline 缓存。"),
      ).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "重试 baseline 校验" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试 baseline 校验" }));

    await waitFor(() =>
      expect(warmSDSBaselineForSelection).toHaveBeenCalledWith(selection),
    );
  });

  it("shows the recent batch homepage before a selection is chosen", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="select" />);

    expect(await screen.findByText("最近批次")).toBeInTheDocument();
    expect(screen.getByText("Retro Cherries")).toBeInTheDocument();
    expect(screen.queryByText("入口商品")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "generate styles" }),
    ).not.toBeInTheDocument();
  });

  it("loads the selected batch into the editor when clicking a recent batch card", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Retro Cherries/ }),
    );
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
  });

  it("updates the loaded homepage batch instead of creating a duplicate when saving", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries updated",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:05:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Retro Cherries/ }),
    );
    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries updated" },
    });

    const onSaveBatch = lastGenerationPanelProps?.onSaveBatch as
      (() => Promise<void> | void) | undefined;
    expect(typeof onSaveBatch).toBe("function");
    await act(async () => {
      await onSaveBatch?.();
    });

    await waitFor(() =>
      expect(
        saveSheinStudioBatch.mock.calls.some(([input]) => {
          const candidate = input as
            { id?: string; prompt?: string } | undefined;
          return (
            candidate?.id === "batch-1" &&
            candidate.prompt === "retro cherries updated"
          );
        }),
      ).toBe(true),
    );
  });

  it("routes a recent review-ready batch to the dedicated batch page", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [
          { id: "design-1", imageUrl: "https://example.com/design.png" },
        ],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: "去创建任务" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });

  it("routes a recent batch with tasks to the dedicated batch page", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [
          { id: "design-1", imageUrl: "https://example.com/design.png" },
        ],
        selectedIds: ["design-1"],
        createdTasks: [{ id: "task-1", title: "Task 1" }],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(await screen.findByRole("button", { name: "查看任务" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });

  it("starts a server-side batch run from homepage selection for generate mode", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);
    startSheinStudioBatchRun.mockResolvedValue({
      run: {
        id: "run-1",
      },
      items: [],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("checkbox", { name: "select batch-1" }),
    );
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1", "batch-2"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-1"),
    ).toBeInTheDocument();
    expect(screen.queryByText("最近批次")).not.toBeInTheDocument();
  });

  it("shows batch run start errors inline on the homepage", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Second Batch",
        prompt: "second prompt",
        styleCount: "1",
        sheinStoreId: "",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T09:00:00.000Z",
      },
    ]);
    startSheinStudioBatchRun.mockRejectedValue(new Error("run start failed"));

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("checkbox", { name: "select batch-1" }),
    );
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));

    await waitFor(() =>
      expect(screen.getByText("run start failed")).toBeInTheDocument(),
    );
    expect(screen.queryByText(/batch run progress:/)).not.toBeInTheDocument();
  });

  it("starts a server-side batch run from homepage selection for create-task mode", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockResolvedValue(
      buildHydratedBatch(
        {
          name: "Retro Cherries",
          designs: [
            { id: "design-1", imageUrl: "https://example.com/design.png" },
          ],
          selectedIds: ["design-1"],
        },
        {
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1000x1000",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-05-26T09:59:00.000Z",
                updatedAt: "2026-05-26T10:00:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1000x1000",
                  imageUrl: "https://example.com/design.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-26T09:59:30.000Z",
                  updatedAt: "2026-05-26T10:00:00.000Z",
                },
              ],
            },
          ],
        },
      ),
    );
    startSheinStudioBatchRun.mockResolvedValue({
      run: {
        id: "run-tasks-1",
      },
      items: [],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("checkbox", { name: "select batch-1" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "批量去创建任务 1 个" }),
    );

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "create_tasks",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-tasks-1"),
    ).toBeInTheDocument();
  });

  it("ignores stale recent-batch hydration when a newer batch selection wins", async () => {
    Element.prototype.scrollIntoView = vi.fn();
    const batchADeferred =
      createDeferred<ReturnType<typeof buildHydratedBatch>>();
    const batchBDeferred =
      createDeferred<ReturnType<typeof buildHydratedBatch>>();
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-a",
        name: "Batch A",
        prompt: "prompt a",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-b",
        name: "Batch B",
        prompt: "prompt b",
        styleCount: "1",
        sheinStoreId: "869",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:01:00.000Z",
      },
    ]);
    getSheinStudioHydratedBatch.mockImplementation((batchId: string) => {
      if (batchId === "batch-a") {
        return batchADeferred.promise;
      }
      if (batchId === "batch-b") {
        return batchBDeferred.promise;
      }
      return Promise.resolve(null);
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    const batchACard = (await screen.findByText("Batch A")).closest(
      '[role="button"]',
    );
    const batchBCard = screen.getByText("Batch B").closest('[role="button"]');
    expect(batchACard).not.toBeNull();
    expect(batchBCard).not.toBeNull();

    fireEvent.click(batchACard!);
    fireEvent.click(batchBCard!);

    await act(async () => {
      batchBDeferred.resolve(
        buildHydratedBatch(
          {
            id: "batch-b",
            name: "Batch B",
            prompt: "prompt b",
            selection: groupedSelection.selection,
            updatedAt: "2026-05-26T10:02:00.000Z",
          },
          {
            batch: {
              id: "batch-b",
              status: "draft",
              prompt: "prompt b",
              styleCount: "1",
              sheinStoreId: 869,
              createdAt: "2026-05-26T10:00:00.000Z",
              updatedAt: "2026-05-26T10:02:00.000Z",
            },
          },
        ),
      );
      await Promise.resolve();
    });

    await waitFor(() =>
      expect(screen.getByDisplayValue("prompt b")).toBeInTheDocument(),
    );

    await act(async () => {
      batchADeferred.resolve(
        buildHydratedBatch(
          {
            id: "batch-a",
            name: "Batch A",
            prompt: "prompt a",
            updatedAt: "2026-05-26T10:02:00.000Z",
          },
          {
            batch: {
              id: "batch-a",
              status: "draft",
              prompt: "prompt a",
              styleCount: "1",
              sheinStoreId: 869,
              createdAt: "2026-05-26T09:59:00.000Z",
              updatedAt: "2026-05-26T10:02:00.000Z",
            },
          },
        ),
      );
      await Promise.resolve();
    });

    expect(screen.getByDisplayValue("prompt b")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("prompt a")).not.toBeInTheDocument();
  });

  it("returns from batch run progress and refreshes saved batches", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Batch 1",
        prompt: "prompt 1",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "Batch 2",
        prompt: "prompt 2",
        styleCount: "1",
        sheinStoreId: "869",
        selection: groupedSelection.selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:01:00.000Z",
      },
    ]);
    startSheinStudioBatchRun.mockResolvedValue({
      run: {
        id: "run-2",
      },
      items: [],
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    fireEvent.click(
      await screen.findByRole("checkbox", { name: "select batch-1" }),
    );
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 2 个" }));

    await waitFor(() =>
      expect(screen.getByText("batch run progress: run-2")).toBeInTheDocument(),
    );
    listSheinStudioBatches.mockResolvedValueOnce([
      {
        id: "batch-1",
        name: "Batch 1",
        prompt: "prompt 1",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);
    const callsBeforeBack = listSheinStudioBatches.mock.calls.length;

    fireEvent.click(
      screen.getByRole("button", { name: "back from batch run" }),
    );

    await waitFor(() =>
      expect(listSheinStudioBatches.mock.calls.length).toBeGreaterThan(
        callsBeforeBack,
      ),
    );
    expect(await screen.findByText("最近批次")).toBeInTheDocument();
  });

  it("restores a historic group prompt into the current prompt field", async () => {
    saveLocalSheinStudioDraftSnapshot({
      prompt: "legacy top-level",
      styleCount: "1",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "1",
      imageStrategy: "ai_generated",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection: selection,
          groupedSelections: [],
          styleCount: "1",
          sheinStoreId: "1",
          imageStrategy: "ai_generated",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00.000Z",
            },
          ],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "nanobanana",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T01:00:00.000Z",
        },
      ],
      updatedAt: "2026-05-26T01:00:00.000Z",
    });

    render(<SheinStudioWorkbench activeStep="generate" />);

    expect(await screen.findByDisplayValue("prompt a")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "restore-prompt old" }));
    await waitFor(() =>
      expect(screen.getByDisplayValue("prompt old")).toBeInTheDocument(),
    );
  });

  it("creates a brand-new homepage batch before generation and skips legacy session sync", async () => {
    loadSheinStudioDraft.mockResolvedValue(buildDraft());
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      id: "batch-new",
      name: "新批次",
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue(
      buildReviewReadyHydratedBatchDetail("batch-new"),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-new"],
        "generate",
      ),
    );
    expect(saveSheinStudioBatch).toHaveBeenCalledWith(
      expect.not.objectContaining({ id: expect.anything() }),
      undefined,
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("guards against leaving the page while style generation is running", async () => {
    loadSheinStudioDraft.mockResolvedValue(buildDraft());
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    generateSheinStudioDesigns.mockImplementation(
      () =>
        new Promise(() => {
          return undefined;
        }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("正在生成款式图")).toBeInTheDocument(),
    );

    const anchor = document.createElement("a");
    anchor.href = "https://example.test/listing-kits";
    document.body.appendChild(anchor);

    const cancelled = !anchor.dispatchEvent(
      new MouseEvent("click", { bubbles: true, cancelable: true }),
    );

    expect(confirmSpy).toHaveBeenCalledWith(
      "当前正在生成款式图或创建 SHEIN 资料。现在离开会中断当前页面上的进度承接，确认还要离开吗？",
    );
    expect(cancelled).toBe(true);

    anchor.remove();
  });

  it("does not auto-resume an in-flight generation job after returning to the page", async () => {
    loadSheinStudioDraft.mockResolvedValue(
      buildDraft({
        generationError: "",
        generationJobId: "job-123",
        batchStatus: "generating",
      }),
    );
    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "generate styles" }),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText("review grid: 1")).not.toBeInTheDocument();
    expect(saveSheinStudioDraftWithOptions).not.toHaveBeenCalled();
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("does not auto-resume multiple in-flight generation jobs after returning to the page", async () => {
    loadSheinStudioDraft.mockResolvedValue(
      buildDraft({
        generationError: "",
        generationJobId: "job-123",
        generationJobs: [
          {
            jobId: "job-123",
            targetGroupKey: "primary",
            targetGroupLabel: "当前商品",
            status: "running",
          },
          {
            jobId: "job-456",
            targetGroupKey: "group-1",
            targetGroupLabel: "分组商品 1",
            status: "running",
          },
        ],
        batchStatus: "generating",
      }),
    );
    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "generate styles" }),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText("review grid: 2")).not.toBeInTheDocument();
    expect(saveSheinStudioDraftWithOptions).not.toHaveBeenCalled();
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("does not use legacy session sync when an unsaved homepage workspace generates through batch APIs", async () => {
    loadSheinStudioDraft.mockResolvedValue(buildDraft());
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      id: "batch-unsaved",
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue(
      buildReviewReadyHydratedBatchDetail("batch-unsaved"),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() => expect(loadSheinStudioDraft).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-unsaved"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(generateSheinStudioBatch).not.toHaveBeenCalled();
  });

  it("keeps batch run progress when parent step changes to review", async () => {
    loadSheinStudioDraft.mockResolvedValue(buildDraft());
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue(
      buildReviewReadyHydratedBatchDetail(),
    );

    const rendered = render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();

    loadSheinStudioDraft.mockResolvedValue(
      buildDraft({
        selectedIds: ["design-1"],
      }),
    );

    rendered.rerender(
      <SheinStudioWorkbench activeStep="review" selection={selection} />,
    );

    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
  });

  it("does not surface a draft-save warning after successful batch generation", async () => {
    loadSheinStudioDraft.mockResolvedValue(buildDraft());
    saveSheinStudioBatch.mockResolvedValue({
      ...buildHydratedBatch().savedBatch,
      updatedAt: "2026-05-26T10:04:00.000Z",
    });
    generateSheinStudioBatch.mockResolvedValue(
      buildReviewReadyHydratedBatchDetail(),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByDisplayValue("retro cherries")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(startSheinStudioBatchRun).toHaveBeenCalledWith(
        ["batch-1"],
        "generate",
      ),
    );
    expect(
      await screen.findByText("batch run progress: run-default"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      ),
    ).not.toBeInTheDocument();
  });

  it("imports a gallery handoff into review after SDS selection is available", async () => {
    saveSheinStudioGalleryHandoff({
      createdAt: new Date().toISOString(),
      height: 1000,
      id: "gallery-style-1",
      imageUrl: "https://example.com/gallery-style.png",
      prompt: "retro cherries",
      source: "studio_saved",
      title: "Gallery style",
      width: 1000,
    });

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
  });

  it("passes a Studio subscription gate message into the generation panel when the tenant is not entitled", async () => {
    useQuery.mockReturnValue({
      data: {
        entitlements: [
          {
            allowed: false,
            module: { code: "studio", name: "Studio" },
          },
        ],
      },
      error: null,
    });

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。",
        ),
      ).toBeInTheDocument(),
    );
    expect(lastGenerationPanelProps?.subscriptionBlockedMessage).toBe(
      "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。",
    );
  });

  it("restores grouped selections from a saved draft even when they are not in recent variants", async () => {
    loadSheinStudioDraft.mockResolvedValue(
      buildDraft({
        imageStrategy: "sds_official",
        selectedSdsImages: [],
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
            sheinStoreId: "9",
            eligible: true,
          },
        ],
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(screen.getByText(/已加入\s*1\s*款/)).toBeInTheDocument(),
    );
    expect(screen.getByText("hoodie")).toBeInTheDocument();
  });

  it("passes the grouped image mode into the generation panel", async () => {
    loadSheinStudioDraft.mockResolvedValue(
      buildDraft({
        imageStrategy: "sds_official",
        groupedImageMode: "per_product",
        selectedSdsImages: [],
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() => expect(loadSheinStudioDraft).toHaveBeenCalled());
    expect(lastGenerationPanelProps?.groupedImageMode).toBe("per_product");
  });

  it("hydrates hot style reference state and persists it with the batch", async () => {
    analyzeSheinStudioReferenceStyle.mockResolvedValue({
      referenceStyleBrief: "analyzed reference brief",
      sanitizedPrompt: "Create an original retro badge.",
      warnings: [],
    });
    loadSheinStudioDraft.mockResolvedValue(
      buildDraft({
        hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
        hotStyleReferenceBrief: "saved reference brief",
        hotStyleReferencePrompt: "saved reference prompt",
      }),
    );

    render(
      <SheinStudioWorkbench activeStep="generate" selection={selection} />,
    );

    await waitFor(() =>
      expect(lastGenerationPanelProps?.hotStyleReferenceImageUrls).toEqual([
        "https://example.com/ref.png",
      ]),
    );
    expect(lastGenerationPanelProps?.hotStyleReferenceBrief).toBe(
      "saved reference brief",
    );
    expect(lastGenerationPanelProps?.hotStyleReferencePrompt).toBe(
      "saved reference prompt",
    );

    await (
      lastGenerationPanelProps?.analyzeReferenceStyle as (input: {
        basePrompt?: string;
        referenceImageUrls: string[];
      }) => Promise<unknown>
    )({
      basePrompt: "retro cherries",
      referenceImageUrls: ["https://example.com/ref.png"],
    });

    expect(analyzeSheinStudioReferenceStyle).toHaveBeenCalledWith(
      expect.objectContaining({
        basePrompt: "retro cherries",
        productName: "tee",
        referenceImageUrls: ["https://example.com/ref.png"],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "save batch" }));

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
          hotStyleReferenceBrief: "saved reference brief",
          hotStyleReferencePrompt: "saved reference prompt",
        }),
        { makeActive: undefined },
      ),
    );
  });
});
