import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinStudioBatchDetail } from "@/components/listingkit/shein-studio/shein-studio-batch-detail";
import { ApiError } from "@/lib/api/client";
import type { SheinStudioHydratedBatch } from "@/lib/utils/shein-studio-batches";

const getSheinStudioBatch = vi.fn();
const getSheinStudioHydratedBatch = vi.fn();
const saveSheinStudioBatch = vi.fn();
const deleteSheinStudioBatch = vi.fn();
const createSheinReviewTasks = vi.fn();
const mockedCreateSheinStudioBatchTasks = vi.fn();
const approveSheinStudioBatchDesigns = vi.fn();
const setActiveSheinStudioBatchId = vi.fn();
const push = vi.fn();

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
  }),
}));

vi.mock("@/components/listingkit/shein-studio/shein-batch-publish-gate", () => ({
  SheinBatchPublishGate: () => <div>publish gate</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-batch-task-tracker", () => ({
  SheinBatchTaskTracker: () => <div>task tracker</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-preview-grid", () => ({
  SheinDesignPreviewGrid: () => <div>design preview grid</div>,
}));

vi.mock("@/lib/shein-studio/create-review-tasks", () => ({
  createSheinReviewTasks: (...args: unknown[]) => createSheinReviewTasks(...args),
}));

vi.mock("@/lib/api/shein-studio-batches", () => ({
  approveSheinStudioBatchDesigns: (...args: unknown[]) =>
    approveSheinStudioBatchDesigns(...args),
  createSheinStudioBatchTasks: (...args: unknown[]) =>
    mockedCreateSheinStudioBatchTasks(...args),
}));

vi.mock("@/lib/utils/shein-studio-batches", async () => {
  const actual = await vi.importActual<
    typeof import("@/lib/utils/shein-studio-batches")
  >("@/lib/utils/shein-studio-batches");
  return {
    ...actual,
    getSheinStudioBatch: (...args: unknown[]) => getSheinStudioBatch(...args),
    getSheinStudioHydratedBatch: (...args: unknown[]) =>
      getSheinStudioHydratedBatch(...args),
    saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
    deleteSheinStudioBatch: (...args: unknown[]) => deleteSheinStudioBatch(...args),
    setActiveSheinStudioBatchId: (...args: unknown[]) =>
      setActiveSheinStudioBatchId(...args),
  };
});

function buildHydratedBatch(): SheinStudioHydratedBatch {
  return {
    savedBatch: {
      id: "batch-1",
      name: "retro cherries",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection: {
        productId: 1,
        parentProductId: 10,
        variantId: 2,
        prototypeGroupId: 3,
        layerId: "layer-1",
        productName: "Curtain",
        variantLabel: "Blue",
        printableWidth: 1200,
        printableHeight: 1600,
        selectedVariantIds: [2, 4],
      },
      imageStrategy: "sds_official" as const,
      groupedImageMode: "shared_by_size" as const,
      productImageCount: "1",
      renderSizeImagesWithSds: true,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-18T18:30:00.000Z",
    },
    detail: {
      batch: {
        id: "batch-1",
        status: "review_ready" as const,
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-18T18:20:00.000Z",
        updatedAt: "2026-05-18T18:30:00.000Z",
      },
      items: [],
    },
  };
}

