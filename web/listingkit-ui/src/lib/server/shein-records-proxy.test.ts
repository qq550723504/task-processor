// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { proxySheinRecords } from "./shein-records-proxy";

const sheinRecordListFixture = () => ({ items: [{ record_id: "12345678-1234-4234-8234-123456789abc", product_key: "source-product", snapshot_version: "1", country: "US", language: "en", created_at: "2026-09-06T01:02:03Z" }], next_cursor: "opaque-cursor_1" });

function request(query = "limit=20", headers: HeadersInit = {}) {
  return new Request(`http://localhost/api/listing/shein-records${query ? `?${query}` : ""}`, { headers: { cookie: "shuomi_effective_organization=200; private=x", "X-Expected-Organization-ID": "200", ...headers } });
}
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });

it.each(["limit=0", "limit=020", "limit=101", "limit=20&limit=20", "cursor=", "unknown=x", `cursor=${"x".repeat(513)}`, "%xx=1"])("rejects invalid query without forwarding: %s", async (query) => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  expect((await proxySheinRecords(request(query), "token")).status).toBe(400);
  expect(fetcher).not.toHaveBeenCalled();
});

it("requires server session and matching selected organization", async () => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  expect((await proxySheinRecords(request(), "")).status).toBe(401);
  expect((await proxySheinRecords(request(undefined, { "X-Expected-Organization-ID": "100" }), "token")).status).toBe(409);
  expect(fetcher).not.toHaveBeenCalled();
});

it("rebuilds fixed path and headers and preserves strict response", async () => {
  vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
  const fetcher = vi.fn().mockResolvedValue(Response.json(sheinRecordListFixture())); vi.stubGlobal("fetch", fetcher);
  const response = await proxySheinRecords(request("limit=20&cursor=opaque-cursor_1", { Authorization: "Bearer browser", "X-Requested-Organization-ID": "100" }), "server-token");
  expect(response.status).toBe(200);
  expect(await response.json()).toEqual(sheinRecordListFixture());
  const [upstream, init] = fetcher.mock.calls[0];
  expect(upstream.toString()).toBe("http://127.0.0.1:9876/api/listing/shein-records?limit=20&cursor=opaque-cursor_1");
  expect(Object.fromEntries(new Headers(init.headers))).toEqual({ accept: "application/json", authorization: "Bearer server-token", "x-requested-organization-id": "200" });
  expect(init).toMatchObject({ method: "GET", cache: "no-store", redirect: "manual" });
  expect(response.headers.get("cache-control")).toContain("no-store");
});

it("fails closed for config, invalid JSON, duplicate fields and actual byte overflow", async () => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  expect((await proxySheinRecords(request(), "token")).status).toBe(502);
  vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
  for (const response of [
    new Response("redirect", { status: 302 }),
    new Response('{"items":[],"items":[],"next_cursor":null}', { headers: { "Content-Type": "application/json" } }),
    new Response(new Uint8Array(128 * 1024 + 1), { headers: { "Content-Type": "application/json", "Content-Length": "1" } }),
  ]) {
    fetcher.mockResolvedValueOnce(response);
    const result = await proxySheinRecords(request(), "token");
    expect(result.status).toBe(502);
    expect((await result.json()).code).toBe("INVALID_UPSTREAM_RESPONSE");
  }
});

it("preserves typed Go errors without private upstream headers", async () => {
  vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ error: "unavailable" }, { status: 503, headers: { "set-cookie": "private=x" } })));
  const response = await proxySheinRecords(request(), "token");
  expect(response.status).toBe(503);
  expect(await response.json()).toEqual({ error: "unavailable" });
  expect(response.headers.has("set-cookie")).toBe(false);
});
