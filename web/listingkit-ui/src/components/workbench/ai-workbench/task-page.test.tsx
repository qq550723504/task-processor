import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { AIWorkbenchError } from "@/lib/api/ai-workbench";
import { parseAIWorkbenchResponse } from "@/lib/contracts/ai-workbench";

import { BusinessTaskPage } from "./task-page";

const fixture = vi.hoisted(() => ({
  request: vi.fn(),
  context: { user: { id: "user-a" }, effectiveOrganization: { id: "org-a", capabilities: { "workbench.chat.use": true } }, roles: ["listingkit_operator"], permissions: ["product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.commercial.read"],
    aiWorkbenchAvailable: true, aiWorkbenchPlanningReadiness: "AVAILABLE", isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null },
}));
vi.mock("@/lib/api/ai-workbench", async original => ({ ...await original<typeof import("@/lib/api/ai-workbench")>(), requestAIWorkbench: fixture.request }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));

afterEach(() => { cleanup(); sessionStorage.clear(); fixture.request.mockReset(); fixture.context.aiWorkbenchAvailable = true; fixture.context.aiWorkbenchPlanningReadiness = "AVAILABLE"; fixture.context.effectiveOrganization.capabilities["workbench.chat.use"] = true; });

it("shows freshly projected Knowledge and removes protected text after refresh", async () => {
  const taskId = "550e8400-e29b-41d4-a716-446655440000";
  const knowledge = { status: "available", originAgentRunId: taskId,
    citations: [{ id: taskId, sourceId: taskId, revisionId: taskId, name: "品牌用语", location: "document", excerpt: "受保护的品牌摘录", state: "PARTIAL" }] };
  const item = { id: taskId, conversationId: taskId, proposalId: taskId, title: "标题任务", goalSummary: "优化标题",
    createdAt: "2026-10-03T00:00:00Z", projectionAvailable: true, state: "WAITING_CONFIRMATION",
    canStart: false, canReconcile: false, canResume: false, canReview: false, productDetailsAvailable: true, knowledge };
  fixture.request.mockResolvedValue(parseAIWorkbenchResponse("task-read", 200, { task: item }));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage taskId={taskId} /></QueryClientProvider>);
  expect(await screen.findByText("受保护的品牌摘录")).toBeInTheDocument();
  expect(screen.getByText(/品牌用语/)).toBeInTheDocument();
  fixture.request.mockResolvedValue(parseAIWorkbenchResponse("task-read", 200, { task: { ...item,
    knowledge: { status: "unavailable", originAgentRunId: taskId, citations: [] } } }));
  fireEvent.click(screen.getByRole("button", { name: "刷新状态" }));
  await screen.findByText(/名称与摘录已隐藏/);
  expect(screen.queryByText("受保护的品牌摘录")).not.toBeInTheDocument();
  expect(screen.queryByText(/品牌用语/)).not.toBeInTheDocument();
  client.clear();
});

