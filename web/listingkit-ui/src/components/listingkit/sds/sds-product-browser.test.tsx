import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SDSProductBrowser } from "@/components/listingkit/sds/sds-product-browser";

const push = vi.fn();
const saveSheinStudioBatch = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds/new",
  useRouter: () => ({ push }),
}));

vi.mock("@/lib/utils/live-search-params", () => ({
  useLiveSearchParams: () => new URLSearchParams(),
}));

vi.mock("@/lib/query/use-sds-recent-variants", () => ({
  useSDSRecentVariants: () => [],
}));

vi.mock("@/lib/query/use-sds-grouped-candidates", () => ({
  useSDSGroupedCandidates: () => [],
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
  getActiveSheinStudioBatchId: () => "",
  listSheinStudioBatches: () => Promise.resolve([]),
  saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
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
  getSDSBaselineReadiness: vi.fn(),
  warmSDSBaselineForSelection: vi.fn(),
}));

vi.mock("@/lib/utils/sds-grouped-candidates", () => ({
  hasSDSGroupedCandidate: () => false,
  removeSDSGroupedCandidate: vi.fn(),
  saveSDSGroupedCandidate: vi.fn(),
}));

vi.mock("@/lib/utils/sds-grouped-candidate-handoff", () => ({
  saveSDSGroupedCandidateHandoff: vi.fn(),
}));

vi.mock("@/components/listingkit/sds/sds-product-browser-filters", () => ({
  SDSProductBrowserFilters: () => <div>filters</div>,
}));
vi.mock("@/components/listingkit/sds/sds-recent-variants", () => ({
  SDSRecentVariants: () => <div>recent variants</div>,
}));
vi.mock("@/components/listingkit/sds/sds-grouped-candidates-panel", () => ({
  SDSGroupedCandidatesPanel: () => <div>grouped candidates</div>,
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
    onSelectVariants,
    open,
    variants,
  }: {
    onSelectVariants: (primary: { id: number }, variants: Array<{ id: number }>) => void;
    open: boolean;
    variants: Array<{ id: number }>;
  }) =>
    open ? (
      <button
        onClick={() => onSelectVariants(variants[0], variants)}
        type="button"
      >
        use selected variants
      </button>
    ) : null,
}));

describe("SDSProductBrowser", () => {
  beforeEach(() => {
    push.mockReset();
    saveSheinStudioBatch.mockReset();
    saveSheinStudioBatch.mockResolvedValue({ id: "batch-new" });
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
});
