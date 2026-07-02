import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinEnrollmentDashboardPage } from "@/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page";

const mocks = vi.hoisted(() => ({
  useSheinEnrollmentDashboard: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinEnrollmentDashboard: (...args: unknown[]) =>
    mocks.useSheinEnrollmentDashboard(...args),
}));

describe("SheinEnrollmentDashboardPage", () => {
  it("renders shein store workbench entries", async () => {
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
            pending_review_count: 2,
            ready_to_enroll_count: 5,
          },
        ],
      },
    });

    render(<SheinEnrollmentDashboardPage />);

    expect(await screen.findByText("SHEIN Activity Enrollment")).toBeInTheDocument();
    expect(await screen.findByText("SHEIN US")).toBeInTheDocument();
    expect(screen.getByText("Store ID: 7")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Workbench" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-enrollment/7",
    );
    expect(screen.queryByRole("link", { name: "Login" })).not.toBeInTheDocument();
  });

  it("renders an explicit error state when the dashboard request fails", async () => {
    mocks.useSheinEnrollmentDashboard.mockReturnValue({
      isLoading: false,
      isError: true,
      error: new Error("ListingKit upstream request timed out after 120000ms"),
      data: undefined,
    });

    render(<SheinEnrollmentDashboardPage />);

    expect(
      await screen.findByText("ListingKit upstream request timed out after 120000ms"),
    ).toBeInTheDocument();
  });
});
