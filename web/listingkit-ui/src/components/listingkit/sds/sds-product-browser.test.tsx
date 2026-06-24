import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SDSProductBrowser } from "@/components/listingkit/sds/sds-product-browser";

const push = vi.fn();
const saveSheinStudioBatch = vi.fn();
const listSheinStudioBatches = vi.fn();
const getSheinStudioBatch = vi.fn();
const getActiveSheinStudioBatchId = vi.fn();
const setActiveSheinStudioBatchId = vi.fn();
const getSDSBaselineReadiness = vi.fn();
const warmSDSBaselineForSelection = vi.fn();
const saveSDSGroupedCandidateHandoff = vi.fn();
const createSDSRetirementRun = vi.fn();
const updateSDSRetirementSelection = vi.fn();
const confirmSDSRetirementRun = vi.fn();
let currentSearchParams = new URLSearchParams();

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds/new",
  useRouter: () => ({ push }),
}));

vi.mock("@/lib/utils/live-search-params", () => ({
  useLiveSearchParams: () => currentSearchParams,
}));

vi.mock("@/lib/query/use-sds-recent-variants", () => ({
  useSDSRecentVariants: () => [],
}));

vi.mock("@/lib/query/use-sds-shipment-areas", () => ({
  useSDSShipmentAreas: () => ({
    data: [{ label: "US", value: "US" }],
    isLoading: false,
  }),
}));

vi.mock("@/lib/query/use-sds-categories", () => ({
  useSDSCategories: () => ({
    data: [],
    isLoading: false,
  }),
}));

vi.mock("@/lib/query/use-sds-products", () => ({
  useSDSProducts: () => ({
    data: {
      items: [{ id: 101, product_name: "Placemat", productName: "Placemat" }],
      size: 12,
      totalCount: 1,
    },
    error: null,
    isLoading: false,
  }),
}));

vi.mock("@/lib/query/use-sds-product-detail", () => ({
  useSDSProductDetail: () => ({
    data: {
      id: 101,
      product_name: "Placemat",
      productName: "Placemat",
      subproducts: {
        items: [{ id: 201, size: "40x30cm", color_name: "White" }],
      },
    },
    error: null,
    isLoading: false,
  }),
}));

vi.mock("@/lib/sds/variant-selection", () => ({
  buildSDSVariantSelection: () => ({
    productId: 101,
    parentProductId: 101,
    variantId: 201,
    prototypeGroupId: 301,
    layerId: "layer-a",
    productName: "Placemat",
    variantLabel: "40x30cm · White",
    selectedVariantIds: [201],
  }),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  getSheinStudioBatch: (batchId: string) => getSheinStudioBatch(batchId),
  getActiveSheinStudioBatchId: () => getActiveSheinStudioBatchId(),
  listSheinStudioBatches: () => listSheinStudioBatches(),
  saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
  setActiveSheinStudioBatchId: (batchId: string) =>
    setActiveSheinStudioBatchId(batchId),
}));

vi.mock("@/lib/utils/sds-recent-variants", () => ({
  saveRecentSDSVariant: vi.fn(),
}));

vi.mock("@/lib/utils/browser-history", () => ({
  replaceBrowserHistory: vi.fn(),
}));

vi.mock("@/lib/utils/navigation-query", () => ({
  sanitizedNavigationSearchParams: (params: URLSearchParams) => new URLSearchParams(params),
}));

vi.mock("@/lib/api/sds-baseline", () => ({
  getSDSBaselineReadiness: (...args: unknown[]) => getSDSBaselineReadiness(...args),
  warmSDSBaselineForSelection: (...args: unknown[]) =>
    warmSDSBaselineForSelection(...args),
}));

vi.mock("@/lib/utils/sds-grouped-candidate-handoff", () => ({
  saveSDSGroupedCandidateHandoff: (...args: unknown[]) =>
    saveSDSGroupedCandidateHandoff(...args),
}));

vi.mock("@/lib/api/sds-retirement", () => ({
  createSDSRetirementRun: (...args: unknown[]) => createSDSRetirementRun(...args),
  updateSDSRetirementSelection: (...args: unknown[]) =>
    updateSDSRetirementSelection(...args),
  confirmSDSRetirementRun: (...args: unknown[]) => confirmSDSRetirementRun(...args),
}));

