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

  it("applies OSS storage limit presets as byte limits", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "oss_storage",
            name: "OSS 存储",
            sort_order: 60,
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
      module_code: "oss_storage",
      status: "active",
      limits: { storage_bytes: 1073741824 },
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("ZITADEL resource owner id"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("OSS 存储");
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByRole("button", { name: "1 GB" }));
    fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformTenantSubscriptionEntitlement).toHaveBeenCalledWith(
        "org-target",
        "oss_storage",
        expect.objectContaining({
          limits: { storage_bytes: 1073741824 },
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
            code: "oss_storage",
            name: "OSS 存储",
            sort_order: 60,
            active: true,
          },
          entitlement: {
            id: 1,
            tenant_id: "org-target",
            module_code: "oss_storage",
            status: "active",
            limits: { storage_bytes: 1048576 },
          },
          usage: [
            {
              id: 1,
              tenant_id: "org-target",
              module_code: "oss_storage",
              period_key: "2026-05",
              metric: "storage_bytes",
              used: 2048,
            },
          ],
          allowed: true,
          used: { storage_bytes: 2048 },
          limits: { storage_bytes: 1048576 },
        },
      ],
    });
    mockedUpdatePlatformTenantSubscriptionUsage.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      module_code: "oss_storage",
      period_key: "2026-05",
      metric: "storage_bytes",
      used: 0,
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("ZITADEL resource owner id"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("OSS 存储");
    expect(screen.getByText("storage_bytes: 1 MB")).toBeInTheDocument();
    expect(screen.getByText("storage_bytes: 2 KB")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByRole("button", { name: "重置为 0" }));
    fireEvent.click(screen.getByRole("button", { name: "保存用量" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformTenantSubscriptionUsage).toHaveBeenCalledWith(
        "org-target",
        "oss_storage",
        expect.objectContaining({
          metric: "storage_bytes",
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
