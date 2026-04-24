import { render, screen } from "@testing-library/react";

import { SlotNavigationList } from "@/components/listingkit/slot-navigation-list";

describe("SlotNavigationList", () => {
  it("renders duplicate slot names without React key collisions", () => {
    render(
      <SlotNavigationList
        slots={[
          {
            platform: "shein",
            slot: "auxiliary",
            asset_id: "asset-1",
            purpose: "selling_point",
            template_label: "SHEIN Selling Point",
            quality_grade_label: "Ready",
            render_preview_available: true,
          },
          {
            platform: "shein",
            slot: "auxiliary",
            asset_id: "asset-2",
            purpose: "size_scene",
            template_label: "SHEIN Size Scene",
            quality_grade_label: "Fallback",
            render_preview_available: false,
          },
        ]}
        onSelect={() => {}}
      />,
    );

    expect(screen.getByText("SHEIN Selling Point")).toBeInTheDocument();
    expect(screen.getByText("SHEIN Size Scene")).toBeInTheDocument();
    expect(screen.getByText("auxiliary / selling_point")).toBeInTheDocument();
    expect(screen.getByText("auxiliary / size_scene")).toBeInTheDocument();
    expect(screen.getByText("Ready")).toBeInTheDocument();
    expect(screen.getByText("Fallback")).toBeInTheDocument();
  });

  it("highlights the selected asset when duplicate slots share the same slot key", () => {
    const { container } = render(
      <SlotNavigationList
        slots={[
          {
            platform: "shein",
            slot: "auxiliary",
            asset_id: "asset-1",
            purpose: "selling_point",
            template_label: "SHEIN Selling Point",
            quality_grade_label: "Ready",
            render_preview_available: true,
          },
          {
            platform: "shein",
            slot: "auxiliary",
            asset_id: "asset-2",
            purpose: "size_scene",
            template_label: "SHEIN Size Scene",
            quality_grade_label: "Fallback",
            render_preview_available: false,
          },
        ]}
        selectedSlot="auxiliary"
        selectedAssetId="asset-2"
        onSelect={() => {}}
      />,
    );

    const selectedCards = container.querySelectorAll(".border-zinc-950");
    expect(selectedCards).toHaveLength(1);
    expect(screen.getByText("SHEIN Size Scene").closest(".border-zinc-950")).not.toBeNull();
  });
});
