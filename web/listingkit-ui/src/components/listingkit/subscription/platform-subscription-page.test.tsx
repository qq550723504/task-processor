import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactElement } from "react";

import { PlatformSubscriptionPage } from "@/components/listingkit/subscription/platform-subscription-page";
import {
  applyPlatformTenantSubscriptionPlan,
  getPlatformSubscriptionPlans,
  getPlatformTenantSubscriptionAuditLogs,
  getPlatformTenantDirectory,
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
    applyPlatformTenantSubscriptionPlan: vi.fn(),
    getPlatformSubscriptionPlans: vi.fn(),
    getPlatformTenantSubscriptionAuditLogs: vi.fn(),
    getPlatformTenantDirectory: vi.fn(),
    getPlatformTenantSubscriptions: vi.fn(),
    getPlatformTenantSubscription: vi.fn(),
    updatePlatformTenantSubscriptionUsage: vi.fn(),
    updatePlatformTenantSubscriptionEntitlement: vi.fn(),
  };
});

const mockedApplyPlatformTenantSubscriptionPlan = vi.mocked(
  applyPlatformTenantSubscriptionPlan,
);
const mockedGetPlatformSubscriptionPlans = vi.mocked(getPlatformSubscriptionPlans);
const mockedGetPlatformTenantSubscriptionAuditLogs = vi.mocked(
  getPlatformTenantSubscriptionAuditLogs,
);
const mockedGetPlatformTenantDirectory = vi.mocked(getPlatformTenantDirectory);
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
    vi.stubGlobal("confirm", vi.fn(() => true));
    mockedGetPlatformTenantSubscriptionAuditLogs.mockReset();
    mockedGetPlatformTenantDirectory.mockReset();
    mockedApplyPlatformTenantSubscriptionPlan.mockReset();
    mockedGetPlatformSubscriptionPlans.mockReset();
    mockedGetPlatformTenantSubscriptions.mockReset();
    mockedGetPlatformTenantSubscription.mockReset();
    mockedUpdatePlatformTenantSubscriptionUsage.mockReset();
    mockedUpdatePlatformTenantSubscriptionEntitlement.mockReset();
    mockedGetPlatformSubscriptionPlans.mockResolvedValue([
      {
        plan: {
          code: "professional",
          name: "专业版",
          sort_order: 20,
          active: true,
        },
        modules: [
          {
            plan_code: "professional",
            module_code: "studio",
            limits: { design_jobs: 100 },
            sort_order: 50,
          },
        ],
      },
    ]);
    mockedGetPlatformTenantSubscriptionAuditLogs.mockResolvedValue([]);
    mockedGetPlatformTenantSubscriptions.mockResolvedValue([
      {
        tenant_id: "org-target",
        tenant_display_name: "目标租户",
        entitlement_count: 1,
        active_count: 0,
      },
    ]);
    mockedGetPlatformTenantDirectory.mockResolvedValue([
      {
        tenant_id: "org-target",
        tenant_display_name: "目标租户",
        primary_domain: "target.example",
        state: "ORGANIZATION_STATE_ACTIVE",
      },
      {
        tenant_id: "org-new",
        tenant_display_name: "新租户",
        primary_domain: "new.example",
        state: "ORGANIZATION_STATE_ACTIVE",
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

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    expect(await screen.findByText("Studio")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByRole("button", { name: "添加 设计任务额度" }));
    fireEvent.change(screen.getByLabelText("额度值 design_jobs"), {
      target: { value: "10" },
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

  it("applies a subscription plan to a tenant", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });
    mockedApplyPlatformTenantSubscriptionPlan.mockResolvedValue({
      id: 1,
      tenant_id: "org-target",
      plan_code: "professional",
      status: "active",
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("专业版");
    fireEvent.change(screen.getByLabelText("套餐"), {
      target: { value: "professional" },
    });
    fireEvent.click(screen.getByRole("button", { name: "应用套餐" }));

    await waitFor(() => {
      expect(mockedApplyPlatformTenantSubscriptionPlan).toHaveBeenCalledWith(
        "org-target",
        expect.objectContaining({
          plan_code: "professional",
          status: "active",
        }),
      );
    });
  });

  it("opens a new tenant directly from subscription management", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-new",
      modules: [],
      entitlements: [],
    });
    mockedApplyPlatformTenantSubscriptionPlan.mockResolvedValue({
      id: 2,
      tenant_id: "org-new",
      plan_code: "professional",
      status: "active",
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.click(screen.getByRole("button", { name: "开通新租户" }));
    await waitFor(() => {
      expect(screen.getByLabelText("新租户套餐").querySelectorAll("option")).toHaveLength(2);
    });
    fireEvent.change(screen.getByLabelText("新租户 ID"), {
      target: { value: "org-new" },
    });
    fireEvent.change(screen.getByLabelText("新租户套餐"), {
      target: { value: "professional" },
    });
    fireEvent.click(screen.getByRole("button", { name: "确认开通" }));

    await waitFor(() => {
      expect(mockedApplyPlatformTenantSubscriptionPlan).toHaveBeenCalledWith(
        "org-new",
        expect.objectContaining({
          plan_code: "professional",
          status: "active",
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

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
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

    fireEvent.click(await screen.findByText("目标租户"));

    await waitFor(() => {
      expect(mockedGetPlatformTenantSubscription).toHaveBeenCalledWith(
        "org-target",
      );
    });
  });

  it("filters tenants by display name and still keeps tenant id as fallback", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "目标" },
    });

    expect(await screen.findByText("目标租户")).toBeInTheDocument();
    expect(screen.getByText("org-target")).toBeInTheDocument();
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

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    await screen.findByText("OSS 存储");
    expect(screen.getByText("存储额度: 1 MB")).toBeInTheDocument();
    expect(screen.getByText("存储额度: 2 KB")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "配置" }));
    fireEvent.click(screen.getByText("高级操作：用量调整"));
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

  it("shows operator guidance and module business summary", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            description: "Design jobs",
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

    renderWithQueryClient(<PlatformSubscriptionPage />);

    expect(
      screen.getByText("可按名称、域名或租户 ID 搜索整个 ZITADEL 租户目录；开通套餐前无需先在订阅表中出现。"),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("搜索或输入租户 ID"), {
      target: { value: "org-target" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));

    expect(await screen.findByText("控制生成任务、工作台和图片生产类能力。")).toBeInTheDocument();
  });

  it("shows usage adjustment as an advanced action", () => {
    renderWithQueryClient(<PlatformSubscriptionPage />);

    expect(screen.getByText("高级操作：用量调整")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存用量" })).toBeDisabled();
  });

  it("uses mobile-first tenant search and tenant list layouts", async () => {
    mockedGetPlatformTenantSubscription.mockResolvedValue({
      tenant_id: "org-target",
      modules: [],
      entitlements: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPage />);

    await screen.findByText("目标租户");

    expect(screen.getByPlaceholderText("搜索或输入租户 ID")).not.toHaveClass("min-w-[260px]");
    expect(screen.getByRole("button", { name: "查询" })).toHaveClass("w-full");
    expect(screen.getAllByRole("button", { name: "刷新" })[0]).toHaveClass("w-full");
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
