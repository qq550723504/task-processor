// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { proxySheinDiagnostic } from "./shein-diagnostic-proxy";

const id = "12345678-1234-4234-8234-123456789abc";
function request(query = "action=publish", headers: HeadersInit = {}) {
  return new Request(`http://localhost/api/listing/shein-records/${id}/offline-diagnostic?${query}`, { headers: { cookie: "shuomi_effective_organization=200; secret=private", "X-Expected-Organization-ID": "200", ...headers } });
}
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });
describe("dedicated diagnostic proxy trust boundary", () => {
  it.each(["", "action=apply", "action=publish&action=save_draft", "action=publish&tenant=100", "action=publish&expected_digest=", `action=publish&junk=${"x".repeat(1024)}`, "action=publish&%xx=1"])("rejects invalid query without forwarding: %s", async (query) => {
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
    expect((await proxySheinDiagnostic(request(query), id, "server-token")).status).toBe(400);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("never trusts browser bearer and requires session and matching organization before forwarding", async () => {
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
    expect((await proxySheinDiagnostic(request(undefined, { Authorization: "Bearer browser" }), id, "")).status).toBe(401);
    expect((await proxySheinDiagnostic(request(undefined, { "X-Expected-Organization-ID": "100" }), id, "server-token")).status).toBe(409);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("rejects invalid record IDs and body metadata without forwarding", async () => {
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
    expect((await proxySheinDiagnostic(request(), "../secret", "token")).status).toBe(400);
    expect((await proxySheinDiagnostic(request(undefined, { "Content-Length": "1" }), id, "token")).status).toBe(400);
    expect((await proxySheinDiagnostic(request(undefined, { "Transfer-Encoding": "chunked" }), id, "token")).status).toBe(400);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("propagates inbound cancellation to fetch and a stalled stream", async () => {
    vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
    const controller = new AbortController();
    const cancel = vi.fn();
    const fetcher = vi.fn().mockResolvedValue(new Response(new ReadableStream({ cancel }), { headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetcher);
    const incoming = new Request(request(), { signal: controller.signal });
    const pending = proxySheinDiagnostic(incoming, id, "token");
    await vi.waitFor(() => expect(fetcher).toHaveBeenCalledOnce());
    controller.abort();
    expect((await pending).status).toBe(504);
    expect(fetcher.mock.calls[0][1].signal.aborted).toBe(true);
    expect(cancel).toHaveBeenCalledOnce();
    fetcher.mockClear();
    expect((await proxySheinDiagnostic(incoming, id, "token")).status).toBe(504);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it.each([undefined, "http://localhost:8085/api/v1", "https://user:pass@example.com", "https://example.com/?url=evil"])("fails closed for missing/non-origin config %s", async (origin) => {
    vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", origin);
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
    expect((await proxySheinDiagnostic(request(), id, "server-token")).status).toBe(502);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("rebuilds narrow headers and preserves typed errors without cookies or redirects", async () => {
    vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
    const fetcher = vi.fn().mockResolvedValue(Response.json({ error: "not_found" }, { status: 404, headers: { "set-cookie": "secret=1", "x-private": "leak" } })); vi.stubGlobal("fetch", fetcher);
    const response = await proxySheinDiagnostic(request(undefined, { Authorization: "Bearer browser", "X-Requested-Organization-ID": "100", "X-User-ID": "admin" }), id, "server-token");
    expect(response.status).toBe(404);
    expect(await response.json()).toEqual({ error: "not_found" });
    expect(response.headers.get("cache-control")).toContain("no-store");
    expect(response.headers.has("set-cookie")).toBe(false);
    const [url, init] = fetcher.mock.calls[0];
    expect(url.toString()).toBe(`http://127.0.0.1:9876/api/listing/shein-records/${id}/offline-diagnostic?action=publish`);
    expect(Object.fromEntries(new Headers(init.headers))).toEqual({ accept: "application/json", authorization: "Bearer server-token", "x-requested-organization-id": "200" });
    expect(init).toMatchObject({ method: "GET", redirect: "manual", cache: "no-store" });
  });
  it.each([new Response("redirect", { status: 302 }), Response.json({ error: "not_found", sql: "secret" }, { status: 404 }), new Response('{"error":"not_found","error":"not_found"}', { status: 404, headers: { "Content-Type": "application/json" } })])("rejects invalid upstream without raw data", async (upstream) => {
    vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876"); vi.stubGlobal("fetch", vi.fn().mockResolvedValue(upstream));
    const response = await proxySheinDiagnostic(request(), id, "token");
    expect(response.status).toBe(502);
    expect((await response.json()).code).toBe("INVALID_UPSTREAM_RESPONSE");
  });
  it("bounds actual streamed bytes even with lying Content-Length", async () => {
    vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
    const cancel = vi.fn();
    const body = new ReadableStream({ start(c) { c.enqueue(new Uint8Array(2 * 1024 * 1024 + 1)); }, cancel });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(body, { headers: { "Content-Type": "application/json", "Content-Length": "1" } })));
    expect((await proxySheinDiagnostic(request(), id, "token")).status).toBe(502);
    expect(cancel).toHaveBeenCalled();
  });
  it("classifies upstream stream resets as dependency outages, without raw transport details", async () => {
    vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(new ReadableStream({ start(c) { c.error(new TypeError("private upstream reset")); } }), { headers: { "Content-Type": "application/json" } })));
    const response = await proxySheinDiagnostic(request(), id, "token");
    expect(response.status).toBe(502);
    expect(await response.json()).toEqual({ code: "DEPENDENCY_UNAVAILABLE", message: "Diagnostic upstream is unavailable", requestId: "", fieldErrors: [] });
  });
  it("times out a stalled response stream and aborts upstream", async () => {
    vi.useFakeTimers(); vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9876");
    const cancel = vi.fn(); const fetcher = vi.fn().mockResolvedValue(new Response(new ReadableStream({ cancel }), { headers: { "Content-Type": "application/json" } })); vi.stubGlobal("fetch", fetcher);
    const pending = proxySheinDiagnostic(request(), id, "token");
    await vi.advanceTimersByTimeAsync(15001);
    expect((await pending).status).toBe(504);
    expect(fetcher.mock.calls[0][1].signal.aborted).toBe(true);
    expect(cancel).toHaveBeenCalled();
  });
});
