import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {act, cleanup, render, screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {afterEach, expect, it, vi} from "vitest";
import {OfficialKnowledgePage} from "./official-knowledge-page";

const fixture = vi.hoisted(() => ({context: {user: {id: "reader"}, effectiveOrganization: {id: "org-a", permissions: ["workbench.knowledge.read"]}, isLoading: false, isSwitching: false, error: null, blockingError: null, retry: vi.fn()}}));
vi.mock("@/components/providers/workbench-context-provider", () => ({useWorkbenchContext: () => fixture.context}));
const summary = {id: "ai-commerce-guide", revision: "1", title: "AI电商应用指南", summary: "知识选择、标题优化与人工审核", category: "AI电商应用指南", updatedAt: "2026-10-11", maintainedBy: "硕米", digest: "a".repeat(64)};
const article = {...summary, body: "Human Review\n<script>unsafe()</script>", sources: [{title: "仓库设计依据", url: "https://github.com/qq550723504/task-processor/blob/60d432aa/docs/architecture/agent-knowledge-context-v1.md"}]};
afterEach(() => {cleanup(); vi.unstubAllGlobals(); fixture.context.effectiveOrganization = {id: "org-a", permissions: ["workbench.knowledge.read"]}; fixture.context.isSwitching = false; fixture.context.retry.mockReset();});
function mount(id?: string, revision?: string) {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  const view = render(<QueryClientProvider client={client}><OfficialKnowledgePage articleId={id} revision={revision}/></QueryClientProvider>);
  return {...view, client, refresh: () => view.rerender(<QueryClientProvider client={client}><OfficialKnowledgePage articleId={id} revision={revision}/></QueryClientProvider>)};
}
it("browses real versioned content independently of enterprise storage and offers no AI selection", async () => {
  const fetcher = vi.fn(async () => Response.json({items: [summary]})); vi.stubGlobal("fetch", fetcher);
  mount(); await screen.findByRole("heading", {name: summary.title});
  expect(screen.getByRole("link", {name: "查看内容"})).toHaveAttribute("href", "/workbench/ai/knowledge/official/ai-commerce-guide/revisions/1");
  expect(screen.getByText(/v1/)).toHaveTextContent("2026-10-11");
  expect(screen.getByText(/AI 引用尚未开放/)).toBeVisible();
  expect(screen.queryByRole("button", {name: "选择使用"})).not.toBeInTheDocument();
  const [url, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit];
  expect(url).toBe("/api/workbench/official-knowledge");
  expect(new Headers(init.headers).get("X-Expected-User-ID")).toBe("reader");
  expect(new Headers(init.headers).get("X-Expected-Organization-ID")).toBe("org-a");
});
it("shows exact revision and sources as inert text", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => Response.json(article))); mount(article.id, article.revision);
  expect(await screen.findByRole("region", {name: "官方资料正文"})).toHaveTextContent("Human Review"); expect(document.querySelector("pre script")).toBeNull();
  expect(screen.getByRole("link", {name: "仓库设计依据"})).toHaveAttribute("href", article.sources[0].url);
  expect(screen.getByText(/SHA-256/)).toHaveTextContent(article.digest);
});
it("hides cached content on a denied refresh and requires fresh context confirmation", async () => {
  let denied = false; const fetcher = vi.fn(async () => denied ? Response.json({code: "PERMISSION_DENIED"}, {status: 403}) : Response.json(article)); vi.stubGlobal("fetch", fetcher);
  const view = mount(article.id, article.revision); await screen.findByRole("region", {name: "官方资料正文"}); denied = true;
  await act(async () => {await view.client.invalidateQueries({queryKey: ["official-knowledge"]});});
  await screen.findByText("当前身份没有知识库读取权限"); expect(screen.queryByRole("region", {name: "官方资料正文"})).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", {name: "重新确认"})); expect(fixture.context.retry).toHaveBeenCalled();
});
it("cancels a late read on organization switch and never reveals revoked data", async () => {
  let signal: AbortSignal | undefined, release!: (r: Response) => void;
  const late = new Promise<Response>(resolve => {release = resolve;});
  vi.stubGlobal("fetch", vi.fn(async (_url: string, init?: RequestInit) => {signal = init?.signal ?? undefined; return late;}));
  const view = mount(); await waitFor(() => expect(signal).toBeDefined());
  fixture.context.isSwitching = true; view.refresh(); expect(signal?.aborted).toBe(true);
  await act(async () => release(Response.json({items: [summary]}))); expect(screen.queryByText(summary.title)).not.toBeInTheDocument();
  fixture.context.isSwitching = false; fixture.context.effectiveOrganization.permissions = []; view.refresh();
  expect(screen.getByText("当前身份没有知识库读取权限")).toBeVisible();
});
it("does not substitute another version when the exact article is absent", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => Response.json({code: "OFFICIAL_KNOWLEDGE_NOT_FOUND"}, {status: 404})));
  mount(article.id, "2"); await screen.findByText("此版本的官方资料不存在"); expect(screen.queryByText(article.body)).not.toBeInTheDocument();
});
