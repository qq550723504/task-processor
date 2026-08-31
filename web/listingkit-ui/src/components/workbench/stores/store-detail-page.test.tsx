import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const context = vi.hoisted(() => ({ effectiveOrganization: { id: "org-a", name: "企业 A", roles: [] as string[] }, registerOrganizationSwitchGuard: vi.fn(() => vi.fn()) }));
const query = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const update = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));
const create = vi.hoisted(() => ({ mutate: vi.fn(), retryLast: vi.fn(), canRetryLast: false, isPending: false }));

vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => context }));
vi.mock("@/lib/query/use-workbench-stores", () => ({ useWorkbenchStore: () => query.value, useUpdateWorkbenchStore: () => update, useCreateWorkbenchStore: () => create }));

import { StoreDetailPage } from "@/components/workbench/stores/store-detail-page";

const STORE = { id: "11111111-1111-4111-8111-111111111111", name: "店铺", platform: "shein" as const, region: "CN", externalStoreId: "", lifecycleStatus: "active" as const, connectionStatus: "disconnected" as const, version: 1, createdAt: "2026-08-31T00:00:00Z", updatedAt: "2026-08-31T00:00:00Z" };
describe("StoreDetailPage", () => {
  afterEach(() => { query.value = {}; update.mutate.mockReset(); update.isPending = false; create.mutate.mockReset(); context.registerOrganizationSwitchGuard.mockClear(); });
  it("renders stable loading, not-found, access, and dependency states", () => {
    query.value = { isPending: true }; const { rerender } = render(<StoreDetailPage storeId={STORE.id} />); expect(screen.getByRole("status")).toHaveTextContent("正在加载店铺");
    query.value = { isPending: false, isError: true, error: { code: "STORE_NOT_FOUND" }, refetch: vi.fn() }; rerender(<StoreDetailPage storeId={STORE.id} />); expect(screen.getByRole("alert")).toHaveTextContent("店铺不存在或已不可访问");
    query.value = { isPending: false, isError: true, error: { code: "PERMISSION_DENIED" }, refetch: vi.fn() }; rerender(<StoreDetailPage storeId={STORE.id} />); expect(screen.getByRole("alert")).toHaveTextContent("没有编辑当前企业店铺的权限");
  });
  it("preserves the actual form draft when conflict refetch returns a newer projection", async () => {
    const latest = { ...STORE, name: "服务端名称", region: "US", version: 2 };
    const refetch = vi.fn().mockResolvedValue({ data: latest, isSuccess: true, isError: false });
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    const user = userEvent.setup(); render(<StoreDetailPage storeId={STORE.id} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "我的草稿");
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    update.mutate.mock.calls[0]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    await waitFor(() => expect(refetch).toHaveBeenCalled());
    expect(screen.getByLabelText("店铺名称")).toHaveValue("我的草稿");
    expect(screen.getByText(/名称、区域已被其他人修改/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "使用最新版本重新保存" }));
    expect(update.mutate).toHaveBeenLastCalledWith({ id: STORE.id, version: 2, input: { name: "我的草稿", region: "CN" } }, expect.any(Object));
  });
  it("fails closed when latest conflict fetch fails and repeats conflict recovery", async () => {
    const latest = { ...STORE, version: 2, name: "服务端名称" };
    const refetch = vi.fn().mockResolvedValueOnce({ data: STORE, isSuccess: false, isError: true }).mockResolvedValueOnce({ data: latest, isSuccess: true, isError: false }).mockResolvedValueOnce({ data: { ...latest, version: 3, region: "US" }, isSuccess: true, isError: false });
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    const user = userEvent.setup(); render(<StoreDetailPage storeId={STORE.id} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "草稿");
    await user.click(screen.getByRole("button", { name: "保存更改" })); update.mutate.mock.calls[0]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    expect(await screen.findByText(/无法确认店铺最新版本/)).toBeInTheDocument();
    expect(screen.getByLabelText("店铺名称")).toHaveValue("草稿");
    expect(screen.getByRole("button", { name: "保存更改" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "重试获取最新版本" })); await waitFor(() => expect(screen.getByRole("button", { name: "使用最新版本重新保存" })).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "使用最新版本重新保存" })); update.mutate.mock.calls[1]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    await waitFor(() => expect(refetch).toHaveBeenCalledTimes(3));
    expect(screen.getByLabelText("店铺名称")).toHaveValue("草稿");
  });
  it("locks normal save while a conflict refetch is pending", async () => {
    let settle!: (result: { data: typeof STORE; isSuccess: boolean; isError: boolean }) => void;
    const refetch = vi.fn(() => new Promise<{ data: typeof STORE; isSuccess: boolean; isError: boolean }>((resolve) => { settle = resolve; }));
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    const user = userEvent.setup(); render(<StoreDetailPage storeId={STORE.id} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "草稿");
    await user.click(screen.getByRole("button", { name: "保存更改" })); update.mutate.mock.calls[0]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    expect(await screen.findByRole("status")).toHaveTextContent("正在获取店铺最新版本");
    expect(screen.getByRole("button", { name: "保存更改" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    expect(update.mutate).toHaveBeenCalledTimes(1);
    settle({ data: STORE, isSuccess: false, isError: true });
    expect(await screen.findByText(/无法确认店铺最新版本/)).toBeInTheDocument();
  });
  it("navigates only after the provider proves an Organization change even when the detail query has been cleared", async () => {
    query.value = { isPending: false, isError: false, data: STORE, refetch: vi.fn() };
    let guard: ((target: { id: string; name: string; roles: string[] }) => boolean) | undefined;
    context.registerOrganizationSwitchGuard.mockImplementation(((next: (target: { id: string; name: string; roles: string[] }) => boolean) => { guard = next; return vi.fn(); }) as never);
    const confirm = vi.fn(() => true); vi.stubGlobal("confirm", confirm);
    const user = userEvent.setup(); const view = render(<StoreDetailPage storeId={STORE.id} />);
    await user.type(screen.getByLabelText("店铺名称"), "草稿");
    await waitFor(() => expect(guard).toBeDefined());
    expect(guard?.({ id: "org-b", name: "企业 B", roles: [] })).toBe(true);
    expect(router.replace).not.toHaveBeenCalled();
    context.effectiveOrganization = { id: "org-b", name: "企业 B", roles: [] };
    query.value = { isPending: true, isError: false, data: undefined, refetch: vi.fn() };
    view.rerender(<StoreDetailPage storeId={STORE.id} />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/workbench/stores"));
  });
});
