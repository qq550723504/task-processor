import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CollectionAPIError, type CollectionIntent } from "@/lib/api/product-collection";
import { CollectionPage } from "./collection-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, list: vi.fn(), mutate: vi.fn(), read: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/product-collection", async original => ({ ...await original<object>(), listCollectionBatches: state.list, mutateCollection: state.mutate, readCollectionOperation: state.read }));
beforeEach(() => {
  state.context = { user: { id: "actor-a" }, effectiveOrganization: { id: "org-a" }, permissions: ["workbench.collection.manage"],
    pendingCollectionIntent: null, setPendingCollectionIntent: (intent: CollectionIntent | null) => { state.context.pendingCollectionIntent = intent; }, registerOrganizationSwitchGuard: () => () => {} };
  state.list.mockReset().mockResolvedValue({ items: [], total: 0 }); state.mutate.mockReset(); state.read.mockReset();
});
afterEach(cleanup);

it("keeps an uncertain batch command and verifies the original key without another write", async () => {
  state.mutate.mockRejectedValue(new CollectionAPIError("OUTCOME_UNKNOWN", 503));
  render(<CollectionPage />);
  await userEvent.click(screen.getByRole("button", { name: "管理批次" }));
  await userEvent.type(screen.getByLabelText("新批次名称"), "运营资料");
  await userEvent.click(screen.getByRole("button", { name: "新建批次" }));
  await screen.findByText("结果待核实");
  const intent = state.mutate.mock.calls[0][0] as CollectionIntent;
  expect(intent).toMatchObject({ userId: "actor-a", organizationId: "org-a", command: { action: "create_batch", name: "运营资料" } });
  state.read.mockResolvedValue({ operationId: "550e8400-e29b-41d4-a716-446655440000", revision: 1, replayed: true });
  await userEvent.click(screen.getByRole("button", { name: "核实原操作" }));
  await waitFor(() => expect(state.read).toHaveBeenCalledWith(intent, expect.any(AbortSignal)));
  expect(state.mutate).toHaveBeenCalledOnce();
});

it("aborts an old enterprise read and ignores its late batch data", async () => {
  let resolve!: (value: unknown) => void;
  state.list.mockReturnValueOnce(new Promise(ok => { resolve = ok; }));
  const view = render(<CollectionPage />);
  await waitFor(() => expect(state.list).toHaveBeenCalledOnce());
  const signal = state.list.mock.calls[0][2] as AbortSignal;
  state.context = { ...state.context, user: { id: "actor-b" }, effectiveOrganization: { id: "org-b" } };
  view.rerender(<CollectionPage />);
  expect(signal.aborted).toBe(true);
  await act(async () => resolve({ items: [{ id: "550e8400-e29b-41d4-a716-446655440000", name: "旧企业私有批次", kind: "manual", revision: 1, count: 1, createdAt: "2026-10-08T00:00:00Z" }], total: 1 }));
  expect(screen.queryByText("旧企业私有批次")).not.toBeInTheDocument();
});
