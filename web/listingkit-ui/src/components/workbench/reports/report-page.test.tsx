import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, expect, it, vi } from "vitest";
import { ReportPage } from "./report-page";
import { type Report } from "@/lib/contracts/report-center";
const fixture = vi.hoisted(() => ({ context: { user: { id: "actor-a" }, effectiveOrganization: { id: "org-a" }, roles: ["listingkit_operator"], permissions: ["workbench.report.read", "workbench.report.manage"], isSwitching: false, isLoading: false, selectionRequired: false, error: null, blockingError: null, aiWorkbenchAvailable: false }, sourceList: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));
vi.mock("@/lib/api/product-title-review-client", () => ({ fetchProductTitleProposals: fixture.sourceList }));
const source = { ref: { kind: "TITLE_REVIEW" as const, id: "550e8400-e29b-41d4-a716-446655440000", version: "2:accepted" }, title: "标题审核 · 商品A", productKey: "product-a", storeId: "" };
const report: Report = { ...source, id: "550e8400-e29b-41d4-a716-446655440001", capturedAt: "2026-10-10T00:00:00Z", favorite: false, content: { schemaVersion: 1, sections: [{ title: "标题历史", fields: [{ label: "建议标题", value: "已保存的标题内容" }] }] }, digest: "a".repeat(64) };
beforeEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); sessionStorage.clear(); fixture.context.effectiveOrganization = { id: "org-a" }; fixture.context.permissions = ["workbench.report.read", "workbench.report.manage"]; fixture.sourceList.mockResolvedValue({ items: [{ proposal_id: source.ref.id }], next_cursor: null }); });
const tree = (client: QueryClient) => <QueryClientProvider client={client}><ReportPage view="all" /></QueryClientProvider>;
it.each(["applied", "rejected"])("saves a %s Review from its existing detail link after it leaves the actionable collection", async state => {
  fixture.sourceList.mockResolvedValue({ items: [], next_cursor: null });
  const terminal = { ...source, ref: { ...source.ref, version: `2:${state}` } }, terminalReport = { ...report, ...terminal };
  const bodies: unknown[] = [];
  vi.stubGlobal("fetch", vi.fn(async (path: string, init: RequestInit) => {
    if (init.method === "POST") { bodies.push(JSON.parse(String(init.body))); return Response.json({ commandId: new Headers(init.headers).get("Idempotency-Key"), report: terminalReport, replayed: false }); }
    if (path.includes("/sources/")) return path.endsWith(source.ref.id) ? Response.json(terminal) : Response.json({ code: "NOT_FOUND" }, { status: 404 });
    if (path.endsWith("/summary")) return Response.json({ saved: 0, recent: 0, favorites: 0, stores: 0 });
    if (path.endsWith(report.id)) return Response.json(terminalReport);
    return Response.json({ items: [], nextCursor: "" });
  }));
  const client = new QueryClient(), view = render(tree(client));
  await screen.findByText("当前范围暂无报告");
  await waitFor(() => expect(screen.getByRole("button", { name: "保存报告" })).toBeEnabled());
  fireEvent.click(await screen.findByRole("button", { name: "保存报告" }));
  fireEvent.click(screen.getByRole("button", { name: "已有审核详情" }));
  const input = screen.getByLabelText("标题审核详情链接或 ID");
  fireEvent.change(input, { target: { value: "/workbench/ai/tasks/pending/other?proposal_id=550e8400-e29b-41d4-a716-446655440099" } });
  fireEvent.click(screen.getByRole("button", { name: "读取当前详情" }));
  await screen.findByText("来源读取失败"); expect(screen.queryByRole("button", { name: "保存此版本" })).not.toBeInTheDocument();
  fireEvent.change(input, { target: { value: `/workbench/ai/tasks/pending/other?proposal_id=${source.ref.id}` } });
  fireEvent.click(screen.getByRole("button", { name: "读取当前详情" }));
  await screen.findByText(`版本 2:${state}`); expect(bodies).toHaveLength(0);
  fireEvent.click(screen.getByRole("button", { name: "保存此版本" }));
  await within(await screen.findByRole("region", { name: "报告详情" })).findByText(`2:${state}`);
  expect(bodies).toEqual([{ source: terminal.ref }]); view.unmount(); client.clear();
});
it("manually saves an exact version, reads it, favorites it and downloads the same snapshot", async () => {
  let saved = false, favorite = false; const bodies: { path: string; key: string; body: object }[] = [];
  vi.stubGlobal("fetch", vi.fn(async (path: string, init: RequestInit) => {
    if (init.method === "POST") { const body = JSON.parse(String(init.body)); const key = new Headers(init.headers).get("Idempotency-Key")!; bodies.push({ path, key, body }); if (path.endsWith("/favorite")) favorite = body.favorite; else saved = true; return Response.json({ commandId: key, report: { ...report, favorite }, replayed: false }); }
    if (path.includes("/sources/")) return Response.json(source);
    if (path.endsWith("/summary")) return Response.json({ saved: saved ? 1 : 0, recent: saved ? 1 : 0, favorites: favorite ? 1 : 0, stores: 0 });
    if (path.endsWith(report.id)) return Response.json({ ...report, favorite });
    const { content, digest, ...summary } = { ...report, favorite }; void content; void digest; return Response.json({ items: saved ? [summary] : [], nextCursor: "" });
  }));
  const create = vi.fn<(blob: Blob) => string>(() => "blob:report"); Object.defineProperty(URL, "createObjectURL", { configurable: true, value: create }); Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() }); vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  const client = new QueryClient(), view = render(tree(client));
  await screen.findByText("当前范围暂无报告"); fireEvent.click(screen.getByRole("button", { name: "保存报告" })); fireEvent.click(await screen.findByRole("button", { name: "保存此版本" }));
  const region = await screen.findByRole("region", { name: "报告详情" }); await within(region).findByText("已保存的标题内容"); expect(within(region).getByText("2:accepted")).toBeInTheDocument(); expect(bodies[0].body).toEqual({ source: source.ref });
  fireEvent.click(within(region).getByRole("button", { name: "收藏报告" })); await within(region).findByRole("button", { name: "取消收藏" }); expect(bodies[1].body).toEqual({ favorite: true }); expect(bodies[1].key).not.toBe(bodies[0].key);
  fireEvent.click(within(region).getByRole("button", { name: "下载 JSON" })); fireEvent.click(within(region).getByRole("button", { name: "下载文本" })); expect(create).toHaveBeenCalledTimes(2); expect((create.mock.calls[0][0] as Blob).type).toBe("application/json;charset=utf-8");
  expect(within(region).getByRole("link", { name: "打开原来源" })).toHaveAttribute("href", `/workbench/ai/tasks/pending/other?proposal_id=${source.ref.id}`); expect(sessionStorage.length).toBe(0); view.unmount(); client.clear();
});
it("hides historical content immediately when enterprise or report permission changes", async () => {
  const { content, digest, ...summary } = report; void content; void digest;
  vi.stubGlobal("fetch", vi.fn(async (path: string, init: RequestInit) => {
    const org = new Headers(init.headers).get("X-Expected-Organization-ID"); if (path.endsWith("/summary")) return Response.json({ saved: 1, recent: 1, favorites: 0, stores: 0 }); if (path.endsWith(report.id)) return Response.json(report); return Response.json({ items: org === "org-a" ? [summary] : [], nextCursor: "" });
  }));
  const client = new QueryClient(), view = render(tree(client)); fireEvent.click(await screen.findByRole("button", { name: source.title })); await screen.findByText("已保存的标题内容");
  fixture.context.effectiveOrganization = { id: "org-b" }; view.rerender(tree(client)); expect(screen.queryByText("已保存的标题内容")).not.toBeInTheDocument(); await screen.findByText("当前范围暂无报告");
  fixture.context.permissions = []; view.rerender(tree(client)); expect(screen.getByText("报告尚不可用")).toBeInTheDocument(); expect(screen.queryByRole("button", { name: "保存报告" })).not.toBeInTheDocument(); view.unmount(); client.clear();
});
it("recovers a saved unknown intent with its original key after remount", async () => {
  const key = "550e8400-e29b-41d4-a716-446655440002"; const intent = { operation: "save", key, source: source.ref }; sessionStorage.setItem("personal-report-intent:actor-a:org-a", JSON.stringify(intent));
  const fetch = vi.fn(async (path: string, init: RequestInit) => { if (init.method === "POST") throw new Error("lost"); if (path.endsWith("/summary")) return Response.json({ saved: 0, recent: 0, favorites: 0, stores: 0 }); return Response.json({ items: [], nextCursor: "" }); }); vi.stubGlobal("fetch", fetch);
  const client = new QueryClient(), view = render(tree(client)); fireEvent.click(await screen.findByRole("button", { name: "恢复原请求" })); await screen.findByText(/操作结果尚未确认/); expect(screen.getByRole("button", { name: "保存报告" })).toBeDisabled();
  await waitFor(() => expect(fetch.mock.calls.some(([,init]) => init.method === "POST")).toBe(true)); const posted = fetch.mock.calls.find(([,init]) => init.method === "POST")![1]; expect(new Headers(posted.headers).get("Idempotency-Key")).toBe(key); expect(JSON.parse(String(posted.body))).toEqual({ source: source.ref }); expect(JSON.parse(sessionStorage.getItem("personal-report-intent:actor-a:org-a")!)).toEqual(intent); view.unmount(); client.clear();
});