it("keeps the exact Task action key after an unknown response and page reload", async () => {
  const taskId = "550e8400-e29b-41d4-a716-446655440000";
  const item = { id: taskId, conversationId: taskId, proposalId: taskId, title: "标题任务", goalSummary: "优化标题",
    createdAt: "2026-10-03T00:00:00Z", projectionAvailable: true, state: "PAUSED", reason: "START_NOT_CLAIMED",
    canStart: true, canReconcile: false, canResume: false, canReview: false, productDetailsAvailable: true };
  const sentKeys: string[] = [];
  fixture.request.mockImplementation(async ({ route, key }) => {
    if (route === "task-read") return { task: item };
    if (route !== "task-start") throw new Error("unexpected route");
    sentKeys.push(key);
    throw new AIWorkbenchError("TASK_OUTCOME_UNKNOWN");
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const first = render(<QueryClientProvider client={client}><BusinessTaskPage taskId={taskId} /></QueryClientProvider>);
  fireEvent.click(await screen.findByRole("button", { name: "启动原任务" }));
  await waitFor(() => expect(sentKeys).toHaveLength(1));
  expect(sentKeys[0]).toMatch(/^[0-9a-f-]{36}$/);
  await screen.findByText(/操作未完成/);
  first.unmount();
  render(<QueryClientProvider client={client}><BusinessTaskPage taskId={taskId} /></QueryClientProvider>);
  fireEvent.click(await screen.findByRole("button", { name: "启动原任务" }));
  await waitFor(() => expect(sentKeys).toHaveLength(2));
  expect(sentKeys[1]).toBe(sentKeys[0]);
  client.clear();
});

it("bounds Resume feedback by UTF-8 bytes before creating an action receipt", async () => {
  const taskId = "550e8400-e29b-41d4-a716-446655440000";
  const item = { id: taskId, conversationId: taskId, proposalId: taskId, title: "标题任务", goalSummary: "优化标题",
    createdAt: "2026-10-03T00:00:00Z", projectionAvailable: true, state: "PAUSED", reason: "HUMAN_REVIEW_REQUIRED",
    agentRevision: "2", canStart: false, canReconcile: false, canResume: true, canReview: false, productDetailsAvailable: true };
  fixture.request.mockResolvedValue({ task: item });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage taskId={taskId} /></QueryClientProvider>);
  const resume = await screen.findByRole("button", { name: "继续原任务" });
  fireEvent.change(screen.getByRole("textbox", { name: "继续执行的反馈" }), { target: { value: "中".repeat(2730) } });
  expect(resume).toBeEnabled();
  fireEvent.change(screen.getByRole("textbox", { name: "继续执行的反馈" }), { target: { value: "中".repeat(2731) } });
  expect(resume).toBeDisabled();
  expect(screen.getByText(/反馈不能超过 8 KiB/)).toBeVisible();
  expect(fixture.request).toHaveBeenCalledTimes(1);
  client.clear();
});

it("keeps Task reads without offering Chat creation when this organization's planning route is unready", async () => {
  fixture.context.aiWorkbenchPlanningReadiness = "NEEDS_CONFIGURATION";
  fixture.request.mockResolvedValue({ tasks: [], next: "" });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage /></QueryClientProvider>);
  expect(await screen.findByText("当前筛选无任务")).toBeVisible();
  expect(screen.queryByRole("link", { name: "向硕米发起任务" })).not.toBeInTheDocument();
  client.clear();
});

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

it("opens only configured Review and history entries from Task Center without AI Workbench", () => {
  fixture.context.aiWorkbenchAvailable = false;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const all = render(<QueryClientProvider client={client}><BusinessTaskPage productReviewAvailable sheinRecordsAvailable /></QueryClientProvider>);
  expect(screen.getByRole("link", { name: "打开标题审核" })).toHaveAttribute("href", "/workbench/ai/tasks/pending/other");
  expect(screen.getByRole("link", { name: "查看历史记录" })).toHaveAttribute("href", "/workbench/ai/tasks/completed/history");
  expect(screen.queryByText("已加载")).not.toBeInTheDocument();
  expect(fixture.request).not.toHaveBeenCalled();
  all.unmount();

  const pending = render(<QueryClientProvider client={client}><BusinessTaskPage mode="pending" productReviewAvailable /></QueryClientProvider>);
  expect(screen.getByRole("link", { name: "打开标题审核" })).toBeVisible();
  expect(screen.queryByRole("link", { name: "查看历史记录" })).not.toBeInTheDocument();
  pending.unmount();

  render(<QueryClientProvider client={client}><BusinessTaskPage mode="completed" sheinRecordsAvailable /></QueryClientProvider>);
  expect(screen.getByRole("link", { name: "查看历史记录" })).toBeVisible();
  expect(screen.queryByRole("link", { name: "打开标题审核" })).not.toBeInTheDocument();
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

it("keeps newer Tasks visible when loading another page", async () => {
  const cursor = "550e8400-e29b-41d4-a716-446655440000";
  const newer = { id: cursor, title: "Newer Task", state: "RUNNING", projectionAvailable: true };
  const older = { id: "550e8400-e29b-41d4-a716-446655440001", title: "Older Task", state: "COMPLETED", projectionAvailable: true };
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "task-list") throw new Error("unexpected route");
    return path.includes(`after=${cursor}`) ? { tasks: [older], next: "" } : { tasks: [newer], next: cursor };
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><BusinessTaskPage /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: /Newer Task/ })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "加载更早任务" }));
  expect(await screen.findByRole("link", { name: /Older Task/ })).toBeVisible();
  expect(screen.getByRole("link", { name: /Newer Task/ })).toBeVisible();
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
