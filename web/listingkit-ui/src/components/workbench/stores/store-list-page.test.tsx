import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

const navigation = vi.hoisted(() => ({
  pathname: "/workbench/stores",
  search: "",
  push: vi.fn(),
}));
const context = vi.hoisted(() => ({
  roles: ["listingkit_operator"],
  effectiveOrganization: { id: "org-a", name: "企业 A", roles: [] },
}));
const storesQuery = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const useWorkbenchStores = vi.hoisted(() => vi.fn(() => storesQuery.value));

vi.mock("next/navigation", () => ({
  usePathname: () => navigation.pathname,
  useRouter: () => ({ push: navigation.push }),
  useSearchParams: () => new URLSearchParams(navigation.search),
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => context,
}));
vi.mock("@/lib/query/use-workbench-stores", () => ({
  useWorkbenchStores,
}));

import { StoreListPage } from "@/components/workbench/stores/store-list-page";

const STORE = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "企业 A 店铺",
  platform: "shein" as const,
  region: "CN",
  externalStoreId: "",
  lifecycleStatus: "active" as const,
  connectionStatus: "disconnected" as const,
  version: 1,
  createdAt: "2026-08-31T00:00:00Z",
  updatedAt: "2026-08-31T01:02:03Z",
};

function listData(overrides: Record<string, unknown> = {}) {
  return {
    data: {
      items: [STORE],
      quota: { used: 4, reserved: 0, limit: 5, allowed: true, reason: "" },
      pagination: { page: 2, pageSize: 20, total: 45 },
    },
    isPending: false,
    isError: false,
    refetch: vi.fn(),
    ...overrides,
  };
}

