import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { AIWorkbenchError } from "@/lib/api/ai-workbench";
import { ChatPage } from "./chat-page";

const fixture = vi.hoisted(() => ({
  request: vi.fn(), push: vi.fn(), search: new URLSearchParams(),
  context: { user: { id: "user-a" }, effectiveOrganization: { id: "org-a", capabilities: { "workbench.chat.use": true } }, roles: ["listingkit_operator"], permissions: ["product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.commercial.read"],
    aiWorkbenchAvailable: true, aiWorkbenchPlanningReadiness: "AVAILABLE", aiWorkbenchTitleReadiness: "AVAILABLE", isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null },
}));
vi.mock("@/lib/api/ai-workbench", async original => ({ ...await original<typeof import("@/lib/api/ai-workbench")>(), requestAIWorkbench: fixture.request }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: fixture.push }), useSearchParams: () => fixture.search }));

const conversationId = "550e8400-e29b-41d4-a716-446655440000";
const operationId = "11111111-1111-4111-8111-111111111111";
const conversation = { ID: conversationId, Scope: { OrganizationID: "org-a", ActorID: "user-a" }, Title: "", Favorite: false,
  Archived: false, MetadataRevision: 1, NextSequence: 1, CreatedAt: "2026-10-03T00:00:00Z", UpdatedAt: "2026-10-03T00:00:00Z" };
let client: QueryClient;
function tree() { return <QueryClientProvider client={client}><ChatPage mode="detail" conversationId={conversationId} /></QueryClientProvider>; }

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  fixture.context.effectiveOrganization = { id: "org-a", capabilities: { "workbench.chat.use": true } };
  fixture.context.roles = ["listingkit_operator"]; fixture.context.permissions = ["product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.commercial.read"];
  fixture.context.isSwitching = false;
  fixture.context.aiWorkbenchAvailable = true;
  fixture.context.aiWorkbenchPlanningReadiness = "AVAILABLE";
  fixture.context.aiWorkbenchTitleReadiness = "AVAILABLE";
  fixture.request.mockReset(); fixture.push.mockReset(); sessionStorage.clear();
  fixture.request.mockImplementation(async ({ route }) => {
    if (route === "conversation-read") return { conversation, messages: [], proposals: [], before: "" };
    throw new AIWorkbenchError("OUTCOME_UNKNOWN");
  });
});
afterEach(() => { cleanup(); client.clear(); sessionStorage.clear(); });

it("shows the unavailable state without sending Chat requests when the module is absent", () => {
  fixture.context.aiWorkbenchAvailable = false;
  render(tree());
  expect(screen.getByText("当前应用未启用硕米 Chat 与业务任务。")).toBeVisible();
  expect(fixture.request).not.toHaveBeenCalled();
});

it("keeps history readable but does not offer new planning while this organization's route needs configuration", async () => {
  fixture.context.aiWorkbenchPlanningReadiness = "NEEDS_CONFIGURATION";
  const view = render(<QueryClientProvider client={client}><ChatPage mode="home" /></QueryClientProvider>);
  await screen.findByText("开始一项新需求");
  expect(screen.queryByRole("button", { name: "进入新建会话 →" })).toBeNull();
  expect(screen.getByText(/当前企业的规划模型需要配置/)).toBeVisible();
  view.rerender(tree());
  await screen.findByText("开始讨论");
  expect(screen.queryByRole("button", { name: "发送消息" })).toBeNull();
  expect(screen.queryByLabelText("需求")).toBeNull();
  expect(fixture.request.mock.calls.every(call => call[0].route === "conversation-list" || call[0].route === "conversation-read")).toBe(true);
});

