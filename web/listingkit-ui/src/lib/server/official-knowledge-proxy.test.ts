import { afterEach, expect, it, vi } from "vitest";
import { proxyKnowledge } from "./knowledge-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
const headers = { "X-Expected-User-ID": "actor", "X-Expected-Organization-ID": "org", cookie: WORKBENCH_COOKIE_NAME + "=org" };
const summary = { id: "ai-commerce-guide", revision: "1", title: "AI电商应用指南", summary: "真实使用说明", category: "AI电商应用指南", updatedAt: "2026-10-11", maintainedBy: "硕米", digest: "a".repeat(64) };
function request(path = "", extra: HeadersInit = {}) {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  return new Request("http://localhost/api/workbench/official-knowledge" + path, { headers: { ...headers, ...extra } });
}
it("forwards official reads with the current server token and organization, independent of enterprise storage", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json({ items: [summary] })); vi.stubGlobal("fetch", fetch);
  const response = await proxyKnowledge(request(), "server-token", "actor");
  expect(response.status).toBe(200);
  expect(await response.json()).toEqual({ items: [summary] });
  expect(fetch.mock.calls[0][0]).toBe("http://127.0.0.1:9000/api/v1/workbench/official-knowledge");
  expect(new Headers(fetch.mock.calls[0][1].headers).get("Authorization")).toBe("Bearer server-token");
  expect(new Headers(fetch.mock.calls[0][1].headers).get("X-Requested-Organization-ID")).toBe("org");
  expect(response.headers.get("Cache-Control")).toContain("no-store");
});
it("rejects a non-producing GET body without reading it or forwarding", async () => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  const pull = vi.fn(() => new Promise<void>(() => {}));
  const stream = new ReadableStream({ pull }, { highWaterMark: 0 });
  const req = request(); Object.defineProperty(req, "body", { value: stream });
  const response = await proxyKnowledge(req, "server-token", "actor");
  expect(response.status).toBe(400); expect(pull).not.toHaveBeenCalled(); expect(fetch).not.toHaveBeenCalled();
});
it("rejects stale organization, invalid revisions, queries and mutation paths before forwarding", async () => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  for (const path of ["?tenant=other", "/ai-commerce-guide/revisions/01", "/../secret", "/ai-commerce-guide/latest"]) {
    expect((await proxyKnowledge(request(path), "server-token", "actor")).status).toBe(400);
  }
  expect((await proxyKnowledge(request("", { "X-Expected-Organization-ID": "other" }), "server-token", "actor")).status).toBe(409);
  expect(fetch).not.toHaveBeenCalled();
});
it("rejects an article returned for another exact revision or identity", async () => {
  const article = { ...summary, body: "正文", sources: [{ title: "依据", url: "https://github.com/qq550723504/task-processor/blob/main/docs/architecture/agent-knowledge-context-v1.md" }] };
  for (const changed of [{ revision: "2" }, { id: "other-guide" }]) {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...article, ...changed })));
    expect((await proxyKnowledge(request("/ai-commerce-guide/revisions/1"), "server-token", "actor")).status).toBe(502);
  }
});
it("rejects oversized and ambiguous upstream JSON instead of accepting a partial article", async () => {
  for (const body of [JSON.stringify({ items: [summary], padding: "x".repeat(128 * 1024) }), '{"items":[],"items":[]}']) {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(body, { headers: { "Content-Type": "application/json" } })));
    expect((await proxyKnowledge(request(), "server-token", "actor")).status).toBe(502);
  }
});
it("enforces the official body's byte limit for Chinese text", async () => {
  const article = {...summary, body: "字".repeat(22 * 1024), sources: [{title: "依据", url: "https://github.com/qq550723504/task-processor/blob/main/docs/architecture/agent-knowledge-context-v1.md"}]};
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(article)));
  expect((await proxyKnowledge(request("/ai-commerce-guide/revisions/1"), "server-token", "actor")).status).toBe(502);
});
