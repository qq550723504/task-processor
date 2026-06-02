import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinSavedBatchesPanel } from "@/components/listingkit/shein-studio/shein-saved-batches-panel";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

describe("SheinSavedBatchesPanel", () => {
  it("opens saved batches in the SDS workbench route", () => {
    render(
      <SheinSavedBatchesPanel
        batches={[
          {
            id: "batch-1",
            name: "Batch 1",
            prompt: "retro cherries",
            styleCount: "1",
            sheinStoreId: "1",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 100,
              prototypeGroupId: 200,
              layerId: "layer-1",
              productName: "tee",
              variantLabel: "M / black",
            },
            groupedSelections: [],
            groups: [],
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-06-02T00:00:00.000Z",
          },
        ]}
        onDelete={vi.fn()}
        onLoad={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "打开批次" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });
});