it("carries the selected saved product into the newly created conversation", async () => {
  fixture.search = new URLSearchParams(`operationId=${operationId}`);
  fixture.request.mockImplementation(async ({ route }) => {
    if (route === "conversation-list") return { conversations: [], next: "" };
    if (route === "conversation-create") return { conversation };
    throw new AIWorkbenchError("INVALID_REQUEST");
  });
  render(<QueryClientProvider client={client}><ChatPage mode="new" /></QueryClientProvider>);
  fireEvent.click(await screen.findByRole("button", { name: "新建会话" }));
  await waitFor(() => expect(fixture.push).toHaveBeenCalledWith(`/workbench/ai/chat/${conversationId}?operationId=${operationId}`));
  fixture.search = new URLSearchParams();
});

it("loads favorites before pagination so an older saved conversation is visible", async () => {
  const saved = { ...conversation, ID: "d5d9d1ca-1db3-43af-9649-dcdf3663745b", Title: "重要会话", Favorite: true };
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "conversation-list") throw new AIWorkbenchError("INVALID_REQUEST");
    return path.includes("saved=true") ? { conversations: [saved], next: "" }
      : { conversations: [conversation], next: conversation.ID };
  });
  render(<QueryClientProvider client={client}><ChatPage mode="saved" /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: /重要会话/ })).toBeVisible();
  expect(fixture.request).toHaveBeenCalledWith(expect.objectContaining({ path: expect.stringContaining("saved=true") }));
});

it("lists archived conversations through the scoped archive filter", async () => {
  const archived = { ...conversation, Title: "已归档的标题讨论", Archived: true };
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "conversation-list") throw new AIWorkbenchError("INVALID_REQUEST");
    return path.includes("archived=true") ? { conversations: [archived], next: "" }
      : { conversations: [], next: "" };
  });
  render(<QueryClientProvider client={client}><ChatPage mode="archived" /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: /已归档的标题讨论/ })).toHaveAttribute("href", `/workbench/ai/chat/${conversationId}`);
  expect(screen.getByRole("link", { name: "返回最近会话" })).toHaveAttribute("href", "/workbench/ai/chat/recent");
  expect(fixture.request).toHaveBeenCalledWith(expect.objectContaining({ path: expect.stringContaining("archived=true") }));
});

it.each(["recent", "saved", "archived"] as const)("keeps the first %s page visible after loading older conversations", async mode => {
  const first = { ...conversation, ID: "d5d9d1ca-1db3-43af-9649-dcdf3663745b", Title: "较新的会话", Favorite: mode === "saved", Archived: mode === "archived" };
  const older = { ...first, ID: "5892474d-1c8a-47e4-9550-45ed946e8197", Title: "较早的会话" };
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "conversation-list") throw new AIWorkbenchError("INVALID_REQUEST");
    return path.includes("after=older") ? { conversations: [older], next: "" } : { conversations: [first], next: "older" };
  });
  render(<QueryClientProvider client={client}><ChatPage mode={mode} /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: /较新的会话/ })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "加载更早会话" }));
  expect(await screen.findByRole("link", { name: /较早的会话/ })).toBeVisible();
  expect(screen.getByRole("link", { name: /较新的会话/ })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "刷新" }));
  await waitFor(() => expect(fixture.request.mock.calls.filter(call => call[0].route === "conversation-list" && !call[0].path.includes("after=")).length).toBeGreaterThan(1));
  expect(screen.getByRole("link", { name: /较新的会话/ })).toBeVisible();
});

it("keeps loaded conversations visible when an older page fails", async () => {
  const first = { ...conversation, Title: "已有会话" };
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "conversation-list") throw new AIWorkbenchError("INVALID_REQUEST");
    if (path.includes("after=older")) throw new AIWorkbenchError("DEPENDENCY_UNAVAILABLE");
    return { conversations: [first], next: "older" };
  });
  render(<QueryClientProvider client={client}><ChatPage mode="recent" /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: /已有会话/ })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "加载更早会话" }));
  expect(await screen.findByText(/服务暂不可用/)).toBeVisible();
  expect(screen.getByRole("link", { name: /已有会话/ })).toBeVisible();
});

