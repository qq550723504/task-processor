import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SDSRetirementPanel } from "@/components/listingkit/sds/sds-retirement-panel";
import type {
  SDSRetirementRunDetail,
  SDSRetirementSelectionUpdate,
} from "@/lib/types/sds-retirement";

const detail: SDSRetirementRunDetail = {
  run: {
    id: "run-1",
    tenant_id: "tenant-a",
    platform: "shein",
    store_id: 177,
    parent_product_id: 238915,
    prototype_group_id: 28345,
    variant_id: 238916,
    status: "ready",
    reason_code: "product_detail_check_failed",
    reason: "SDS product detail check failed: 产品已下架",
  },
  items: [
    {
      id: "item-1",
      run_id: "run-1",
      platform: "shein",
      store_id: 177,
      spu_name: "SPU-1",
      skc_name: "SKC-1",
      selected: true,
      site_selection: JSON.stringify([{ site_abbr: "US", store_type: 1 }]),
      status: "selected",
    },
  ],
};

function PanelHarness({
  initialDetail = detail,
  onSelectionChange = vi.fn(),
}: {
  initialDetail?: SDSRetirementRunDetail;
  onSelectionChange?: (items: SDSRetirementSelectionUpdate[]) => void;
}) {
  const [currentDetail, setCurrentDetail] = useState(initialDetail);

  return (
    <SDSRetirementPanel
      detail={currentDetail}
      isExecuting={false}
      onConfirm={() => undefined}
      onSelectionChange={(items) => {
        onSelectionChange(items);
        setCurrentDetail((current) => ({
          ...current,
          items: current.items.map((item) => {
            const update = items.find((candidate) => candidate.item_id === item.id);
            if (!update) {
              return item;
            }
            return {
              ...item,
              selected: update.selected,
              site_selection: update.site_selection,
              status: update.selected ? "selected" : "pending",
            };
          }),
        }));
      }}
    />
  );
}

describe("SDSRetirementPanel", () => {
  it("renders selected SKCs and asks for explicit acknowledgement before confirm", () => {
    const onSelectionChange = vi.fn();
    const onConfirm = vi.fn();

    render(
      <SDSRetirementPanel
        detail={detail}
        isExecuting={false}
        onConfirm={onConfirm}
        onSelectionChange={onSelectionChange}
      />,
    );

    expect(screen.getByText("SKC-1")).toBeInTheDocument();
    expect(screen.getByText(/产品已下架/)).toBeInTheDocument();

    const button = screen.getByRole("button", { name: /确认下架/ });
    expect(button).toBeDisabled();

    fireEvent.click(screen.getByLabelText(/我确认/));

    expect(button).not.toBeDisabled();

    fireEvent.click(button);

    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("keeps candidate sites renderable so a site can be unchecked and rechecked", () => {
    const onSelectionChange = vi.fn();
    const detailWithTwoSites: SDSRetirementRunDetail = {
      ...detail,
      items: [
        {
          ...detail.items[0],
          site_selection: JSON.stringify([
            { site_abbr: "US", store_type: 1 },
            { site_abbr: "CA", store_type: 1 },
          ]),
        },
      ],
    };

    render(
      <PanelHarness
        initialDetail={detailWithTwoSites}
        onSelectionChange={onSelectionChange}
      />,
    );

    const usSite = screen.getByLabelText("SKC-1 US");
    expect(usSite).toBeChecked();

    fireEvent.click(usSite);

    expect(onSelectionChange).toHaveBeenLastCalledWith([
      {
        item_id: "item-1",
        selected: true,
        site_selection: JSON.stringify([{ site_abbr: "CA", store_type: 1 }]),
      },
    ]);
    expect(screen.getByLabelText("SKC-1 US")).not.toBeChecked();

    fireEvent.click(screen.getByLabelText("SKC-1 US"));

    expect(onSelectionChange).toHaveBeenLastCalledWith([
      {
        item_id: "item-1",
        selected: true,
        site_selection: JSON.stringify([
          { site_abbr: "CA", store_type: 1 },
          { site_abbr: "US", store_type: 1 },
        ]),
      },
    ]);
    expect(screen.getByLabelText("SKC-1 US")).toBeChecked();
  });
});
