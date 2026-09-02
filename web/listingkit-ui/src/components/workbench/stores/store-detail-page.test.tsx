import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const context = vi.hoisted(() => ({ effectiveOrganization: { id: "org-a", name: "企业 A", roles: [] as string[] }, roles: ["listingkit_operator"], retry: vi.fn(), registerOrganizationSwitchGuard: vi.fn(() => vi.fn()) }));
const query = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const update = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));
const create = vi.hoisted(() => ({ mutate: vi.fn(), retryLast: vi.fn(), canRetryLast: false, isPending: false }));
const enable = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));
const disable = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));
const resumeCreate = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));
const remove = vi.hoisted(() => ({ mutate: vi.fn(), retryLast: vi.fn(), canRetryLast: false, isPending: false }));
const queryClient = vi.hoisted(() => ({ removeQueries: vi.fn() }));

vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => context }));
vi.mock("@tanstack/react-query", async (importOriginal) => ({ ...(await importOriginal<typeof import("@tanstack/react-query")>()), useQueryClient: () => queryClient }));
vi.mock("@/lib/query/use-workbench-stores", () => ({
  useWorkbenchStore: () => query.value,
  useUpdateWorkbenchStore: () => update,
  useCreateWorkbenchStore: () => create,
  useEnableWorkbenchStore: () => enable,
  useDisableWorkbenchStore: () => disable,
  useResumeWorkbenchStore: () => resumeCreate,
  useDeleteWorkbenchStore: () => remove,
  workbenchStoreKeys: { root: (organizationId: string) => ["workbench", organizationId, "stores"] },
}));

import { StoreDetailPage } from "@/components/workbench/stores/store-detail-page";