it("retains the newest message and proposal while loading older messages", async () => {
  const newest = { ID: operationId, ConversationID: conversationId, Sequence: 102, Author: "USER", Content: "最新需求", CreatedAt: "2026-10-03T00:00:00Z" };
  const oldest = { ...newest, ID: "11111111-1111-4111-8111-111111111112", Sequence: 1, Content: "最早需求" };
  const proposal = { id: "d5d9d1ca-1db3-43af-9649-dcdf3663745b", digest: "a".repeat(64), sourceSequence: 102,
    goalSummary: "最新标题方案", productKey: "product", targetPlatform: "shein", humanReviewRequired: true, detailsAvailable: true, titleProfileReady: true, executionAuthorized: true };
  fixture.request.mockImplementation(async ({ route, path }) => {
    if (route !== "conversation-read") throw new AIWorkbenchError("INVALID_REQUEST");
    return path.includes("before=53") ? { conversation, messages: [oldest], proposals: [proposal], before: "" }
      : { conversation, messages: [newest], proposals: [proposal], before: "53" };
  });
  render(tree());
  expect(await screen.findByText("最新需求")).toBeVisible();
  expect(screen.getByText("最新标题方案")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "加载更早消息" }));
  expect(await screen.findByText("最早需求")).toBeVisible();
  expect(screen.getByText("最新需求")).toBeVisible();
  expect(screen.getByText("最新标题方案")).toBeVisible();
  expect(await screen.findByRole("button", { name: "确认并创建任务" })).toBeEnabled();
});

it("gates an existing proposal on title readiness independently of planning readiness", async () => {
  fixture.context.aiWorkbenchPlanningReadiness = "NEEDS_CONFIGURATION";
  fixture.context.aiWorkbenchTitleReadiness = "NEEDS_CONFIGURATION";
  const user = { ID: operationId, ConversationID: conversationId, Sequence: 1, Author: "USER", Content: "已有需求", CreatedAt: "2026-10-03T00:00:00Z" };
  const proposal = { id: "d5d9d1ca-1db3-43af-9649-dcdf3663745b", digest: "a".repeat(64), sourceSequence: 1,
    goalSummary: "已准备的方案", productKey: "product", targetPlatform: "shein", humanReviewRequired: true, detailsAvailable: true, titleProfileReady: true, executionAuthorized: true };
  fixture.request.mockImplementation(async ({ route }) => route === "conversation-read"
    ? { conversation, messages: [user], proposals: [proposal], before: "" }
    : Promise.reject(new AIWorkbenchError("INVALID_REQUEST")));
  const view = render(tree());
  expect(await screen.findByRole("button", { name: "确认并创建任务" })).toBeDisabled();
  expect(screen.getByText(/标题执行模型需要配置/)).toBeVisible();
  fixture.context.aiWorkbenchTitleReadiness = "AVAILABLE";
  view.rerender(tree());
  expect(await screen.findByRole("button", { name: "确认并创建任务" })).toBeEnabled();
  expect(screen.queryByRole("button", { name: "发送消息" })).toBeNull();
});

it("disables confirmation when the proposal's frozen title profile no longer matches the ready route", async () => {
  const proposal = { id: "d5d9d1ca-1db3-43af-9649-dcdf3663745b", digest: "a".repeat(64), sourceSequence: 1,
    goalSummary: "旧模型方案", productKey: "product", targetPlatform: "shein", humanReviewRequired: true,
    detailsAvailable: true, titleProfileReady: false, executionAuthorized: true };
  fixture.request.mockImplementation(async ({ route }) => route === "conversation-read"
    ? { conversation, messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1, Author: "USER", Content: "需求", CreatedAt: "2026-10-03T00:00:00Z" }], proposals: [proposal], before: "" }
    : Promise.reject(new AIWorkbenchError("INVALID_REQUEST")));
  render(tree());
  expect(await screen.findByRole("button", { name: "确认并创建任务" })).toBeDisabled();
  expect(screen.getByText(/提案的标题模型配置已变化/)).toBeVisible();
});

