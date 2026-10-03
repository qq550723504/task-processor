import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { BusinessTaskPage } from "./task-page";

const fixture = vi.hoisted(() => ({
  request: vi.fn(),
  context: { user: { id: "user-a" }, effectiveOrganization: { id: "org-a", capabilities: { "workbench.chat.use": true } }, roles: ["listingkit_operator"],
    aiWorkbenchAvailable: true, isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null },
}));
vi.mock("@/lib/api/ai-workbench", async original => ({ ...await original<typeof import("@/lib/api/ai-workbench")>(), requestAIWorkbench: fixture.request }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));

afterEach(() => { cleanup(); fixture.request.mockReset(); fixture.context.aiWorkbenchAvailable = true; fixture.context.effectiveOrganization.capabilities["workbench.chat.use"] = true; });

it("keeps Task reads but hides the Chat creation link for a read-only actor", async () => {
  fixture.context.effectiveOrganization.capabilities["workbench.chat.use"] = false;
  fixture.request.mockResolvedValue({ tasks: [], next: "" });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage /></QueryClientProvider>);
  expect(await screen.findByText("当前筛选无任务")).toBeVisible();
  expect(screen.queryByRole("link", { name: "向硕米发起任务" })).not.toBeInTheDocument();
  client.clear();
});

it("does not request BusinessTask data when AI Workbench is not mounted", () => {
  fixture.context.aiWorkbenchAvailable = false;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage /></QueryClientProvider>);
  expect(screen.getByText("当前应用未启用硕米 Chat 与业务任务。")).toBeVisible();
  expect(fixture.request).not.toHaveBeenCalled();
  client.clear();
});

it("offers reconciliation, without a restart claim, for an expired running execution", async () => {
  const taskId = "550e8400-e29b-41d4-a716-446655440000";
  const item = { id: taskId, conversationId: taskId, proposalId: taskId, title: "标题任务", goalSummary: "优化标题",
    createdAt: "2026-10-03T00:00:00Z", projectionAvailable: true, state: "ERROR", reason: "EXECUTION_OUTCOME_UNKNOWN",
    canStart: false, canReconcile: true, canResume: false, canReview: false, productDetailsAvailable: false };
  fixture.request.mockImplementation(async ({ route }) => route === "task-read" ? { task: item } : {});
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage taskId={taskId} /></QueryClientProvider>);
  expect(await screen.findByRole("button", { name: "核实超期执行结果" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "启动原任务" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "核实超期执行结果" }));
  await waitFor(() => expect(fixture.request).toHaveBeenCalledWith(expect.objectContaining({
    route: "task-start", method: "POST", path: `tasks/${taskId}/start`,
  })));
  client.clear();
});

it("continues across unrelated Task pages before declaring a state view empty", async () => {
  const first = "550e8400-e29b-41d4-a716-446655440000";
  const second = "550e8400-e29b-41d4-a716-446655440001";
  const third = "550e8400-e29b-41d4-a716-446655440002";
  const task = (id: string, state: string, title: string) => ({ id, title, state, projectionAvailable: true,
    productDetailsAvailable: false, reason: "pending" });
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "task-list") throw new Error("unexpected route");
    if (path.includes(`after=${second}`)) return { tasks: [task(third, "WAITING_CONFIRMATION", "Older pending")], next: "" };
    if (path.includes(`after=${first}`)) return { tasks: [task(second, "COMPLETED", "Unrelated two")], next: second };
    return { tasks: [task(first, "COMPLETED", "Unrelated one")], next: first };
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage mode="pending" /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: /Older pending/ })).toBeVisible();
  expect(fixture.request).toHaveBeenCalledTimes(3);
  expect(screen.queryByText("当前筛选无任务")).toBeNull();
  client.clear();
});

it("shows an empty state only after every Task page has been checked", async () => {
  const cursor = "550e8400-e29b-41d4-a716-446655440000";
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "task-list") throw new Error("unexpected route");
    return path.includes(`after=${cursor}`) ? { tasks: [], next: "" }
      : { tasks: [{ id: cursor, title: "Unrelated", state: "COMPLETED" }], next: cursor };
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage mode="running" /></QueryClientProvider>);
  expect(await screen.findByText("当前筛选无任务")).toBeVisible();
  expect(fixture.request).toHaveBeenCalledTimes(2);
  client.clear();
});

it("waits for the refreshed first page before following its old cursor", async () => {
  const cursor = "550e8400-e29b-41d4-a716-446655440000";
  const task = (id: string, state: string, title: string) => ({ id, title, state, projectionAvailable: true });
  let resolveRefresh!: (page: unknown) => void;
  let firstReads = 0;
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "task-list") throw new Error("unexpected route");
    if (path.includes(`after=${cursor}`)) return { tasks: [task(cursor, "WAITING_CONFIRMATION", "Older pending")], next: "" };
    firstReads++;
    if (firstReads === 1) return { tasks: [task(cursor, "COMPLETED", "Unrelated")], next: cursor };
    return new Promise(resolve => { resolveRefresh = resolve; });
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage mode="pending" /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: /Older pending/ })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "刷新状态" }));
  await waitFor(() => expect(firstReads).toBe(2));
  expect(screen.getByText("正在查找符合筛选的任务")).toBeVisible();
  resolveRefresh({ tasks: [task(cursor, "WAITING_CONFIRMATION", "New pending")], next: cursor });
  expect(await screen.findByRole("link", { name: /New pending/ })).toBeVisible();
  expect(screen.queryByRole("link", { name: /Older pending/ })).toBeNull();
  client.clear();
});
