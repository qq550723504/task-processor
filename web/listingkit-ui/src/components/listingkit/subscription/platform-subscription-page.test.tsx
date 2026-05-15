import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactElement } from "react";

import { PlatformSubscriptionPage } from "@/components/listingkit/subscription/platform-subscription-page";
import {
  getPlatformTenantSubscriptionAuditLogs,
  getPlatformTenantSubscriptions,
  getPlatformTenantSubscription,
  updatePlatformTenantSubscriptionUsage,
  updatePlatformTenantSubscriptionEntitlement,
} from "@/lib/api/subscription";

vi.mock("@/lib/api/subscription", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/lib/api/subscription")>();
  return {
    ...actual,
    getPlatformTenantSubscriptionAuditLogs: vi.fn(),
    getPlatformTenantSubscriptions: vi.fn(),
    getPlatformTenantSubscription: vi.fn(),
    updatePlatformTenantSubscriptionUsage: vi.fn(),
    updatePlatformTenantSubscriptionEntitlement: vi.fn(),
  };
});

const mockedGetPlatformTenantSubscriptionAuditLogs = vi.mocked(
  getPlatformTenantSubscriptionAuditLogs,
);
const mockedGetPlatformTenantSubscriptions = vi.mocked(
  getPlatformTenantSubscriptions,
);
const mockedGetPlatformTenantSubscription = vi.mocked(getPlatformTenantSubscription);
const mockedUpdatePlatformTenantSubscriptionUsage = vi.mocked(
  updatePlatformTenantSubscriptionUsage,
);
const mockedUpdatePlatformTenantSubscriptionEntitlement = vi.mocked(
  updatePlatformTenantSubscriptionEntitlement,
);

describe("PlatformSubscriptionPage", () => {
  beforeEach(() => {
    mockedGetPlatformTenantSubscriptionAuditLogs.mockReset();
    mockedGetPlatformTenantSubscriptions.mockReset();
    mockedGetPlatformTenantSubscription.mockReset();
    mockedUpdatePlatformTenantSubscriptionUsage.mockReset();
    mockedUpdatePlatformTenantSubscriptionEntitlement.mockReset();
    mockedGetPlatformTenantSubscriptionAuditLogs.mockResolvedValue([]);
    mockedGetPlatformTenantSubscriptions.mockResolvedValue([
      {
        tenant_id: "org-target",
        entitlement_count: 1,
        active_count: 0,
      },
    ]);
  });

  it("loads a tenant subscription and updates a module entitlement", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            sort_order: 50,
            active: true,
          },
          usage: [],
          allowed: false,
          reason: "not_configured",
          used: {},
          limits: {},
        },
      ],
    });
    mockedUpdatePlatformTenantSubscriptionEntitlement.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      module_code: "studio",
      status: "active",
      limits: { design_jobs: 10 },
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("ZITADEL resource owner id"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    expect(await screen.findByText("Studio")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.change(screen.getByLabelText("额度 JSON"), {
      target: { value: "{\"design_jobs\":10}" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformTenantSubscriptionEntitlement).toHaveBeenCalledWith(
        "org-target",
        "studio",
        expect.objectContaining({
          status: "active",
          limits: { design_jobs: 10 },
        }),
      );
    });
  });

  it("loads a tenant from the configured tenant list", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.click(await screen.findByText("org-target"));

    await waitFor(() => {
      expect(mockedGetPlatformTenantSubscription).toHaveBeenCalledWith(
        "org-target",
      );
    });
  });

  it("adjusts module usage for a billing period", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            sort_order: 50,
            active: true,
          },
          entitlement: {
            id: 1,
            tenant_id: "org-target",
            module_code: "studio",
            status: "active",
            limits: { design_jobs: 10 },
          },
          usage: [
            {
              id: 1,
              tenant_id: "org-target",
              module_code: "studio",
              period_key: "2026-05",
              metric: "design_jobs",
              used: 4,
            },
          ],
          allowed: true,
          used: { design_jobs: 4 },
          limits: { design_jobs: 10 },
        },
      ],
    });
    mockedUpdatePlatformTenantSubscriptionUsage.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      module_code: "studio",
      period_key: "2026-05",
      metric: "design_jobs",
      used: 0,
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("ZITADEL resource owner id"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("Studio");
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByRole("button", { name: "重置为 0" }));
    fireEvent.click(screen.getByRole("button", { name: "保存用量" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformTenantSubscriptionUsage).toHaveBeenCalledWith(
        "org-target",
        "studio",
        expect.objectContaining({
          metric: "design_jobs",
          period_key: "2026-05",
          used: 0,
        }),
      );
    });
  });
});

function renderWithQueryClient(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}