const STORE = { id: "11111111-1111-4111-8111-111111111111", name: "店铺", platform: "shein" as const, region: "CN", externalStoreId: "", lifecycleStatus: "active" as const, connectionStatus: "disconnected" as const, version: 1, createdAt: "2026-08-31T00:00:00Z", updatedAt: "2026-08-31T00:00:00Z" };
describe("StoreDetailPage", () => {
  afterEach(() => { query.value = {}; update.mutate.mockReset(); update.isPending = false; create.mutate.mockReset(); enable.mutate.mockReset(); enable.isPending = false; disable.mutate.mockReset(); disable.isPending = false; resumeCreate.mutate.mockReset(); resumeCreate.isPending = false; remove.mutate.mockReset(); remove.retryLast.mockReset(); remove.canRetryLast = false; remove.isPending = false; queryClient.removeQueries.mockReset(); context.retry.mockReset(); context.effectiveOrganization = { id: "org-a", name: "企业 A", roles: [] }; context.roles = ["listingkit_operator"]; context.registerOrganizationSwitchGuard.mockReset(); context.registerOrganizationSwitchGuard.mockImplementation(() => vi.fn()); router.push.mockReset(); router.replace.mockReset(); });
  it("renders stable loading, not-found, access, and dependency states", () => {
    query.value = { isPending: true }; const { rerender } = render(<StoreDetailPage storeId={STORE.id} />); expect(screen.getByRole("status")).toHaveTextContent("正在加载店铺");
    query.value = { isPending: false, isError: true, error: { code: "STORE_NOT_FOUND" }, refetch: vi.fn() }; rerender(<StoreDetailPage storeId={STORE.id} />); expect(screen.getByRole("alert")).toHaveTextContent("店铺不存在或已不可访问");
    query.value = { isPending: false, isError: true, error: { code: "PERMISSION_DENIED" }, refetch: vi.fn() }; rerender(<StoreDetailPage storeId={STORE.id} />); expect(screen.getByRole("alert")).toHaveTextContent("没有编辑当前企业店铺的权限");
  });
  it.each([
    "ORGANIZATION_CONTEXT_CHANGED",
    "ORGANIZATION_ACCESS_REVOKED",
    "ORGANIZATION_ACCESS_DENIED",
  ])("refreshes organization context before retrying a detail request after %s", async (code) => {
    const refetch = vi.fn();
    query.value = { isPending: false, isError: true, error: { code }, refetch };
    render(<StoreDetailPage storeId={STORE.id} />);

    await userEvent.click(screen.getByRole("button", { name: "重试" }));

    expect(context.retry).toHaveBeenCalledTimes(1);
    expect(refetch).not.toHaveBeenCalled();
  });

  it("keeps a safe Store identity and lifecycle information visible while hiding edit for a viewer", () => {
    context.roles = ["listingkit_viewer"];
    query.value = { isPending: false, isError: false, data: STORE, refetch: vi.fn() };
    render(<StoreDetailPage storeId={STORE.id} />);
    expect(screen.getByRole("heading", { name: "店铺" })).toBeInTheDocument();
    expect(screen.getByText(/店铺状态：已启用/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /停用店铺|重新启用店铺|删除店铺/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存更改" })).not.toBeInTheDocument();
  });
  it("hides editing but keeps deleting lifecycle information visible while deletion is in progress", () => {
    query.value = { isPending: false, isError: false, data: { ...STORE, lifecycleStatus: "deleting" as const }, refetch: vi.fn() };
    render(<StoreDetailPage storeId={STORE.id} />);
    expect(screen.getByText(/店铺状态：删除中/)).toBeInTheDocument();
    expect(screen.getByText(/删除正在进行中/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存更改" })).not.toBeInTheDocument();
  });
  it("hides editing while the store is still provisioning", () => {
    query.value = { isPending: false, isError: false, data: { ...STORE, lifecycleStatus: "provisioning" as const }, refetch: vi.fn() };
    render(<StoreDetailPage storeId={STORE.id} />);
    expect(screen.getByText(/店铺状态：开通中/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存更改" })).not.toBeInTheDocument();
  });
  it("locks the real detail form when a terminal delete refresh proves deleting", async () => {
    const deleting = { ...STORE, lifecycleStatus: "deleting" as const, version: 2 };
    const refetch = vi.fn().mockResolvedValue({ data: deleting, isSuccess: true, isError: false });
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    context.roles = ["listingkit_admin"];
    remove.canRetryLast = true;
    const user = userEvent.setup();
    render(<StoreDetailPage storeId={STORE.id} />);
    expect(screen.getByRole("button", { name: "保存更改" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "删除店铺" }));
    await user.type(screen.getByLabelText("确认删除文本"), "删除 企业 A 的店铺 店铺");
    await user.click(screen.getByRole("button", { name: "确认删除" }));
    remove.mutate.mock.calls[0]?.[1].onError({ status: 503, code: "DEPENDENCY_UNAVAILABLE" });
    expect(await screen.findByText(/店铺状态：删除中/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存更改" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "重试删除" })).toBeInTheDocument();
    expect(remove.mutate).toHaveBeenCalledTimes(1);
  });
  it("keeps the real detail deletion terminal-locked when refresh returns a newer active projection", async () => {
    const newerActive = { ...STORE, version: 2 };
    const refetch = vi.fn().mockResolvedValue({ data: newerActive, isSuccess: true, isError: false });
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    context.roles = ["listingkit_admin"];
    remove.canRetryLast = true;
    const user = userEvent.setup();
    render(<StoreDetailPage storeId={STORE.id} />);
    await user.click(screen.getByRole("button", { name: "删除店铺" }));
    await user.type(screen.getByLabelText("确认删除文本"), "删除 企业 A 的店铺 店铺");
    await user.click(screen.getByRole("button", { name: "确认删除" }));
    remove.mutate.mock.calls[0]?.[1].onError({ status: 503, code: "DEPENDENCY_UNAVAILABLE" });
    await waitFor(() => expect(refetch).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("button", { name: "删除店铺" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "确认删除" })).toBeDisabled();
    expect(screen.getByLabelText("店铺名称")).toHaveValue("店铺");
    await user.click(screen.getByRole("button", { name: "重试删除" }));
    expect(remove.retryLast).toHaveBeenCalledTimes(1);
    expect(remove.mutate).toHaveBeenCalledTimes(1);
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
  it("switches to deleting lifecycle recovery when conflict refetch returns deleting", async () => {
    const deleting = { ...STORE, lifecycleStatus: "deleting" as const, version: 2 };
    const refetch = vi.fn().mockResolvedValue({ data: deleting, isSuccess: true, isError: false });
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    const user = userEvent.setup(); render(<StoreDetailPage storeId={STORE.id} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "我的草稿");
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    update.mutate.mock.calls[0]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });

    expect(await screen.findByText(/店铺状态：删除中/)).toBeInTheDocument();
    expect(screen.getByText(/删除正在进行中/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "使用最新版本重新保存" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存更改" })).not.toBeInTheDocument();
    update.mutate.mock.calls[0]?.[1].onSuccess({ ...STORE, version: 3 });
    expect(screen.getByText(/店铺状态：删除中/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存更改" })).not.toBeInTheDocument();
  });
  it("lets a newer background refetch replace a locally displayed mutation result", async () => {
    const refetch = vi.fn();
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    const user = userEvent.setup();
    const view = render(<StoreDetailPage storeId={STORE.id} />);

    await user.clear(screen.getByLabelText("店铺名称"));
    await user.type(screen.getByLabelText("店铺名称"), "本地保存");
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    update.mutate.mock.calls[0]?.[1].onSuccess({ ...STORE, name: "本地保存", version: 2 });
    expect(await screen.findByDisplayValue("本地保存")).toBeInTheDocument();

    query.value = { isPending: false, isError: false, data: { ...STORE, name: "其他用户保存", version: 3 }, refetch };
    view.rerender(<StoreDetailPage storeId={STORE.id} />);

    expect(screen.getByDisplayValue("其他用户保存")).toBeInTheDocument();
  });
  it("keeps the original submitted baseline when a dirty form receives a newer projection", async () => {
    const background = { ...STORE, name: "后台名称", version: 2 };
    const latest = { ...background, region: "US", version: 3 };
    const refetch = vi.fn().mockResolvedValue({ data: latest, isSuccess: true, isError: false });
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    const user = userEvent.setup(); const view = render(<StoreDetailPage storeId={STORE.id} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "我的草稿");
    query.value = { isPending: false, isError: false, data: background, refetch };
    view.rerender(<StoreDetailPage storeId={STORE.id} />);
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    expect(update.mutate).toHaveBeenCalledWith({ id: STORE.id, version: 1, input: { name: "我的草稿", region: "CN" } }, expect.any(Object));
    update.mutate.mock.calls[0]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    expect(await screen.findByText(/名称、区域已被其他人修改/)).toBeInTheDocument();
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
    expect(screen.getByText(/最新版本中区域已被其他人修改/)).toBeInTheDocument();
    expect(screen.queryByText(/最新版本中名称、区域已被其他人修改/)).not.toBeInTheDocument();
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
    expect(screen.getByRole("status")).toHaveTextContent("正在切换企业");
    expect(screen.queryByDisplayValue(/店铺草稿/)).not.toBeInTheDocument();
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/workbench/stores"));
  });

  it("hides a successful edit projection on the first frame of an actual Organization change", async () => {
    query.value = { isPending: false, isError: false, data: STORE, refetch: vi.fn() };
    const user = userEvent.setup(); const view = render(<StoreDetailPage storeId={STORE.id} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "旧企业已保存");
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    update.mutate.mock.calls[0]?.[1].onSuccess({ ...STORE, name: "旧企业已保存", version: 2 });
    expect(await screen.findByDisplayValue("旧企业已保存")).toBeInTheDocument();
    context.effectiveOrganization = { id: "org-b", name: "企业 B", roles: [] };
    query.value = { isPending: true, isError: false, data: undefined, refetch: vi.fn() };
    view.rerender(<StoreDetailPage storeId={STORE.id} />);
    expect(screen.getByRole("status")).toHaveTextContent("正在切换企业");
    expect(screen.queryByDisplayValue("旧企业已保存")).not.toBeInTheDocument();
  });

  it("hides conflict recovery and its draft on the first frame of an actual Organization change", async () => {
    const refetch = vi.fn().mockResolvedValue({ data: { ...STORE, name: "服务端", version: 2 }, isSuccess: true, isError: false });
    query.value = { isPending: false, isError: false, data: STORE, refetch };
    const user = userEvent.setup(); const view = render(<StoreDetailPage storeId={STORE.id} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "旧企业冲突草稿");
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    update.mutate.mock.calls[0]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    expect(await screen.findByRole("button", { name: "使用最新版本重新保存" })).toBeInTheDocument();
    context.effectiveOrganization = { id: "org-b", name: "企业 B", roles: [] };
    query.value = { isPending: true, isError: false, data: undefined, refetch: vi.fn() };
    view.rerender(<StoreDetailPage storeId={STORE.id} />);
    expect(screen.getByRole("status")).toHaveTextContent("正在切换企业");
    expect(screen.queryByDisplayValue("旧企业冲突草稿")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "使用最新版本重新保存" })).not.toBeInTheDocument();
  });
});