describe("SheinStudioBatchDetail", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    approveSheinStudioBatchDesigns.mockReset();
    mockedCreateSheinStudioBatchTasks.mockReset();
    getSheinStudioBatch.mockReset();
    getSheinStudioHydratedBatch.mockReset();
    saveSheinStudioBatch.mockReset();
    deleteSheinStudioBatch.mockReset();
    createSheinReviewTasks.mockReset();
    setActiveSheinStudioBatchId.mockReset();
    push.mockReset();
  });

  it("uses a mobile-first summary layout for batch details", async () => {
    getSheinStudioHydratedBatch.mockResolvedValueOnce(buildHydratedBatch());

    const { container } = render(<SheinStudioBatchDetail batchId="batch-1" />);

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: "retro cherries" }),
      ).toBeInTheDocument(),
    );

    const summarySection = screen
      .getByRole("heading", { level: 1, name: "retro cherries" })
      .closest("section");
    expect(summarySection).not.toBeNull();
    expect(summarySection?.className).not.toContain("lg:grid-cols-[1.15fr_0.85fr]");

    const actionGroup = screen
      .getByRole("button", { name: "继续选品并加入当前批次" })
      .parentElement;
    expect(actionGroup).not.toBeNull();
    expect(actionGroup?.className).toContain("flex-col");

    const metricsGrid = container.querySelector(
      ".grid.gap-3",
    ) as HTMLDivElement | null;
    expect(metricsGrid).not.toBeNull();
    expect(metricsGrid?.className).not.toContain("lg:grid-cols-1");
  });

  it("routes back to SDS selection with the current batch activated for adding products", async () => {
    getSheinStudioHydratedBatch.mockResolvedValueOnce(buildHydratedBatch());

    render(<SheinStudioBatchDetail batchId="batch-1" />);

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: "retro cherries" }),
      ).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "继续选品并加入当前批次" }));

    expect(setActiveSheinStudioBatchId).toHaveBeenCalledWith("batch-1");
    expect(push).toHaveBeenCalledWith(
      "/listing-kits/sds?productId=1&parentProductId=10&variantId=2&prototypeGroupId=3&layerId=layer-1&printWidth=1200&printHeight=1600&variantIds=2%2C4",
    );
  });

  it("shows not found only for a real 404 batch lookup", async () => {
    getSheinStudioHydratedBatch.mockRejectedValueOnce(
      new ApiError("ListingKit API request failed: 404", 404, {
        message: "not found",
      }),
    );

    render(<SheinStudioBatchDetail batchId="missing-batch" />);

    await waitFor(() =>
      expect(screen.getByText("未找到批次")).toBeInTheDocument(),
    );
    expect(screen.queryByText("批次加载失败")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重试加载" })).not.toBeInTheDocument();
  });

  it("surfaces server errors instead of pretending the batch is missing", async () => {
    getSheinStudioHydratedBatch
      .mockRejectedValueOnce(
        new ApiError("ListingKit API request failed: 500", 500, {
          message: "upstream unavailable",
        }),
      )
      .mockResolvedValueOnce(buildHydratedBatch());

    render(<SheinStudioBatchDetail batchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("批次加载失败")).toBeInTheDocument(),
    );
    expect(screen.getByText("ListingKit API request failed: 500")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试加载" }));

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: "retro cherries" }),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText("批次加载失败")).not.toBeInTheDocument();
    expect(getSheinStudioHydratedBatch.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it("creates tasks from hydrated itemized batch detail while keeping saved batch selection context", async () => {
    const hydratedBatch = buildHydratedBatch();
    hydratedBatch.savedBatch.groupedSelections = [
      {
        selectionId: "sel-2",
        selection: {
          productId: 1,
          parentProductId: 10,
          variantId: 4,
          prototypeGroupId: 3,
          layerId: "layer-2",
          productName: "Curtain",
          variantLabel: "White",
        },
        baselineStatus: "ready",
        baselineReason: "",
        sheinStoreId: "869",
        eligible: true,
      },
    ];
    hydratedBatch.detail.items = [
      {
        item: {
          id: "item-1",
          batchId: "batch-1",
          targetGroupKey: "size:1200x1600",
          status: "review_ready",
          selectionCount: 1,
          createdAt: "2026-05-18T18:20:00.000Z",
          updatedAt: "2026-05-18T18:30:00.000Z",
        },
        designs: [
          {
            id: "design-1",
            batchId: "batch-1",
            itemId: "item-1",
            sourceAttemptId: "attempt-1",
            targetGroupKey: "size:1200x1600",
            imageUrl: "https://example.com/design-1.png",
            reviewStatus: "approved",
            createdAt: "2026-05-18T18:25:00.000Z",
            updatedAt: "2026-05-18T18:30:00.000Z",
          },
          {
            id: "design-2",
            batchId: "batch-1",
            itemId: "item-1",
            sourceAttemptId: "attempt-2",
            targetGroupKey: "size:1200x1600",
            imageUrl: "https://example.com/design-2.png",
            reviewStatus: "unreviewed",
            createdAt: "2026-05-18T18:26:00.000Z",
            updatedAt: "2026-05-18T18:30:00.000Z",
          },
        ],
      },
    ];
    getSheinStudioHydratedBatch.mockResolvedValueOnce(hydratedBatch);
    mockedCreateSheinStudioBatchTasks.mockResolvedValueOnce({
      batch: {
        id: "batch-1",
        status: "tasks_created",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-18T18:20:00.000Z",
        updatedAt: "2026-05-18T18:40:00.000Z",
      },
      items: [],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
    });

    render(<SheinStudioBatchDetail batchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("共 2 个 / 已批准 1 个")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "生成 SHEIN 资料" }));

    await waitFor(() =>
      expect(mockedCreateSheinStudioBatchTasks).toHaveBeenCalledWith(
        "batch-1",
        ["design-1"],
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "继续选品并加入当前批次" }));

    expect(setActiveSheinStudioBatchId).toHaveBeenCalledWith("batch-1");
    expect(push).toHaveBeenCalledWith(
      "/listing-kits/sds?productId=1&parentProductId=10&variantId=2&prototypeGroupId=3&layerId=layer-1&printWidth=1200&printHeight=1600&variantIds=2%2C4",
    );
  });
});