describe("StoreListPage", () => {
  afterEach(() => {
    navigation.pathname = "/workbench/stores";
    navigation.search = "";
    navigation.push.mockReset();
    useWorkbenchStores.mockClear();
    context.roles = ["listingkit_operator"];
    context.effectiveOrganization = { id: "org-a", name: "企业 A", roles: [] };
    storesQuery.value = {};
  });

  it("normalizes URL state, exposes the server quota, and writes only allowlisted filters", async () => {
    navigation.search = "page=02&page=3&pageSize=020&platform=shein&status=active&organizationId=org-a&random=keep";
    storesQuery.value = listData();
    const user = userEvent.setup();
    render(<StoreListPage />);

    expect(screen.getByText("已使用 4 / 5")).toBeInTheDocument();
    expect(useWorkbenchStores).toHaveBeenCalledWith({
      page: 1,
      pageSize: 20,
      platform: "shein",
      status: "active",
    });
    expect(screen.getByText("第 2 页，共 45 家店铺")).toBeInTheDocument();
    await user.selectOptions(screen.getByRole("combobox", { name: "店铺状态" }), "disabled");
    expect(navigation.push).toHaveBeenCalledWith(
      "/workbench/stores?page=1&pageSize=20&platform=shein&status=disabled",
    );
    await user.click(screen.getByRole("button", { name: "清除筛选" }));
    expect(navigation.push).toHaveBeenLastCalledWith("/workbench/stores?page=1&pageSize=20");
  });

  it("renders bounded state/error variants and never retains old Organization rows", async () => {
    storesQuery.value = { isPending: true, isError: false, data: undefined, refetch: vi.fn() };
    const { rerender } = render(<StoreListPage />);
    expect(screen.getByRole("status")).toHaveTextContent("正在加载店铺");

    const retry = vi.fn();
    storesQuery.value = { isPending: false, isError: true, error: { code: "PERMISSION_DENIED", message: "raw error", requestId: "req-secret" }, refetch: retry };
    rerender(<StoreListPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("没有查看当前企业店铺的权限");
    expect(screen.queryByText(/raw error|req-secret/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "重试" }));
    expect(retry).toHaveBeenCalledTimes(1);

    storesQuery.value = listData();
    rerender(<StoreListPage />);
    expect(screen.getByText("企业 A 店铺")).toBeInTheDocument();
    context.effectiveOrganization = { id: "org-b", name: "企业 B", roles: [] };
    storesQuery.value = { isPending: true, isError: false, data: undefined, refetch: vi.fn() };
    rerender(<StoreListPage />);
    await waitFor(() => expect(screen.queryByText("企业 A 店铺")).not.toBeInTheDocument());
  });

  it("honors create roles and server quota decisions without payment links", () => {
    storesQuery.value = listData({ data: { ...listData().data, quota: { used: 5, reserved: 0, limit: 5, allowed: false, reason: "store_limit_reached" } } });
    render(<StoreListPage />);
    expect(screen.queryByRole("link", { name: "新建店铺" })).not.toBeInTheDocument();
    expect(screen.getByText(/联系管理员或升级套餐/)).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /升级|支付/ })).not.toBeInTheDocument();

    context.roles = ["listingkit_viewer"];
    storesQuery.value = listData();
    const { rerender } = render(<StoreListPage />);
    rerender(<StoreListPage />);
    expect(screen.queryByRole("link", { name: "新建店铺" })).not.toBeInTheDocument();
  });

  it("shows the create action only for a standard create role when the server allows it", () => {
    storesQuery.value = listData();
    render(<StoreListPage />);

    expect(screen.getByRole("link", { name: "新建店铺" })).toHaveAttribute(
      "href",
      "/workbench/stores/new",
    );
  });

  it("distinguishes true empty, filtered empty, and subscription-required quota", () => {
    storesQuery.value = listData({
      data: {
        items: [],
        quota: {
          used: 2,
          reserved: 0,
          limit: null,
          allowed: false,
          reason: "subscription_required",
        },
        pagination: { page: 1, pageSize: 20, total: 0 },
      },
    });
    const { rerender } = render(<StoreListPage />);
    expect(screen.getByText("还没有店铺")).toBeInTheDocument();
    expect(screen.getByText("已使用 2 / —")).toBeInTheDocument();
    expect(screen.getByText(/需要管理员配置有效订阅/)).toBeInTheDocument();

    navigation.search = "platform=shein";
    rerender(<StoreListPage />);
    expect(screen.getByText("没有符合筛选条件的店铺")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "清除筛选" })).toBeInTheDocument();
  });

  it("resets platform filters to page one and preserves filters across pagination boundaries", async () => {
    navigation.search = "page=2&pageSize=50&platform=shein&status=active";
    storesQuery.value = listData({
      data: {
        ...listData().data,
        pagination: { page: 2, pageSize: 50, total: 101 },
      },
    });
    const user = userEvent.setup();
    render(<StoreListPage />);

    await user.selectOptions(screen.getByRole("combobox", { name: "平台" }), "");
    expect(navigation.push).toHaveBeenLastCalledWith(
      "/workbench/stores?page=1&pageSize=50&status=active",
    );
    await user.click(screen.getByRole("button", { name: "下一页" }));
    expect(navigation.push).toHaveBeenLastCalledWith(
      "/workbench/stores?page=3&pageSize=50&platform=shein&status=active",
    );
    await user.click(screen.getByRole("button", { name: "上一页" }));
    expect(navigation.push).toHaveBeenLastCalledWith(
      "/workbench/stores?page=1&pageSize=50&platform=shein&status=active",
    );
  });

  it("disables pagination at server boundaries", () => {
    storesQuery.value = listData({
      data: {
        ...listData().data,
        pagination: { page: 1, pageSize: 20, total: 20 },
      },
    });
    render(<StoreListPage />);

    expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "下一页" })).toBeDisabled();
  });

  it.each([
    ["ORGANIZATION_ACCESS_REVOKED", "当前企业访问已不可用"],
    ["DEPENDENCY_UNAVAILABLE", "店铺服务暂时不可用"],
  ])("renders stable bounded copy for %s", (code, expectedCopy) => {
    storesQuery.value = {
      isPending: false,
      isError: true,
      error: { code, message: "raw service message", requestId: "request-id" },
      refetch: vi.fn(),
    };
    render(<StoreListPage />);

    expect(screen.getByRole("alert")).toHaveTextContent(expectedCopy);
    expect(screen.queryByText(/raw service message|request-id/)).not.toBeInTheDocument();
  });

  it("hides stale quota, table, and pagination whenever access has errored", () => {
    storesQuery.value = listData({
      isError: true,
      error: { code: "ORGANIZATION_ACCESS_REVOKED", message: "raw", requestId: "req" },
    });
    render(<StoreListPage />);

    expect(screen.getByRole("alert")).toHaveTextContent("当前企业访问已不可用");
    expect(screen.queryByRole("table", { name: "我的店铺列表" })).not.toBeInTheDocument();
    expect(screen.queryByText("已使用 4 / 5")).not.toBeInTheDocument();
    expect(screen.queryByText("第 2 页，共 45 家店铺")).not.toBeInTheDocument();
  });
});