vi.mock("@/components/listingkit/sds/sds-product-browser-filters", () => ({
  SDSProductBrowserFilters: () => <div>filters</div>,
}));
vi.mock("@/components/listingkit/sds/sds-recent-variants", () => ({
  SDSRecentVariants: () => <div>recent variants</div>,
}));
vi.mock("@/components/listingkit/sds/sds-selection-summary", () => ({
  SDSSelectionSummary: () => null,
}));
vi.mock("@/components/listingkit/sds/sds-pagination", () => ({
  SDSPagination: () => null,
}));
vi.mock("@/components/listingkit/sds/sds-product-card", () => ({
  SDSProductCard: ({ onOpenVariants }: { onOpenVariants: () => void }) => (
    <button onClick={onOpenVariants} type="button">
      open variants
    </button>
  ),
}));
vi.mock("@/components/listingkit/sds/sds-variant-picker", () => ({
  SDSVariantPicker: ({
    activeBatchId,
    onAddSelectedVariantsToBatch,
    onSelectVariants,
    open,
    variants,
  }: {
    activeBatchId?: string;
    onAddSelectedVariantsToBatch?: (
      primary: { id: number },
      variants: Array<{ id: number }>,
      batchId: string,
    ) => void;
    onSelectVariants: (primary: { id: number }, variants: Array<{ id: number }>) => void;
    open: boolean;
    variants: Array<{ id: number }>;
  }) =>
    open ? (
      <>
        <button
          onClick={() => onSelectVariants(variants[0], variants)}
          type="button"
        >
          use selected variants
        </button>
        {onAddSelectedVariantsToBatch && activeBatchId ? (
          <button
            onClick={() =>
              onAddSelectedVariantsToBatch(variants[0], variants, activeBatchId)
            }
            type="button"
          >
            add selected variants to current batch
          </button>
        ) : null}
      </>
    ) : null,
}));