it("keeps planning available but disables execution when this actor lacks the current enterprise execution role", async () => {
  const proposal = { id: "d5d9d1ca-1db3-43af-9649-dcdf3663745b", digest: "a".repeat(64), sourceSequence: 1,
    goalSummary: "有规划权限的方案", productKey: "product", targetPlatform: "shein", humanReviewRequired: true,
    detailsAvailable: true, titleProfileReady: true, executionAuthorized: false };
  fixture.request.mockImplementation(async ({ route }) => route === "conversation-read"
    ? { conversation, messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1, Author: "USER", Content: "需求", CreatedAt: "2026-10-03T00:00:00Z" }], proposals: [proposal], before: "" }
    : Promise.reject(new AIWorkbenchError("FORBIDDEN")));
  render(tree());
  expect(await screen.findByRole("button", { name: "确认并创建任务" })).toBeDisabled();
  expect(screen.getByText(/需要当前企业的标题执行权限/)).toBeVisible();
  expect(screen.getByRole("button", { name: "发送消息" })).toBeVisible();
  expect(fixture.request).toHaveBeenCalledTimes(1);
});

it("shows chat history without mutation controls when the current organization lacks chat use", async () => {
  fixture.context.roles = ["listingkit_viewer"]; fixture.context.permissions = ["workbench.task.read","workbench.store.read","workbench.source_account.read","workbench.organization_member.read","workbench.commercial.read"];
  fixture.context.effectiveOrganization = { id: "org-a", capabilities: { "workbench.chat.use": false } };
  const proposal = { id: "d5d9d1ca-1db3-43af-9649-dcdf3663745b", digest: "a".repeat(64), sourceSequence: 1,
    goalSummary: "只读方案", productKey: "product", targetPlatform: "shein", humanReviewRequired: true, detailsAvailable: true, titleProfileReady: true, executionAuthorized: false };
  fixture.request.mockImplementation(async ({ route }) => route === "conversation-read" ? { conversation,
    messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1, Author: "USER", Content: "已有内容", CreatedAt: "2026-10-03T00:00:00Z" }],
    proposals: [proposal], before: "" } : Promise.reject(new AIWorkbenchError("FORBIDDEN")));
  render(tree());
  expect(await screen.findByText("已有内容")).toBeVisible();
  expect(screen.getByText("只读方案")).toBeVisible();
  for (const name of ["收藏", "归档", "发送消息", "确认并创建任务"]) expect(screen.queryByRole("button", { name })).toBeNull();
  expect(screen.queryByLabelText("需求")).toBeNull();
  expect(fixture.request).toHaveBeenCalledTimes(1);
});

it("restores an archived conversation using its current metadata revision", async () => {
  let current = { ...conversation, Archived: true, MetadataRevision: 2 };
  fixture.request.mockImplementation(async ({ route, method, body, revision }) => {
    if (route === "conversation-read") return { conversation: current, messages: [], proposals: [], before: "" };
    if (route === "conversation-metadata" && method === "PATCH") {
      expect(body).toEqual({ archived: false });
      expect(revision).toBe(2);
      current = { ...current, Archived: false, MetadataRevision: 3 };
      return { conversation: current };
    }
    throw new AIWorkbenchError("INVALID_REQUEST");
  });
  render(tree());
  expect(await screen.findByText("会话已归档")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "恢复会话" }));
  expect(await screen.findByLabelText("需求")).toBeVisible();
  expect(screen.queryByText("会话已归档")).toBeNull();
});

it("edits a conversation title through the existing revision-checked metadata route", async () => {
  let current = { ...conversation, Title: "原始标题", MetadataRevision: 4 };
  fixture.request.mockImplementation(async ({ route, method, body, revision }) => {
    if (route === "conversation-read") return { conversation: current, messages: [], proposals: [], before: "" };
    if (route === "conversation-metadata" && method === "PATCH") {
      expect(body).toEqual({ title: "修改后的标题" });
      expect(revision).toBe(4);
      current = { ...current, Title: "修改后的标题", MetadataRevision: 5 };
      return { conversation: current };
    }
    throw new AIWorkbenchError("INVALID_REQUEST");
  });
  render(tree());
  fireEvent.click(await screen.findByRole("button", { name: "修改标题" }));
  fireEvent.change(screen.getByRole("textbox", { name: "会话标题" }), { target: { value: "修改后的标题" } });
  fireEvent.click(screen.getByRole("button", { name: "保存标题" }));
  expect(await screen.findByText("修改后的标题")).toBeVisible();
  expect(fixture.request).toHaveBeenCalledWith(expect.objectContaining({ route: "conversation-metadata", method: "PATCH", revision: 4 }));
});

