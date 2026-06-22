import { describe, expect, it, vi } from "vitest";

import {
  buildGenerationPromptHistoryGroups,
  executeStandaloneGeneration,
  mergeGeneratedDesignCollections,
  mergeGeneratedSelectedIds,
  resolveGenerationStartValidation,
  resolveRegenerationStartValidation,
  replaceRegeneratedDesign,
  withGenerationTargetMetadata,
} from "@/lib/shein-studio/generation-controller";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioGeneratedDesign,
  SheinStudioGroupedWorkspace,
} from "@/lib/types/shein-studio";

const selection: SDSProductVariantSelection = {
  productId: 100,
  parentProductId: 100,
  variantId: 101,
  prototypeGroupId: 1,
  layerId: "layer-1",
  productName: "T-shirt",
  variantLabel: "L / white",
};

function buildGroup(
  overrides: Partial<SheinStudioGroupedWorkspace> = {},
): SheinStudioGroupedWorkspace {
  return {
    id: "group-1",
    name: "Group 1",
    primarySelection: selection,
    groupedSelections: [],
    sheinStoreId: "869",
    currentPrompt: "old prompt",
    promptHistory: [],
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-22T00:00:00.000Z",
    ...overrides,
  };
}

describe("SHEIN Studio generation controller", () => {
  it("validates generation start input with existing messages", () => {
    expect(
      resolveGenerationStartValidation({
        activeSelection: undefined,
        prompt: "prompt",
        sheinStoreId: "869",
      }),
    ).toEqual({ error: "请先选择 SDS 变体。" });
    expect(
      resolveGenerationStartValidation({
        activeSelection: selection,
        prompt: "prompt",
        sheinStoreId: " ",
      }),
    ).toEqual({ error: "请先选择批次店铺。" });
    expect(
      resolveGenerationStartValidation({
        activeSelection: selection,
        prompt: " ",
        sheinStoreId: "869",
      }),
    ).toEqual({ error: "请先填写主题提示词。", focusPrompt: true });
    expect(
      resolveGenerationStartValidation({
        activeSelection: selection,
        prompt: "prompt",
        sheinStoreId: "869",
      }),
    ).toBeNull();
  });

  it("validates regeneration start input with existing messages", () => {
    expect(
      resolveRegenerationStartValidation({
        activeSelection: undefined,
        prompt: "prompt",
      }),
    ).toEqual({ error: "请先选择 SDS 变体。" });
    expect(
      resolveRegenerationStartValidation({
        activeSelection: selection,
        prompt: "",
      }),
    ).toEqual({ error: "请先填写主题提示词。", focusPrompt: true });
    expect(
      resolveRegenerationStartValidation({
        activeSelection: selection,
        prompt: "prompt",
      }),
    ).toBeNull();
  });

  it("appends prompt history for the active group without duplicating newest entries", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-22T12:00:00.000Z"));
    const groups = [
      buildGroup({
        promptHistory: [
          {
            prompt: "new prompt",
            groupedImageMode: "shared_by_size",
            createdAt: "2026-06-22T11:00:00.000Z",
          },
        ],
      }),
      buildGroup({ id: "group-2", name: "Group 2" }),
    ];

    const deduped = buildGenerationPromptHistoryGroups({
        activeGroupId: "group-1",
        groupedImageMode: "shared_by_size",
        groups,
        prompt: " new prompt ",
      });
    expect(deduped).not.toBe(groups);
    expect(deduped[0]?.currentPrompt).toBe("new prompt");
    expect(deduped[0]?.promptHistory).toEqual(groups[0]?.promptHistory);

    const updated = buildGenerationPromptHistoryGroups({
      activeGroupId: "group-1",
      groupedImageMode: "per_product",
      groups,
      prompt: "new prompt",
    });

    expect(updated).not.toBe(groups);
    expect(updated[0]).toMatchObject({
      currentPrompt: "new prompt",
      updatedAt: "2026-06-22T12:00:00.000Z",
    });
    expect(updated[0]?.promptHistory).toEqual([
      {
        prompt: "new prompt",
        groupedImageMode: "per_product",
        createdAt: "2026-06-22T12:00:00.000Z",
      },
      {
        prompt: "new prompt",
        groupedImageMode: "shared_by_size",
        createdAt: "2026-06-22T11:00:00.000Z",
      },
    ]);
    expect(updated[1]).toBe(groups[1]);
    vi.useRealTimers();
  });

  it("keeps prompt history capped at five entries", () => {
    const history = Array.from({ length: 5 }, (_, index) => ({
      prompt: `old ${index}`,
      groupedImageMode: "shared_by_size" as const,
      createdAt: `2026-06-22T0${index}:00:00.000Z`,
    }));
    const [group] = buildGenerationPromptHistoryGroups({
      activeGroupId: "group-1",
      groupedImageMode: "shared_by_size",
      groups: [buildGroup({ promptHistory: history })],
      prompt: "new",
    });

    expect(group?.promptHistory).toHaveLength(5);
    expect(group?.promptHistory[0]?.prompt).toBe("new");
    expect(group?.promptHistory.at(-1)?.prompt).toBe("old 3");
  });

  it("merges generated designs by id and selected ids without duplicates", () => {
    const current = [
      { id: "design-1", imageUrl: "old.png" },
      { id: "design-2", imageUrl: "second.png" },
    ];
    const incoming = [
      { id: "design-1", imageUrl: "new.png" },
      { id: "design-3", imageUrl: "third.png" },
    ];

    expect(mergeGeneratedDesignCollections(current, incoming)).toEqual([
      { id: "design-1", imageUrl: "new.png" },
      { id: "design-2", imageUrl: "second.png" },
      { id: "design-3", imageUrl: "third.png" },
    ]);
    expect(mergeGeneratedSelectedIds(["design-1"], incoming)).toEqual([
      "design-1",
      "design-3",
    ]);
  });

  it("adds generation target metadata to all returned images", () => {
    expect(
      withGenerationTargetMetadata(
        [{ id: "design-1" }],
        { key: "size:L", label: "Large" },
      ),
    ).toEqual([
      { id: "design-1", targetGroupKey: "size:L", targetGroupLabel: "Large" },
    ]);
  });

  it("replaces a regenerated design while preserving stable id and target metadata", () => {
    const designs: SheinStudioGeneratedDesign[] = [
      {
        id: "design-1",
        imageUrl: "old.png",
        targetGroupKey: "size:L",
        targetGroupLabel: "Large",
      },
      { id: "design-2", imageUrl: "other.png" },
    ];

    expect(
      replaceRegeneratedDesign({
        designs,
        designId: "design-1",
        replacement: { id: "replacement", imageUrl: "new.png" },
      }),
    ).toEqual([
      {
        id: "design-1",
        imageUrl: "new.png",
        targetGroupKey: "size:L",
        targetGroupLabel: "Large",
      },
      { id: "design-2", imageUrl: "other.png" },
    ]);
  });

  it("executes standalone grouped generation and persists successful results", async () => {
    const setField = vi.fn();
    const persistDraft = vi.fn().mockResolvedValue(undefined);
    const navigateToStep = vi.fn();
    const generateDesigns = vi.fn().mockResolvedValue({
      images: [{ id: "design-1", imageUrl: "generated.png" }],
      warnings: ["soft warning"],
    });
    const localWorkflowStateRef = { current: false };
    const groups = [buildGroup()];

    const result = await executeStandaloneGeneration({
      activeGroupId: "group-1",
      activeSelection: {
        ...selection,
        printableWidth: 300,
        printableHeight: 400,
      },
      artworkModel: "gpt-image-1",
      generateDesigns,
      generationJobs: [],
      groupedImageMode: "shared_by_size",
      groupedSelections: [],
      groups,
      hasLocalWorkflowStateRef: localWorkflowStateRef,
      navigateToStep,
      persistDraft,
      prompt: " summer flowers ",
      setField,
      styleCount: "1",
      transparentBackground: false,
      variationIntensity: "medium",
    });

    expect(result.designs).toEqual([
      {
        id: "design-1",
        imageUrl: "generated.png",
        targetGroupKey: expect.any(String),
        targetGroupLabel: expect.any(String),
      },
    ]);
    expect(generateDesigns).toHaveBeenCalledWith(
      expect.objectContaining({
        count: 1,
        printableWidth: 300,
        printableHeight: 400,
        prompt: expect.stringContaining("summer flowers"),
      }),
      expect.objectContaining({ onJobStarted: expect.any(Function) }),
    );
    expect(setField).toHaveBeenCalledWith("designs", result.designs);
    expect(setField).toHaveBeenCalledWith("selectedIds", ["design-1"]);
    expect(setField).toHaveBeenCalledWith("generationJobs", []);
    expect(setField).toHaveBeenCalledWith(
      "generationWarning",
      "soft warning",
    );
    expect(navigateToStep).toHaveBeenCalledWith("review");
    expect(persistDraft).toHaveBeenCalledWith(
      expect.objectContaining({
        designs: result.designs,
        selectedIds: ["design-1"],
        createdTasks: [],
        generationJobs: [],
      }),
      expect.objectContaining({
        navigationTriggered: true,
        source: "generate_success",
      }),
    );
    expect(localWorkflowStateRef.current).toBe(true);
  });

  it("keeps standalone generation results when final draft persistence fails", async () => {
    const setField = vi.fn();
    const persistDraft = vi.fn().mockRejectedValue(new Error("save failed"));
    const navigateToStep = vi.fn();
    const generateDesigns = vi.fn().mockResolvedValue({
      images: [{ id: "design-1", imageUrl: "generated.png" }],
    });
    const localWorkflowStateRef = { current: false };

    const result = await executeStandaloneGeneration({
      activeGroupId: "group-1",
      activeSelection: selection,
      artworkModel: "gpt-image-1",
      generateDesigns,
      generationJobs: [],
      groupedImageMode: "shared_by_size",
      groupedSelections: [],
      groups: [buildGroup()],
      hasLocalWorkflowStateRef: localWorkflowStateRef,
      navigateToStep,
      persistDraft,
      prompt: "summer flowers",
      setField,
      styleCount: "1",
      transparentBackground: false,
      variationIntensity: "medium",
    });

    expect(result.designs).toEqual([
      {
        id: "design-1",
        imageUrl: "generated.png",
        targetGroupKey: expect.any(String),
        targetGroupLabel: expect.any(String),
      },
    ]);
    expect(setField).toHaveBeenCalledWith("designs", result.designs);
    expect(setField).toHaveBeenCalledWith("selectedIds", ["design-1"]);
    expect(setField).toHaveBeenCalledWith("generationJobs", []);
    expect(navigateToStep).toHaveBeenCalledWith("review");
    expect(persistDraft).toHaveBeenCalledWith(
      expect.objectContaining({
        designs: result.designs,
        selectedIds: ["design-1"],
        generationJobs: [],
      }),
      expect.objectContaining({
        navigationTriggered: true,
        source: "generate_success",
        warnOnFailure: false,
      }),
    );
    expect(localWorkflowStateRef.current).toBe(true);
  });

  it("fails standalone generation with an explicit error when no images are returned", async () => {
    const setField = vi.fn();
    const persistDraft = vi.fn().mockResolvedValue(undefined);
    const navigateToStep = vi.fn();
    const generateDesigns = vi.fn().mockResolvedValue({
      images: [],
    });
    const localWorkflowStateRef = { current: false };

    await expect(
      executeStandaloneGeneration({
        activeGroupId: "group-1",
        activeSelection: selection,
        artworkModel: "gpt-image-1",
        generateDesigns,
        generationJobs: [],
        groupedImageMode: "shared_by_size",
        groupedSelections: [],
        groups: [buildGroup()],
        hasLocalWorkflowStateRef: localWorkflowStateRef,
        navigateToStep,
        persistDraft,
        prompt: "summer flowers",
        setField,
        styleCount: "1",
        transparentBackground: false,
        variationIntensity: "medium",
      }),
    ).rejects.toThrow(
      "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
    );

    expect(generateDesigns).toHaveBeenCalled();
    expect(navigateToStep).not.toHaveBeenCalledWith("review");
    expect(persistDraft).not.toHaveBeenCalledWith(
      expect.objectContaining({ source: "generate_success" }),
      expect.anything(),
    );
    expect(localWorkflowStateRef.current).toBe(false);
  });
});
