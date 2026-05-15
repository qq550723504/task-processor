import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SubscriptionPage } from "@/components/listingkit/subscription/subscription-page";
import {
  getCurrentSubscription,
  updateSubscriptionEntitlement,
} from "@/lib/api/subscription";

vi.mock("@/lib/api/subscription", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/lib/api/subscription")>();
  return {
    ...actual,
    getCurrentSubscription: vi.fn(),
    updateSubscriptionEntitlement: vi.fn(),
  };
});

const mockedGetCurrentSubscription = vi.mocked(getCurrentSubscription);
const mockedUpdateSubscriptionEntitlement = vi.mocked(
  updateSubscriptionEntitlement,
);

describe("SubscriptionPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedGetCurrentSubscription.mockResolvedValue({
      tenant_id: "org-286",
      modules: [
        {
          code: "studio",
          name: "Studio",
          description: "Design jobs",
          sort_order: 50,
          active: true,
        },
        {
          code: "oss_storage",
          name: "OSS 存储",
          description: "Storage",
          sort_order: 60,
          active: true,
        },
      ],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            description: "Design jobs",
            sort_order: 50,
            active: true,
          },
          entitlement: {
            id: 1,
            tenant_id: "org-286",
            module_code: "studio",
            status: "active",
            limits: { design_jobs: 10 },
          },
          usage: [],
          allowed: true,
          limits: { design_jobs: 10 },
          used: { design_jobs: 2 },
        },
        {
          module: {
            code: "oss_storage",
            name: "OSS 存储",
            description: "Storage",
            sort_order: 60,
            active: true,
          },
          entitlement: {
            id: 2,
            tenant_id: "org-286",
            module_code: "oss_storage",
            status: "active",
            limits: { storage_bytes: 1048576 },
          },
          usage: [],
          allowed: true,
          limits: { storage_bytes: 1048576 },
          used: { storage_bytes: 2048 },
        },
      ],
    });
    mockedUpdateSubscriptionEntitlement.mockResolvedValue({
      id: 1,
      tenant_id: "org-286",
      module_code: "studio",
      status: "active",
      limits: { design_jobs: 10 },
    });
  });

  it("renders current tenant subscription as read-only", async () => {
    renderWithQueryClient(<SubscriptionPage />);

    expect(await screen.findByText("Studio")).toBeInTheDocument();
    expect(screen.getAllByText("已开通").length).toBeGreaterThan(0);
    expect(screen.getByText("design_jobs: 10")).toBeInTheDocument();
    expect(screen.getByText("design_jobs: 2")).toBeInTheDocument();
    expect(screen.getByText("storage_bytes: 1 MB")).toBeInTheDocument();
    expect(screen.getByText("storage_bytes: 2 KB")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "配置" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存配置" })).not.toBeInTheDocument();
    expect(mockedUpdateSubscriptionEntitlement).not.toHaveBeenCalled();
  });
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}
