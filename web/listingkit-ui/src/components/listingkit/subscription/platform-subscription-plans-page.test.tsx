import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PlatformSubscriptionPlansPage } from "@/components/listingkit/subscription/platform-subscription-plans-page";
import {
  deletePlatformSubscriptionPlanModule,
  getPlatformSubscriptionPlans,
  getSubscriptionModules,
  setPlatformSubscriptionPlanStatus,
  updatePlatformSubscriptionPlanModule,
  upsertPlatformSubscriptionPlan,
} from "@/lib/api/subscription";

vi.mock("@/lib/api/subscription", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/lib/api/subscription")>();
  return {
    ...actual,
    deletePlatformSubscriptionPlanModule: vi.fn(),
    getPlatformSubscriptionPlans: vi.fn(),
    getSubscriptionModules: vi.fn(),
    setPlatformSubscriptionPlanStatus: vi.fn(),
    updatePlatformSubscriptionPlanModule: vi.fn(),
    upsertPlatformSubscriptionPlan: vi.fn(),
  };
});

const mockedDeletePlatformSubscriptionPlanModule = vi.mocked(
  deletePlatformSubscriptionPlanModule,
);
const mockedGetPlatformSubscriptionPlans = vi.mocked(getPlatformSubscriptionPlans);
const mockedGetSubscriptionModules = vi.mocked(getSubscriptionModules);
const mockedSetPlatformSubscriptionPlanStatus = vi.mocked(
  setPlatformSubscriptionPlanStatus,
);
const mockedUpdatePlatformSubscriptionPlanModule = vi.mocked(
  updatePlatformSubscriptionPlanModule,
);
const mockedUpsertPlatformSubscriptionPlan = vi.mocked(
  upsertPlatformSubscriptionPlan,
);

describe("PlatformSubscriptionPlansPage", () => {
  beforeEach(() => {
    mockedDeletePlatformSubscriptionPlanModule.mockReset();
    mockedGetPlatformSubscriptionPlans.mockReset();
    mockedGetSubscriptionModules.mockReset();
    mockedSetPlatformSubscriptionPlanStatus.mockReset();
    mockedUpdatePlatformSubscriptionPlanModule.mockReset();
    mockedUpsertPlatformSubscriptionPlan.mockReset();
    mockedGetSubscriptionModules.mockResolvedValue([
      {
        code: "studio",
        name: "Studio",
        sort_order: 50,
        active: true,
      },
      {
        code: "oss_storage",
        name: "OSS 存储",
        sort_order: 60,
        active: true,
      },
    ]);
    mockedGetPlatformSubscriptionPlans.mockResolvedValue([
      {
        plan: {
          code: "professional",
          name: "专业版",
          description: "Studio 和 OSS",
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
  });

  it("creates a plan and refreshes the plan list", async () => {
    mockedUpsertPlatformSubscriptionPlan.mockResolvedValue({
      plan: {
        code: "growth",
        name: "增长版",
        sort_order: 25,
        active: true,
      },
      modules: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPlansPage />);

    fireEvent.change(screen.getByLabelText("套餐编码"), {
      target: { value: "growth" },
    });
    fireEvent.change(screen.getByLabelText("套餐名称"), {
      target: { value: "增长版" },
    });
    fireEvent.change(screen.getByLabelText("排序"), {
      target: { value: "25" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存套餐" }));

    await waitFor(() => {
      expect(mockedUpsertPlatformSubscriptionPlan).toHaveBeenCalledWith(
        expect.objectContaining({
          code: "growth",
          name: "增长版",
          sort_order: 25,
          active: true,
        }),
      );
    });
  });

  it("adds and deletes modules on a plan", async () => {
    mockedUpdatePlatformSubscriptionPlanModule.mockResolvedValue({
      plan: {
        code: "professional",
        name: "专业版",
        sort_order: 20,
        active: true,
      },
      modules: [
        {
          plan_code: "professional",
          module_code: "oss_storage",
          limits: { storage_bytes: 10737418240 },
          sort_order: 60,
        },
      ],
    });
    mockedDeletePlatformSubscriptionPlanModule.mockResolvedValue({
      plan: {
        code: "professional",
        name: "专业版",
        sort_order: 20,
        active: true,
      },
      modules: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPlansPage />);

    await screen.findByText("专业版");
    fireEvent.click(screen.getByRole("button", { name: "编辑 professional" }));
    fireEvent.change(screen.getByLabelText("模块"), {
      target: { value: "oss_storage" },
    });
    fireEvent.change(screen.getByLabelText("模块额度 JSON"), {
      target: { value: "{\"storage_bytes\":10737418240}" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存模块" }));

    await waitFor(() => {
      expect(mockedUpdatePlatformSubscriptionPlanModule).toHaveBeenCalledWith(
        "professional",
        "oss_storage",
        expect.objectContaining({
          limits: { storage_bytes: 10737418240 },
        }),
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "移除 studio" }));
    await waitFor(() => {
      expect(mockedDeletePlatformSubscriptionPlanModule).toHaveBeenCalledWith(
        "professional",
        "studio",
      );
    });
  });

  it("disables a plan", async () => {
    mockedSetPlatformSubscriptionPlanStatus.mockResolvedValue({
      plan: {
        code: "professional",
        name: "专业版",
        sort_order: 20,
        active: false,
      },
      modules: [],
    });

    renderWithQueryClient(<PlatformSubscriptionPlansPage />);

    await screen.findByText("专业版");
    fireEvent.click(screen.getByRole("button", { name: "禁用 professional" }));

    await waitFor(() => {
      expect(mockedSetPlatformSubscriptionPlanStatus).toHaveBeenCalledWith(
        "professional",
        false,
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
