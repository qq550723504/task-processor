import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { recordListFixture } from "@/test/fixtures/shein-records";
import { SheinRecordListPage } from "./record-list-page";

vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ user: { id: "reader" }, effectiveOrganization: { id: "200", name: "企业甲" }, roles: [], retry: vi.fn() }) }));
let client: QueryClient;
beforeEach(() => { client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); });
afterEach(() => { cleanup(); client.clear(); vi.unstubAllGlobals(); });
function show(payload: unknown, status = 200) {
  vi.stubGlobal("fetch", vi.fn(async () => Response.json(payload, { status })));
  render(<QueryClientProvider client={client}><SheinRecordListPage available /></QueryClientProvider>);
}

it("consumes the actual #327 client and decoder, retaining a version above Number.MAX_SAFE_INTEGER", async () => {
  show(recordListFixture());
  expect(await screen.findByText("9007199254740993")).toBeVisible();
});

it.each([
  ["numeric version", () => ({ ...recordListFixture(), items: [{ ...recordListFixture().items[0], snapshot_version: 9007199254740992 }] })],
  ["missing record identity", () => ({ ...recordListFixture(), items: [{ ...recordListFixture().items[0], record_id: undefined }] })],
  ["missing creation time", () => ({ ...recordListFixture(), items: [{ ...recordListFixture().items[0], created_at: undefined }] })],
  ["invalid collection", () => ({ items: null, next_cursor: null })],
])("does not turn %s into an empty list or clickable record", async (_label, payload) => {
  show(payload());
  expect(await screen.findByRole("alert")).toHaveTextContent("资料列表响应不合法");
  expect(screen.queryByText("当前范围内暂无本地资料")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
});

it("uses the actual structured permission error without exposing the raw message", async () => {
  show({ code: "PERMISSION_DENIED", message: "private upstream context", requestId: "test", fieldErrors: [] }, 403);
  expect(await screen.findByRole("alert")).toHaveTextContent("没有读取本地资料的权限");
  expect(screen.queryByText("private upstream context")).not.toBeInTheDocument();
});
