import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinEnrollmentDashboardPage } from "@/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page";

const mocks = vi.hoisted(() => ({
  getTenantListingStores: vi.fn(),
  useSheinLoginAccounts: vi.fn(),
}));

vi.mock("@/lib/api/tenant-stores", () => ({
  getTenantListingStores: (...args: unknown[]) => mocks.getTenantListingStores(...args),
}));

vi.mock("@/lib/query/use-shein-login", () => ({
  useSheinLoginAccounts: () => mocks.useSheinLoginAccounts(),
}));

describe("SheinEnrollmentDashboardPage", () => {
  it("renders shein store workbench entries", async () => {
    mocks.getTenantListingStores.mockResolvedValue({
      items: [
        {
          id: 7,
          name: "SHEIN US",
          username: "shein-us",
          platform: "SHEIN",
          shopType: "0",
          region: "US",
          enableAutoListing: true,
        },
      ],
      total: 1,
      page: 1,
      page_size: 100,
    });
    mocks.useSheinLoginAccounts.mockReturnValue({
      data: [
        {
          account: { store_id: 7 },
          has_cookie: true,
        },
      ],
    });

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <SheinEnrollmentDashboardPage />
      </QueryClientProvider>,
    );

    expect(await screen.findByText("SHEIN 活动报名")).toBeInTheDocument();
    expect(await screen.findByText("SHEIN US")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "进入工作台" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-enrollment/7",
    );
  });
});
