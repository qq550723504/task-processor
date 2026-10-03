import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { BusinessTaskPage } from "./task-page";

const fixture = vi.hoisted(() => ({
  request: vi.fn(),
  context: { user: { id: "user-a" }, effectiveOrganization: { id: "org-a" }, roles: ["listingkit_operator"],
    isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null },
}));
vi.mock("@/lib/api/ai-workbench", async original => ({ ...await original<typeof import("@/lib/api/ai-workbench")>(), requestAIWorkbench: fixture.request }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));

afterEach(() => { cleanup(); fixture.request.mockReset(); });

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
