import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

const context = vi.hoisted(() => ({
  effectiveOrganization: { id: "org-a", name: "企业 A", roles: [] as string[] },
  roles: ["listingkit_admin"],
  retry: vi.fn(),
}));
const queryClient = vi.hoisted(() => ({ removeQueries: vi.fn() }));
const enable = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));
const disable = vi.hoisted(() => ({ mutate: vi.fn(), isPending: false }));
const remove = vi.hoisted(() => ({ mutate: vi.fn(), retryLast: vi.fn(), canRetryLast: false, isPending: false }));

vi.mock("@tanstack/react-query", () => ({ useQueryClient: () => queryClient }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => context }));
vi.mock("@/lib/query/use-workbench-stores", () => ({
  useEnableWorkbenchStore: () => enable,
  useDisableWorkbenchStore: () => disable,
  useDeleteWorkbenchStore: () => remove,
  workbenchStoreKeys: { root: (organizationId: string) => ["workbench", organizationId, "stores"] },
}));

import { StoreLifecycleActions } from "@/components/workbench/stores/store-lifecycle-actions";

const STORE = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "华东旗舰店",
  platform: "shein" as const,
  region: "CN",
  externalStoreId: "",
  lifecycleStatus: "active" as const,
  connectionStatus: "disconnected" as const,
  version: 4,
  createdAt: "2026-08-31T00:00:00Z",
  updatedAt: "2026-08-31T00:00:00Z",
};