it("hides creation for a read-only organization and restores it after switching to a writable one", async () => {
  fixture.context.roles = ["listingkit_viewer"]; fixture.context.permissions = ["workbench.task.read","workbench.store.read","workbench.source_account.read","workbench.organization_member.read","workbench.commercial.read"];
  fixture.context.effectiveOrganization = { id: "org-a", capabilities: { "workbench.chat.use": false } };
  fixture.request.mockImplementation(async ({ route }) => route === "conversation-list" ? { conversations: [], next: "" }
    : Promise.reject(new AIWorkbenchError("FORBIDDEN")));
  const view = render(<QueryClientProvider client={client}><ChatPage mode="home" /></QueryClientProvider>);
  await screen.findByText("开始一项新需求");
  expect(screen.queryByRole("button", { name: "进入新建会话 →" })).toBeNull();
  fixture.context.roles = ["listingkit_operator"]; fixture.context.permissions = ["product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.commercial.read"];
  fixture.context.effectiveOrganization = { id: "org-b", capabilities: { "workbench.chat.use": true } };
  view.rerender(<QueryClientProvider client={client}><ChatPage mode="home" /></QueryClientProvider>);
  expect(await screen.findByRole("button", { name: "进入新建会话 →" })).toBeVisible();
  fixture.context.effectiveOrganization = { id: "org-b", capabilities: { "workbench.chat.use": false } };
  view.rerender(<QueryClientProvider client={client}><ChatPage mode="home" /></QueryClientProvider>);
  expect(screen.queryByRole("button", { name: "进入新建会话 →" })).toBeNull();
});

it("explains a terminal plan failure before dispatch and keeps the goal for a new attempt", async () => {
  fixture.request.mockImplementation(async ({ route }) => {
    if (route === "conversation-read") return { conversation, messages: [], proposals: [], before: "" };
    if (route === "message") return { state: "FAILED_BEFORE_DISPATCH" };
    throw new AIWorkbenchError("INVALID_REQUEST");
  });
  render(tree());
  await screen.findByText("开始讨论");
  fireEvent.change(screen.getByLabelText("采集操作 ID"), { target: { value: operationId } });
  fireEvent.change(screen.getByLabelText("需求"), { target: { value: "优化标题" } });
  fireEvent.click(screen.getByRole("button", { name: "发送消息" }));
  expect(await screen.findByText(/规划尚未发送到模型/)).toBeInTheDocument();
  expect(screen.getByLabelText("需求")).toHaveValue("优化标题");
  expect(screen.getByRole("button", { name: "发送消息" })).toBeEnabled();
});

it("explains a settled invalid planner response and releases the composer for a new intent", async () => {
  fixture.request.mockImplementation(async ({ route }) => {
    if (route === "conversation-read") return { conversation, messages: [], proposals: [], before: "" };
    if (route === "message") return { state: "PLANNER_INVALID_OUTPUT" };
    throw new AIWorkbenchError("INVALID_REQUEST");
  });
  render(tree());
  await screen.findByText("开始讨论");
  fireEvent.change(screen.getByLabelText("采集操作 ID"), { target: { value: operationId } });
  fireEvent.change(screen.getByLabelText("需求"), { target: { value: "优化标题" } });
  fireEvent.click(screen.getByRole("button", { name: "发送消息" }));
  expect(await screen.findByText(/模型返回的规划内容格式无效/)).toBeInTheDocument();
  expect(screen.getByLabelText("需求")).toHaveValue("优化标题");
  expect(screen.getByRole("button", { name: "发送消息" })).toBeEnabled();
});