describe("SDSProductBrowser", () => {
  beforeEach(() => {
    push.mockReset();
    saveSheinStudioBatch.mockReset();
    listSheinStudioBatches.mockReset();
    getSheinStudioBatch.mockReset();
    getActiveSheinStudioBatchId.mockReset();
    saveSheinStudioBatch.mockResolvedValue({ id: "batch-new" });
    listSheinStudioBatches.mockResolvedValue([]);
    getSheinStudioBatch.mockResolvedValue(null);
    getActiveSheinStudioBatchId.mockReturnValue("");
    setActiveSheinStudioBatchId.mockReset();
    getSDSBaselineReadiness.mockReset();
    warmSDSBaselineForSelection.mockReset();
    saveSDSGroupedCandidateHandoff.mockReset();
    createSDSRetirementRun.mockReset();
    updateSDSRetirementSelection.mockReset();
    confirmSDSRetirementRun.mockReset();
    currentSearchParams = new URLSearchParams();
    getSDSBaselineReadiness.mockResolvedValue({
      status: "ready",
      reason: "",
    });
    updateSDSRetirementSelection.mockImplementation(async (runId, items) => ({
      run: {
        id: runId,
        tenant_id: "tenant-a",
        platform: "shein",
        store_id: 869,
        parent_product_id: 101,
        prototype_group_id: 301,
        variant_id: 201,
        status: "ready",
        reason_code: "product_detail_check_failed",
        reason: "SDS product detail check failed: 产品已下架",
      },
      items: [
        {
          id: "item-1",
          run_id: runId,
          platform: "shein",
          store_id: 869,
          spu_name: "Placemat",
          skc_name: items[0]?.selected ? "SKC-1" : "SKC-1",
          selected: items[0]?.selected ?? true,
          site_selection: '[{"site_abbr":"US","store_type":1}]',
          status: items[0]?.selected ? "selected" : "pending",
        },
      ],
    }));
    confirmSDSRetirementRun.mockImplementation(async (runId) => ({
      run: {
        id: runId,
        tenant_id: "tenant-a",
        platform: "shein",
        store_id: 869,
        parent_product_id: 101,
        prototype_group_id: 301,
        variant_id: 201,
        status: "running",
        reason_code: "product_detail_check_failed",
        reason: "SDS product detail check failed: 产品已下架",
      },
      items: [
        {
          id: "item-1",
          run_id: runId,
          platform: "shein",
          store_id: 869,
          spu_name: "Placemat",
          skc_name: "SKC-1",
          selected: true,
          site_selection: '[{"site_abbr":"US","store_type":1}]',
          status: "running",
        },
      ],
    }));
  });

  it("creates a batch and routes to the dedicated batch page on the /new route", async () => {
    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(screen.getByRole("button", { name: "use selected variants" }));

    await waitFor(() => {
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          prompt: "",
          selection: expect.objectContaining({
            variantId: 201,
          }),
        }),
      );
    });
    await waitFor(() => {
      expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-new");
    });
  });

  it("adds the selected variant directly into the active batch", async () => {
    getActiveSheinStudioBatchId.mockReturnValue("batch-1");
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "Retro Cherries",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );

    await waitFor(() => {
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          groupedSelections: [
            expect.objectContaining({
              selection: expect.objectContaining({
                variantId: 201,
              }),
            }),
          ],
        }),
        { makeActive: true },
      );
    });
  });

  it("surfaces a current target batch selector for existing batches", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
      {
        id: "batch-2",
        name: "Summer Flag",
        prompt: "summer flag",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
    ]);

    render(<SDSProductBrowser />);

    const select = await screen.findByLabelText("当前接收批次");
    expect(
      screen.getByText("先选择一个已有批次，后面选中的商品就能直接加入它。"),
    ).toBeInTheDocument();

    fireEvent.change(select, { target: { value: "batch-2" } });

    expect(setActiveSheinStudioBatchId).toHaveBeenCalledWith("batch-2");
    expect(
      screen.getByText("现在选到的商品会优先加入“Summer Flag”"),
    ).toBeInTheDocument();
  });

  it("keeps the current target batch controls mobile-safe", async () => {
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
    ]);

    render(<SDSProductBrowser />);

    const select = await screen.findByLabelText("当前接收批次");
    const controlColumn = select.parentElement as HTMLDivElement | null;
    expect(controlColumn).not.toBeNull();
    expect(controlColumn?.className).not.toContain("sm:min-w-[18rem]");
  });

  it("stays on the SDS page while adding multiple products to an existing batch", async () => {
    currentSearchParams = new URLSearchParams("targetBatchId=batch-1");
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "TEST1",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        variationIntensity: "medium",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "",
        transparentBackground: false,
        imageStrategy: "sds_official",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        renderSizeImagesWithSds: true,
        selection: {
          productId: 101,
          parentProductId: 101,
          variantId: 201,
          prototypeGroupId: 301,
          layerId: "layer-a",
          productName: "Placemat",
          variantLabel: "40x30cm · White",
        },
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-28T12:00:00Z",
      },
    ]);
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "TEST1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });
    saveSheinStudioBatch.mockResolvedValue({ id: "batch-1" });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );

    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({ id: "batch-1" }),
        { makeActive: true },
      ),
    );
    expect(push).not.toHaveBeenCalled();
    expect(
      await screen.findByRole("button", { name: "完成并返回批次（已加 1 款）" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "完成并返回批次（已加 1 款）" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });

  it("keeps the add-success flow even if refreshing recent batches fails afterwards", async () => {
    currentSearchParams = new URLSearchParams("targetBatchId=batch-1");
    listSheinStudioBatches
      .mockResolvedValueOnce([
        {
          id: "batch-1",
          name: "TEST1",
          prompt: "retro cherries",
          styleCount: "1",
          groupedSelections: [],
          designs: [],
          selectedIds: [],
          createdTasks: [],
        },
      ])
      .mockRejectedValueOnce(new Error("ListingKit API request failed: 504"));
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "TEST1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });
    saveSheinStudioBatch.mockResolvedValue({ id: "batch-1" });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );

    expect(
      await screen.findByText("已加入 1 款商品到批次 TEST1，可以继续选下一款。"),
    ).toBeInTheDocument();
    expect(screen.queryByText("ListingKit API request failed: 504")).not.toBeInTheDocument();
    expect(push).not.toHaveBeenCalled();
  });

  it("stays on the SDS page and shows the baseline reason instead of recursively retrying when grouped baseline is blocked", async () => {
    currentSearchParams = new URLSearchParams("targetBatchId=batch-1");
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "TEST1",
        prompt: "retro cherries",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
    ]);
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "TEST1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });
    getSDSBaselineReadiness.mockResolvedValue({
      status: "failed",
      reason: "Baseline cache is not usable for grouped SDS create.",
      reasonCode: "cache_unavailable",
    });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );

    expect(
      await screen.findByText(
        "这款商品还没有加入当前批次。Baseline cache is not usable for grouped SDS create.",
      ),
    ).toBeInTheDocument();
    expect(push).not.toHaveBeenCalled();
    expect(saveSheinStudioBatch).not.toHaveBeenCalled();
    expect(getSheinStudioBatch).toHaveBeenCalledTimes(1);
    expect(getSDSBaselineReadiness).toHaveBeenCalledTimes(1);
    expect(saveSDSGroupedCandidateHandoff).toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: "一键预热并校验 baseline" }),
    ).toBeInTheDocument();
  });

  it("warms a missing baseline and then adds the product into the current batch without leaving the page", async () => {
    currentSearchParams = new URLSearchParams("targetBatchId=batch-1");
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "TEST1",
        prompt: "retro cherries",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
    ]);
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "TEST1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });
    getSDSBaselineReadiness.mockResolvedValue({
      status: "failed",
      reason: "No baseline cache entry exists for this SDS selection.",
      reasonCode: "cache_missing",
    });
    warmSDSBaselineForSelection.mockResolvedValue({
      status: "ready",
      reason: "",
      baselineKey: "baseline-key",
    });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "一键预热并校验 baseline" }),
    );

    await waitFor(() =>
      expect(warmSDSBaselineForSelection).toHaveBeenCalledWith(
        expect.objectContaining({
          variantId: 201,
        }),
      ),
    );
    await waitFor(() =>
      expect(saveSheinStudioBatch).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "batch-1",
          groupedSelections: [
            expect.objectContaining({
              selection: expect.objectContaining({
                variantId: 201,
              }),
            }),
          ],
        }),
        { makeActive: true },
      ),
    );
    expect(push).not.toHaveBeenCalled();
    expect(
      await screen.findByText("已加入 1 款商品到批次 TEST1，可以继续选下一款。"),
    ).toBeInTheDocument();
  });

  it("opens the retirement review flow when SHEIN reports the SDS product is already off shelf", async () => {
    currentSearchParams = new URLSearchParams("targetBatchId=batch-1");
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "TEST1",
        prompt: "retro cherries",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
    ]);
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "TEST1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });
    getSDSBaselineReadiness.mockResolvedValue({
      status: "blocked",
      reason: "SDS product detail check failed: 产品已下架",
      reasonCode: "product_detail_check_failed",
    });
    createSDSRetirementRun.mockResolvedValue({
      run: {
        id: "run-1",
        tenant_id: "tenant-a",
        platform: "shein",
        store_id: 869,
        parent_product_id: 101,
        prototype_group_id: 301,
        variant_id: 201,
        status: "ready",
        reason_code: "product_detail_check_failed",
        reason: "SDS product detail check failed: 产品已下架",
      },
      items: [
        {
          id: "item-1",
          run_id: "run-1",
          platform: "shein",
          store_id: 869,
          spu_name: "Placemat",
          skc_name: "SKC-1",
          selected: true,
          site_selection:
            '[{"site_abbr":"US","store_type":1},{"site_abbr":"CA","store_type":1}]',
          status: "selected",
        },
      ],
    });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );

    expect(await screen.findByText("SDS 底版下架处置")).toBeInTheDocument();
    expect(createSDSRetirementRun).toHaveBeenCalledWith({
      platform: "shein",
      store_id: 869,
      parent_product_id: 101,
      prototype_group_id: 301,
      variant_id: 201,
      selected_variant_ids: [201],
    });

    const confirmButton = screen.getByRole("button", { name: /确认下架/ });
    expect(confirmButton).toBeDisabled();

    fireEvent.click(screen.getByLabelText(/我确认所选 SHEIN 商品将执行下架操作/));
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(confirmSDSRetirementRun).toHaveBeenCalledWith("run-1");
    });
  });

  it("updates retirement selection when an item is deselected", async () => {
    currentSearchParams = new URLSearchParams("targetBatchId=batch-1");
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "TEST1",
        prompt: "retro cherries",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
    ]);
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "TEST1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });
    getSDSBaselineReadiness.mockResolvedValue({
      status: "blocked",
      reason: "SDS product detail check failed: 产品已下架",
      reasonCode: "product_detail_check_failed",
    });
    createSDSRetirementRun.mockResolvedValue({
      run: {
        id: "run-1",
        tenant_id: "tenant-a",
        platform: "shein",
        store_id: 869,
        parent_product_id: 101,
        prototype_group_id: 301,
        variant_id: 201,
        status: "ready",
        reason_code: "product_detail_check_failed",
        reason: "SDS product detail check failed: 产品已下架",
      },
      items: [
        {
          id: "item-1",
          run_id: "run-1",
          platform: "shein",
          store_id: 869,
          spu_name: "Placemat",
          skc_name: "SKC-1",
          selected: true,
          site_selection:
            '[{"site_abbr":"US","store_type":1},{"site_abbr":"CA","store_type":1}]',
          status: "selected",
        },
      ],
    });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );
    fireEvent.click(await screen.findByLabelText("SKC-1"));

    await waitFor(() =>
      expect(updateSDSRetirementSelection).toHaveBeenCalledWith("run-1", [
        {
          item_id: "item-1",
          selected: false,
          site_selection:
            '[{"site_abbr":"US","store_type":1},{"site_abbr":"CA","store_type":1}]',
        },
      ]),
    );
  });

  it("updates retirement selection when a site toggle changes before confirm", async () => {
    currentSearchParams = new URLSearchParams("targetBatchId=batch-1");
    listSheinStudioBatches.mockResolvedValue([
      {
        id: "batch-1",
        name: "TEST1",
        prompt: "retro cherries",
        styleCount: "1",
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
      },
    ]);
    getSheinStudioBatch.mockResolvedValue({
      id: "batch-1",
      name: "TEST1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 101,
        parentProductId: 101,
        variantId: 201,
        prototypeGroupId: 301,
        layerId: "layer-a",
        productName: "Placemat",
        variantLabel: "40x30cm · White",
      },
      groupedSelections: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-28T12:00:00Z",
    });
    getSDSBaselineReadiness.mockResolvedValue({
      status: "blocked",
      reason: "SDS product detail check failed: 产品已下架",
      reasonCode: "product_detail_check_failed",
    });
    createSDSRetirementRun.mockResolvedValue({
      run: {
        id: "run-1",
        tenant_id: "tenant-a",
        platform: "shein",
        store_id: 869,
        parent_product_id: 101,
        prototype_group_id: 301,
        variant_id: 201,
        status: "ready",
        reason_code: "product_detail_check_failed",
        reason: "SDS product detail check failed: 产品已下架",
      },
      items: [
        {
          id: "item-1",
          run_id: "run-1",
          platform: "shein",
          store_id: 869,
          spu_name: "Placemat",
          skc_name: "SKC-1",
          selected: true,
          site_selection:
            '[{"site_abbr":"US","store_type":1},{"site_abbr":"CA","store_type":1}]',
          status: "selected",
        },
      ],
    });

    render(<SDSProductBrowser />);

    fireEvent.click(screen.getByRole("button", { name: "open variants" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "add selected variants to current batch",
      }),
    );
    fireEvent.click(await screen.findByLabelText("SKC-1 US"));

    await waitFor(() =>
      expect(updateSDSRetirementSelection).toHaveBeenCalledWith("run-1", [
        {
          item_id: "item-1",
          selected: true,
          site_selection: '[{"site_abbr":"CA","store_type":1}]',
        },
      ]),
    );
  });
});