describe("StoreLifecycleActions", () => {
  afterEach(() => {
    context.roles = ["listingkit_admin"];
    context.effectiveOrganization = { id: "org-a", name: "企业 A", roles: [] };
    context.retry.mockReset(); queryClient.removeQueries.mockReset();
    enable.mutate.mockReset(); enable.isPending = false;
    disable.mutate.mockReset(); disable.isPending = false;
    remove.mutate.mockReset(); remove.retryLast.mockReset(); remove.canRetryLast = false; remove.isPending = false;
  });

  it.each([
    ["listingkit_viewer", false, false],
    ["listingkit_operator", true, false],
    ["listingkit_admin", true, true],
    ["platform_admin", true, true],
  ])("uses role %s to expose lifecycle controls without treating UI gates as authority", (role, canUpdate, canDelete) => {
    context.roles = [role];
    render(<StoreLifecycleActions store={STORE} />);
    expect(screen.queryByRole("button", { name: "停用店铺" }) !== null).toBe(canUpdate);
    expect(screen.queryByRole("button", { name: "删除店铺" }) !== null).toBe(canDelete);
  });

  it("uses the active/disabled action labels and exact displayed version", async () => {
    const user = userEvent.setup();
    const onStoreUpdated = vi.fn();
    const { rerender } = render(<StoreLifecycleActions onStoreUpdated={onStoreUpdated} store={STORE} />);
    await user.click(screen.getByRole("button", { name: "停用店铺" }));
    expect(disable.mutate).toHaveBeenCalledWith({ id: STORE.id, version: 4 }, expect.any(Object));
    disable.mutate.mock.calls[0]?.[1].onSuccess({ ...STORE, lifecycleStatus: "disabled", version: 5 });
    expect(onStoreUpdated).toHaveBeenCalledWith(expect.objectContaining({ lifecycleStatus: "disabled", version: 5 }));
    rerender(<StoreLifecycleActions store={{ ...STORE, lifecycleStatus: "disabled", version: 5 }} />);
    expect(screen.getByRole("button", { name: "重新启用店铺" })).toBeInTheDocument();
  });

  it("locks a local lifecycle action synchronously against a double click", async () => {
    const user = userEvent.setup(); render(<StoreLifecycleActions store={STORE} />);
    const action = screen.getByRole("button", { name: "停用店铺" });
    await user.dblClick(action);
    expect(disable.mutate).toHaveBeenCalledTimes(1);
  });

  it("requires the exact visible Organization and Store phrase and clears it on cancel", async () => {
    const user = userEvent.setup(); render(<StoreLifecycleActions store={STORE} />);
    await user.click(screen.getByRole("button", { name: "删除店铺" }));
    const dialog = screen.getByRole("alertdialog");
    const phrase = "删除 企业 A 的店铺 华东旗舰店";
    expect(dialog).toHaveTextContent(phrase);
    const input = screen.getByLabelText("确认删除文本");
    await user.type(input, `${phrase} `);
    expect(screen.getByRole("button", { name: "确认删除" })).toBeDisabled();
    await user.clear(input); await user.type(input, phrase);
    expect(screen.getByRole("button", { name: "确认删除" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "取消" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "删除店铺" }));
    expect(screen.getByLabelText("确认删除文本")).toHaveValue("");
  });

  it("keeps an interrupted deletion in its dialog and retries only through retryLast", async () => {
    const user = userEvent.setup(); remove.canRetryLast = true;
    render(<StoreLifecycleActions store={STORE} />);
    await user.click(screen.getByRole("button", { name: "删除店铺" }));
    await user.type(screen.getByLabelText("确认删除文本"), "删除 企业 A 的店铺 华东旗舰店");
    await user.click(screen.getByRole("button", { name: "确认删除" }));
    remove.mutate.mock.calls[0]?.[1].onError({ status: 503, code: "DEPENDENCY_UNAVAILABLE" });
    expect(await screen.findByRole("button", { name: "重试删除" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "重试删除" }));
    expect(remove.retryLast).toHaveBeenCalledTimes(1);
    expect(remove.mutate).toHaveBeenCalledTimes(1);
  });

  it("locks a deleting Store and only offers an eligible existing delete retry", () => {
    const { rerender } = render(<StoreLifecycleActions store={{ ...STORE, lifecycleStatus: "deleting" }} />);
    expect(screen.getByText(/删除正在进行中/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /停用|重新启用|删除店铺/ })).not.toBeInTheDocument();
    expect(screen.getByText(/刷新页面或联系支持/)).toBeInTheDocument();
    remove.canRetryLast = true;
    rerender(<StoreLifecycleActions store={{ ...STORE, lifecycleStatus: "deleting" }} />);
    expect(screen.getByRole("button", { name: "重试删除" })).toBeInTheDocument();
  });

  it("refreshes a version conflict before allowing another action", async () => {
    const user = userEvent.setup();
    const onRefreshStore = vi.fn().mockResolvedValue({ ...STORE, version: 5, lifecycleStatus: "active" });
    const onStoreUpdated = vi.fn();
    render(<StoreLifecycleActions onRefreshStore={onRefreshStore} onStoreUpdated={onStoreUpdated} store={STORE} />);
    await user.click(screen.getByRole("button", { name: "停用店铺" }));
    disable.mutate.mock.calls[0]?.[1].onError({ status: 409, code: "STORE_VERSION_CONFLICT" });
    expect(await screen.findByText(/店铺信息已变化/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "刷新店铺信息" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "刷新店铺信息" }));
    await waitFor(() => expect(onStoreUpdated).toHaveBeenCalledWith(expect.objectContaining({ version: 5 })));
  });

  it("fails closed on a revoked grant by removing only this Organization Store queries and retrying context", async () => {
    const user = userEvent.setup(); render(<StoreLifecycleActions store={STORE} />);
    await user.click(screen.getByRole("button", { name: "停用店铺" }));
    disable.mutate.mock.calls[0]?.[1].onError({ status: 403, code: "ORGANIZATION_ACCESS_REVOKED" });
    expect(queryClient.removeQueries).toHaveBeenCalledWith({ queryKey: ["workbench", "org-a", "stores"] });
    expect(context.retry).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole("alert")).toHaveTextContent("当前企业访问已被撤销");
    expect(screen.queryByRole("button", { name: "停用店铺" })).not.toBeInTheDocument();
  });
});