it("keeps the exact message key and body across a lost response and page reload", async () => {
  const view = render(tree());
  await screen.findByText("开始讨论");
  fireEvent.change(screen.getByLabelText("采集操作 ID"), { target: { value: operationId } });
  fireEvent.change(screen.getByLabelText("需求"), { target: { value: "优化标题" } });
  fireEvent.click(screen.getByRole("button", { name: "发送消息" }));
  await screen.findByText(/结果暂无法确认/);
  const first = fixture.request.mock.calls.find(call => call[0].route === "message")?.[0];
  expect(first).toMatchObject({ key: expect.any(String), body: { content: "优化标题", operationId, targetPlatform: "shein" } });
  view.unmount();

  render(tree());
  await screen.findByRole("button", { name: "重试同一消息" });
  expect(screen.getByLabelText("需求")).toBeDisabled();
  expect(fixture.request.mock.calls.filter(call => call[0].route === "message")).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: "重试同一消息" }));
  await waitFor(() => expect(fixture.request.mock.calls.filter(call => call[0].route === "message")).toHaveLength(2));
  const second = fixture.request.mock.calls.filter(call => call[0].route === "message")[1][0];
  expect(second.key).toBe(first.key);
  expect(second.body).toEqual(first.body);
});

it("unmounts the old organization view and never sends its pending draft to the new organization", async () => {
  const view = render(tree());
  await screen.findByText("开始讨论");
  fireEvent.change(screen.getByLabelText("采集操作 ID"), { target: { value: operationId } });
  fireEvent.change(screen.getByLabelText("需求"), { target: { value: "企业甲内容" } });
  fireEvent.click(screen.getByRole("button", { name: "发送消息" }));
  await screen.findByText(/结果暂无法确认/);
  fixture.context.isSwitching = true;
  view.rerender(tree());
  expect(screen.queryByDisplayValue("企业甲内容")).toBeNull();
  fixture.context.isSwitching = false;
  fixture.context.effectiveOrganization = { id: "org-b", capabilities: { "workbench.chat.use": true } };
  fixture.request.mockImplementation(async ({ route }) => route === "conversation-read"
    ? { conversation: { ...conversation, Scope: { OrganizationID: "org-b", ActorID: "user-a" } }, messages: [], proposals: [], before: "" }
    : Promise.reject(new AIWorkbenchError("OUTCOME_UNKNOWN")));
  view.rerender(tree());
  await screen.findByText("开始讨论");
  expect(screen.queryByDisplayValue("企业甲内容")).toBeNull();
  expect(screen.getByRole("button", { name: "发送消息" })).toBeDisabled();
});

it("reuses the confirmation key after a lost response and links to the committed task", async () => {
  const proposalId = "d5d9d1ca-1db3-43af-9649-dcdf3663745b";
  const taskId = "5892474d-1c8a-47e4-9550-45ed946e8197";
  const proposal = { id: proposalId, digest: "a".repeat(64), sourceSequence: 1, goalSummary: "优化标题", productKey: "product",
    targetPlatform: "shein", humanReviewRequired: true, detailsAvailable: true, titleProfileReady: true, executionAuthorized: true };
  let confirmCalls = 0;
  fixture.request.mockImplementation(async ({ route }) => {
    if (route === "conversation-read") return { conversation, messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1,
      Author: "USER", Content: "优化标题", CreatedAt: "2026-10-03T00:00:00Z" }], proposals: [proposal], before: "" };
    if (route === "confirm") {
      confirmCalls++;
      if (confirmCalls === 1) throw new AIWorkbenchError("OUTCOME_UNKNOWN");
      return { task: { id: taskId } };
    }
    throw new AIWorkbenchError("INVALID_REQUEST");
  });
  const view = render(tree());
  await screen.findByText("待确认的执行方案");
  fireEvent.click(screen.getByRole("button", { name: "确认并创建任务" }));
  await screen.findByText(/结果暂无法确认/);
  const first = fixture.request.mock.calls.find(call => call[0].route === "confirm")?.[0];
  view.unmount();

  render(tree());
  await screen.findByRole("button", { name: "确认并创建任务" });
  fireEvent.click(screen.getByRole("button", { name: "确认并创建任务" }));
  await waitFor(() => expect(fixture.request.mock.calls.filter(call => call[0].route === "confirm")).toHaveLength(2));
  const second = fixture.request.mock.calls.filter(call => call[0].route === "confirm")[1][0];
  expect(second.key).toBe(first.key);
  expect(await screen.findByRole("link", { name: "查看已确认任务" })).toHaveAttribute("href", `/workbench/ai/tasks/${taskId}`);
  expect(fixture.push).toHaveBeenCalledWith(`/workbench/ai/tasks/${taskId}`);
});

