import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SDSConsoleMetrics } from "@/components/listingkit/sds/sds-console-metrics";

vi.mock("@/lib/utils/live-search-params", () => ({
  useLiveSearchParams: () => new URLSearchParams("shipmentArea=US&variantId=123&prototypeGroupId=456"),
}));

describe("SDSConsoleMetrics", () => {
  it("uses a progressive responsive metric grid", () => {
    const { container } = render(
      <SDSConsoleMetrics
        initialShipmentArea="CN"
        initialVariantId=""
        initialPrototypeGroupId=""
      />,
    );

    expect(screen.getByText("US")).toBeInTheDocument();
    const grid = container.querySelector(".grid.gap-3") as HTMLDivElement | null;
    expect(grid).not.toBeNull();
    expect(grid?.className).not.toContain("sm:grid-cols-3");
    expect(grid?.className).not.toContain("md:grid-cols-1");
  });
});
