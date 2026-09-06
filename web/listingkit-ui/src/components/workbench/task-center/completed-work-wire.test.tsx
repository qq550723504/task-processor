import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { completedWorkFixture } from "@/test/fixtures/completed-work";
import { CompletedWorkPageContent } from "./completed-work-page";

vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ user: { id: "reader" }, effectiveOrganization: { id: "200", name: "企业甲" }, roles: [], retry: vi.fn() }) }));
let client: QueryClient;
beforeEach(() => { client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); });
afterEach(() => { cleanup(); client.clear(); vi.unstubAllGlobals(); });

it.each([
  ["numeric version", () => ({ ...completedWorkFixture(), items: [{ ...completedWorkFixture().items[0], snapshot_version: 9007199254740992 }] })],
  ["different source UUID in href", () => ({ ...completedWorkFixture(), items: [{ ...completedWorkFixture().items[0], result: completedWorkFixture(null, "2").items[0].result }] })],
  ["missing coverage", () => ({ ...completedWorkFixture(), coverage: undefined })],
  ["invalid collection", () => ({ items: null, next_cursor: null })],
])("actual #340 decoder rejects %s before any selectable UI", async (_label, payload) => {
  vi.stubGlobal("fetch", vi.fn(async () => Response.json(payload())));
  render(<QueryClientProvider client={client}><CompletedWorkPageContent available completed /></QueryClientProvider>);
  expect(await screen.findByRole("alert")).toHaveTextContent("工作记录响应不合法");
  expect(screen.queryByText("当前授权范围内暂无本地资料准备记录")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
});
