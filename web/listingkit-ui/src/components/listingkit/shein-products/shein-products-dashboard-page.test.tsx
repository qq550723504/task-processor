import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinProductsDashboardPage } from "@/components/listingkit/shein-products/shein-products-dashboard-page";

const mocks = vi.hoisted(() => ({
  useSheinEnrollmentDashboard: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinEnrollmentDashboard: (...args: unknown[]) =>
    mocks.useSheinEnrollmentDashboard(...args),
}));

describe("SheinProductsDashboardPage", () => {
  it("opens store cards on the product sync workbench route", async () => {
    mocks.useSheinEnrollmentDashboard.mockReturnValue({
      isLoading: false,
      isError: false,
      error: null,
      data: {
        items: [
          {
            store_id: 7,
            store_name: "SHEIN US",
            store_username: "shein-us",
            region: "US",
            synced_product_count: 12,
            missing_cost_count: 3,
          },
        ],
      },
    });

    render(<SheinProductsDashboardPage />);

    expect(await screen.findByText("SHEIN Synced Products")).toBeInTheDocument();
    expect(screen.getByText("SHEIN US")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View Products" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-products/7",
    );
    expect(screen.queryByRole("link", { name: "Login" })).not.toBeInTheDocument();
  });
});
