import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const context = vi.hoisted(() => ({
  effectiveOrganization: { id: "org-a", name: "企业 A", roles: [] as string[] },
  registerOrganizationSwitchGuard: vi.fn(() => vi.fn()),
}));
const create = vi.hoisted(() => ({ mutate: vi.fn(), retryLast: vi.fn(), canRetryLast: false, isPending: false }));
const update = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));

vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => context }));
vi.mock("@/lib/query/use-workbench-stores", () => ({
  useCreateWorkbenchStore: () => create,
  useUpdateWorkbenchStore: () => update,
}));

import { StoreForm } from "@/components/workbench/stores/store-form";

const STORE = {
  id: "11111111-1111-4111-8111-111111111111", name: "旧店铺", platform: "shein" as const,
  region: "CN", externalStoreId: "external-1", lifecycleStatus: "active" as const,
  connectionStatus: "disconnected" as const, version: 3,
  createdAt: "2026-08-31T00:00:00Z", updatedAt: "2026-08-31T00:00:00Z",
};

describe("StoreForm", () => {
  afterEach(() => { vi.restoreAllMocks(); router.push.mockReset(); router.replace.mockReset(); create.mutate.mockReset(); create.retryLast.mockReset(); create.canRetryLast = false; create.isPending = false; update.mutate.mockReset(); update.isPending = false; context.effectiveOrganization = { id: "org-a", name: "企业 A", roles: [] }; context.registerOrganizationSwitchGuard.mockClear(); });

  it("validates and canonicalizes create input while keeping SHEIN trusted and read-only", async () => {
    const user = userEvent.setup();
    render(<StoreForm mode="create" />);
    expect(screen.getByRole("heading", { name: "在企业 A 新建店铺" })).toBeInTheDocument();
    expect(screen.getByDisplayValue("SHEIN")).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "创建店铺" }));
    expect(await screen.findByText("请填写店铺名称")).toBeInTheDocument();
    await user.type(screen.getByLabelText("店铺名称"), "  新店铺  ");
    await user.type(screen.getByLabelText("区域"), "  CN  ");
    await user.click(screen.getByRole("button", { name: "创建店铺" }));
    expect(create.mutate).toHaveBeenCalledWith({ name: "新店铺", platform: "shein", region: "CN" }, expect.any(Object));
    expect(document.body.textContent).not.toMatch(/username|password|token|secret|cookie|credential/i);
  });

  it("keeps create drafts and exposes only the keyed retry after a retryable failure", async () => {
    const user = userEvent.setup();
    render(<StoreForm mode="create" />);
    await user.type(screen.getByLabelText("店铺名称"), "新店铺");
    await user.type(screen.getByLabelText("区域"), "CN");
    await user.click(screen.getByRole("button", { name: "创建店铺" }));
    const options = create.mutate.mock.calls[0]?.[1];
    create.canRetryLast = true;
    options.onError({ code: "DEPENDENCY_UNAVAILABLE", status: 503, fieldErrors: [] });
    await waitFor(() => expect(screen.getByRole("button", { name: "创建店铺" })).toBeDisabled());
    expect(await screen.findByRole("button", { name: "重试创建" })).toBeInTheDocument();
    create.retryLast.mockResolvedValue({ id: STORE.id });
    await user.click(screen.getByRole("button", { name: "重试创建" }));
    expect(create.retryLast).toHaveBeenCalledTimes(1);
    expect(screen.getByLabelText("店铺名称")).toHaveValue("新店铺");
  });

  it("redirects only after a successful create and shows bounded quota and access errors", async () => {
    const user = userEvent.setup(); render(<StoreForm mode="create" />);
    await user.type(screen.getByLabelText("店铺名称"), "新店铺"); await user.type(screen.getByLabelText("区域"), "CN");
    await user.click(screen.getByRole("button", { name: "创建店铺" }));
    create.mutate.mock.calls[0]?.[1].onError({ code: "STORE_LIMIT_REACHED", status: 409, fieldErrors: [], message: "raw" });
    expect(await screen.findByText(/店铺额度不可用/)).toBeInTheDocument();
    expect(router.push).not.toHaveBeenCalled();
    create.mutate.mock.calls[0]?.[1].onSuccess({ id: STORE.id });
    expect(router.push).toHaveBeenCalledWith(`/workbench/stores/${STORE.id}`);
  });

  it("maps known server fields and bounds unknown server details", async () => {
    const user = userEvent.setup();
    render(<StoreForm mode="create" />);
    await user.type(screen.getByLabelText("店铺名称"), "新店铺"); await user.type(screen.getByLabelText("区域"), "CN");
    await user.click(screen.getByRole("button", { name: "创建店铺" }));
    create.mutate.mock.calls[0]?.[1].onError({ code: "INVALID_REQUEST", status: 400, fieldErrors: [{ field: "name", code: "invalid" }, { field: "platform", code: "unexpected" }], message: "secret message" });
    expect(await screen.findByText("店铺名称格式不正确")).toBeInTheDocument();
    expect(screen.getByText("表单包含无法处理的字段错误")).toBeInTheDocument();
    expect(screen.queryByText("secret message")).not.toBeInTheDocument();
  });

  it("submits edit with the displayed version and requires explicit conflict retry", async () => {
    const user = userEvent.setup(); const onConflict = vi.fn();
    const view = render(<StoreForm mode="edit" store={STORE} onConflict={onConflict} />);
    expect(screen.getByDisplayValue("external-1")).toBeDisabled();
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "我的草稿");
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    expect(update.mutate).toHaveBeenCalledWith({ id: STORE.id, version: 3, input: { name: "我的草稿", region: "CN" } }, expect.any(Object));
    update.mutate.mock.calls[0]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    expect(onConflict).toHaveBeenCalledWith({ name: "我的草稿", region: "CN" }, STORE);
    const latest = { ...STORE, name: "他人更新", version: 4 };
    view.rerender(<StoreForm mode="edit" store={STORE} conflict={{ latest, changedFields: ["name"] }} onConflict={onConflict} />);
    expect(screen.getByText(/名称已被其他人修改/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "使用最新版本重新保存" }));
    expect(update.mutate).toHaveBeenLastCalledWith({ id: STORE.id, version: 4, input: { name: "我的草稿", region: "CN" } }, expect.any(Object));
    update.mutate.mock.calls[1]?.[1].onError({ code: "STORE_VERSION_CONFLICT", status: 409, fieldErrors: [] });
    expect(onConflict).toHaveBeenLastCalledWith({ name: "我的草稿", region: "CN" }, latest);
  });

  it("keeps a dirty edit draft while advancing its baseline across background query projections", async () => {
    const user = userEvent.setup();
    const view = render(<StoreForm mode="edit" store={STORE} />);
    await user.clear(screen.getByLabelText("店铺名称")); await user.type(screen.getByLabelText("店铺名称"), "草稿");
    view.rerender(<StoreForm mode="edit" store={{ ...STORE, name: "后台刷新", version: 4 }} />);
    expect(screen.getByLabelText("店铺名称")).toHaveValue("草稿");
    await user.click(screen.getByRole("button", { name: "保存更改" }));
    expect(update.mutate).toHaveBeenCalledWith({ id: STORE.id, version: STORE.version + 1, input: { name: "草稿", region: "CN" } }, expect.any(Object));
  });

  it("advances a clean edit baseline for a newer same-lifecycle projection", async () => {
    const view = render(<StoreForm mode="edit" store={STORE} />);
    view.rerender(<StoreForm mode="edit" store={{ ...STORE, name: "后台刷新", version: 4 }} />);
    await waitFor(() => expect(screen.getByLabelText("店铺名称")).toHaveValue("后台刷新"));
    await userEvent.setup().click(screen.getByRole("button", { name: "保存更改" }));
    expect(update.mutate).toHaveBeenCalledWith(
      { id: STORE.id, version: 4, input: { name: "后台刷新", region: "CN" } },
      expect.any(Object),
    );
  });

  it("refreshes a clean edit baseline and form after a lifecycle projection advances", async () => {
    const view = render(<StoreForm mode="edit" store={STORE} />);
    view.rerender(<StoreForm mode="edit" store={{ ...STORE, name: "已停用店铺", lifecycleStatus: "disabled" as const, version: 4 }} />);
    await waitFor(() => {
      expect(screen.getByLabelText("店铺名称")).toHaveValue("已停用店铺");
    });
    await userEvent.setup().click(screen.getByRole("button", { name: "保存更改" }));
    expect(update.mutate).toHaveBeenCalledWith(
      { id: STORE.id, version: 4, input: { name: "已停用店铺", region: "CN" } },
      expect.any(Object),
    );
  });

  it("guards dirty Organization switches and waits for the actual context change before navigating", async () => {
    let guard: ((target: { id: string; name: string; roles: string[] }) => boolean) | undefined;
    context.registerOrganizationSwitchGuard.mockImplementation(((next: (target: { id: string; name: string; roles: string[] }) => boolean) => { guard = next; return vi.fn(); }) as never);
    const confirm = vi.fn(() => false); vi.stubGlobal("confirm", confirm);
    const user = userEvent.setup(); const view = render(<StoreForm mode="create" />);
    await user.type(screen.getByLabelText("店铺名称"), "草稿");
    await waitFor(() => expect(guard).toBeDefined());
    expect(guard?.({ id: "org-b", name: "企业 B", roles: [] })).toBe(false);
    expect(confirm).toHaveBeenCalledWith(expect.stringContaining("企业 A → 企业 B"));
    confirm.mockReturnValue(true);
    expect(guard?.({ id: "org-b", name: "企业 B", roles: [] })).toBe(true);
    expect(router.replace).not.toHaveBeenCalled();
    context.effectiveOrganization = { id: "org-b", name: "企业 B", roles: [] };
    view.rerender(<StoreForm mode="create" />);
    expect(screen.getByRole("status")).toHaveTextContent("正在切换企业");
    expect(screen.queryByDisplayValue("草稿")).not.toBeInTheDocument();
    expect(router.replace).toHaveBeenCalledWith("/workbench/stores");
  });

  it("does not lock explicit conflict retry after keyboard or programmatic form submission", async () => {
    const latest = { ...STORE, version: 4 };
    const user = userEvent.setup();
    const { container } = render(<StoreForm conflict={{ latest, changedFields: ["region"] }} mode="edit" store={STORE} />);
    const formElement = container.querySelector("form");
    expect(formElement).not.toBeNull();
    fireEvent.submit(formElement!);
    expect(update.mutate).not.toHaveBeenCalled();
    const retry = screen.getByRole("button", { name: "使用最新版本重新保存" });
    expect(retry).toBeEnabled();
    await user.click(retry);
    expect(update.mutate).toHaveBeenCalledWith({ id: STORE.id, version: 4, input: { name: STORE.name, region: STORE.region } }, expect.any(Object));
  });
});
