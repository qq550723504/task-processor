import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinStudioWorkbench } from "@/components/listingkit/shein-studio/shein-studio-workbench";

const replace = vi.fn();
const generateSheinStudioDesigns = vi.fn();
const createSheinReviewTasks = vi.fn();
const hydrateSDSVariantSelection = vi.fn();
const listSheinStudioBatches = vi.fn();
const loadSheinStudioDraft = vi.fn();
const saveSheinStudioBatch = vi.fn();
const saveSheinStudioDraftWithOptions = vi.fn();
const deleteSheinStudioBatch = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/shein",
  useRouter: () => ({
    replace,
  }),
  useSearchParams: () => new URLSearchParams("step=generate"),
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-selection-overview", () => ({
  SheinStudioSelectionOverview: () => <div>selection overview</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-progress-strip", () => ({
  SheinStudioProgressStrip: () => <div>progress strip</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-created-tasks-list", () => ({
  SheinCreatedTasksList: ({ tasks }: { tasks: Array<{ id: string }> }) => (
    <div>created tasks: {tasks.length}</div>
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-preview-grid", () => ({
  SheinDesignPreviewGrid: ({
    designs,
    onCreateReviewTasks,
  }: {
    designs: Array<{ id: string }>;
    onCreateReviewTasks?: () => void;
  }) => (
    <div>
      <div>review grid: {designs.length}</div>
      {onCreateReviewTasks ? (
        <button onClick={onCreateReviewTasks} type="button">
          create review tasks
        </button>
      ) : null}
    </div>
  ),
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-generation-panel", () => ({
  SheinStudioGenerationPanel: ({
    onGenerate,
    prompt,
    setPrompt,
  }: {
    onGenerate: () => void;
    prompt: string;
    setPrompt: (value: string) => void;
  }) => (
    <div>
      <label htmlFor="prompt">prompt</label>
      <input
        id="prompt"
        onChange={(event) => setPrompt(event.target.value)}
        value={prompt}
      />
      <button onClick={onGenerate} type="button">
        generate styles
      </button>
    </div>
  ),
}));

vi.mock("@/lib/api/shein-studio", () => ({
  generateSheinStudioDesigns: (...args: unknown[]) => generateSheinStudioDesigns(...args),
}));

vi.mock("@/lib/shein-studio/create-review-tasks", async () => {
  const actual = await vi.importActual<typeof import("@/lib/shein-studio/create-review-tasks")>(
    "@/lib/shein-studio/create-review-tasks",
  );
  return {
    ...actual,
    createSheinReviewTasks: (...args: unknown[]) => createSheinReviewTasks(...args),
  };
});

vi.mock("@/lib/shein-studio/hydrate-sds-selection", () => ({
  hydrateSDSVariantSelection: (...args: unknown[]) => hydrateSDSVariantSelection(...args),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  deleteSheinStudioBatch: (...args: unknown[]) => deleteSheinStudioBatch(...args),
  listSheinStudioBatches: (...args: unknown[]) => listSheinStudioBatches(...args),
  loadSheinStudioDraft: (...args: unknown[]) => loadSheinStudioDraft(...args),
  saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
  saveSheinStudioDraftWithOptions: (...args: unknown[]) =>
    saveSheinStudioDraftWithOptions(...args),
}));

const selection = {
  productId: 1,
  parentProductId: 1,
  variantId: 100,
  prototypeGroupId: 200,
  layerId: "layer-1",
  productName: "tee",
  variantLabel: "M / black",
  printableWidth: 1000,
  printableHeight: 1000,
};

describe("SheinStudioWorkbench", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    replace.mockReset();
    generateSheinStudioDesigns.mockReset();
    createSheinReviewTasks.mockReset();
    hydrateSDSVariantSelection.mockResolvedValue(selection);
    listSheinStudioBatches.mockResolvedValue([]);
    loadSheinStudioDraft.mockResolvedValue(null);
    saveSheinStudioBatch.mockResolvedValue(null);
    saveSheinStudioDraftWithOptions.mockRejectedValue(new Error("timeout"));
    deleteSheinStudioBatch.mockResolvedValue(undefined);
  });

  it("keeps generated designs visible when draft save and route replace fail", async () => {
    replace.mockImplementation(() => {
      throw new Error("navigation failed");
    });
    generateSheinStudioDesigns.mockResolvedValue({
      images: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    fireEvent.change(screen.getByLabelText("prompt"), {
      target: { value: "retro cherries" },
    });
    fireEvent.click(screen.getByRole("button", { name: "generate styles" }));

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      ),
    ).toBeInTheDocument();
  });

  it("enters tasks view after task creation even when draft save fails", async () => {
    replace.mockImplementation(() => {
      throw new Error("navigation failed");
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
      imageStrategy: "ai_generated",
      renderSizeImagesWithSds: true,
      selectionVariantId: 100,
      selection,
      designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
      selectedIds: ["design-1"],
      createdTasks: [],
      updatedAt: "2026-04-29T00:00:00.000Z",
    });
    createSheinReviewTasks.mockResolvedValue([
      { id: "task-1", title: "Task 1", designId: "design-1" },
    ]);

    render(<SheinStudioWorkbench activeStep="review" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByText("review grid: 1")).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "create review tasks" }));

    await waitFor(() =>
      expect(screen.getByText("created tasks: 1")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      ),
    ).toBeInTheDocument();
  });
});
