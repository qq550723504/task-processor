import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { AIWorkbenchError } from "@/lib/api/ai-workbench";
import { ChatPage } from "./chat-page";

const fixture = vi.hoisted(() => ({
  request: vi.fn(), push: vi.fn(), search: new URLSearchParams(),
  context: { user: { id: "user-a" }, effectiveOrganization: { id: "org-a" }, roles: ["listingkit_operator"],
    isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null },
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
  fixture.context.effectiveOrganization = { id: "org-a" };
  fixture.context.roles = ["listingkit_operator"];
  fixture.context.isSwitching = false;
  fixture.request.mockReset(); fixture.push.mockReset(); sessionStorage.clear();
  fixture.request.mockImplementation(async ({ route }) => {
    if (route === "conversation-read") return { conversation, messages: [], proposals: [], before: "" };
    throw new AIWorkbenchError("OUTCOME_UNKNOWN");
  });
});
afterEach(() => { cleanup(); client.clear(); sessionStorage.clear(); });

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
  fixture.context.effectiveOrganization = { id: "org-b" };
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
    targetPlatform: "shein", humanReviewRequired: true, detailsAvailable: true };
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

it("does not show a cached conversation when the current role set changes", async () => {
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
  fixture.context.roles = ["listingkit_viewer"];
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
  fixture.context.effectiveOrganization = { id: "org-b" };
  view.rerender(tree());
  await screen.findByText("开始讨论");
  fixture.context.isSwitching = true;
  view.rerender(tree());
  client.clear();
  fixture.context.isSwitching = false;
  fixture.context.effectiveOrganization = { id: "org-a" };
  view.rerender(tree());
  await screen.findByText("企业甲的当前内容");
  await act(async () => resolveOld({ conversation, messages: [{ ID: operationId, ConversationID: conversationId, Sequence: 1,
    Author: "USER", Content: "企业甲的过期响应", CreatedAt: "2026-10-03T00:00:00Z" }], proposals: [], before: "" }));
  expect(screen.queryByText("企业甲的过期响应")).toBeNull();
  expect(screen.getByText("企业甲的当前内容")).toBeVisible();
});