it.each([true, false])("does not show a cached conversation after authorization changes, native role changes=%s", async (roleChanged) => {
  fixture.context.roles = ["sumi_role_0123456789abcdef0123456789abcdef_01"];
  let reads = 0;
  fixture.request.mockImplementation(async ({ route }) => {
    if (route !== "conversation-read") throw new AIWorkbenchError("INVALID_REQUEST");
    reads++;
    if (reads === 1) return { conversation, messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1,
      Author: "USER", Content: "之前有权查看的内容", CreatedAt: "2026-10-03T00:00:00Z" }], proposals: [], before: "" };
    return new Promise(() => {});
  });
  const view = render(tree());
  await screen.findByText("之前有权查看的内容");
  if (roleChanged) fixture.context.roles = ["sumi_role_0123456789abcdef0123456789abcdef_02"];
  fixture.context.permissions = fixture.context.permissions.filter(permission => permission !== "workbench.chat.read");
  view.rerender(tree());
  await screen.findByText("正在读取会话");
  expect(screen.queryByText("之前有权查看的内容")).toBeNull();
  expect(reads).toBe(2);
});

it("discards an A response that arrives after A to B to A context switching", async () => {
  let resolveOld!: (value: unknown) => void;
  const old = new Promise<unknown>(resolve => { resolveOld = resolve; });
  let aReads = 0;
  fixture.request.mockImplementation(async ({ route, scope }) => {
    if (route !== "conversation-read") throw new AIWorkbenchError("INVALID_REQUEST");
    if (scope.organizationId === "org-b") return { conversation: { ...conversation, Scope: { OrganizationID: "org-b", ActorID: "user-a" } },
      messages: [], proposals: [], before: "" };
    aReads++;
    if (aReads === 1) return old;
    return { conversation, messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1,
      Author: "USER", Content: "企业甲的当前内容", CreatedAt: "2026-10-03T00:00:00Z" }], proposals: [], before: "" };
  });
  const view = render(tree());
  await waitFor(() => expect(aReads).toBe(1));
  const oldSignal: AbortSignal = fixture.request.mock.calls[0][0].signal;
  fixture.context.isSwitching = true;
  view.rerender(tree());
  expect(oldSignal.aborted).toBe(true);
  client.clear();
  fixture.context.isSwitching = false;
  fixture.context.effectiveOrganization = { id: "org-b", capabilities: { "workbench.chat.use": true } };
  view.rerender(tree());
  await screen.findByText("开始讨论");
  fixture.context.isSwitching = true;
  view.rerender(tree());
  client.clear();
  fixture.context.isSwitching = false;
  fixture.context.effectiveOrganization = { id: "org-a", capabilities: { "workbench.chat.use": true } };
  view.rerender(tree());
  await screen.findByText("企业甲的当前内容");
  await act(async () => resolveOld({ conversation, messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1,
    Author: "USER", Content: "企业甲的过期响应", CreatedAt: "2026-10-03T00:00:00Z" }], proposals: [], before: "" }));
  expect(screen.queryByText("企业甲的过期响应")).toBeNull();
  expect(screen.getByText("企业甲的当前内容")).toBeVisible();
});
