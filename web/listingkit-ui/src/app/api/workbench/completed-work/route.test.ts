// @vitest-environment node
import { NextRequest } from "next/server";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
const auth = vi.hoisted(() => ({ delay: 0, token: "server-token", calls: vi.fn() }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (request: NextRequest & { auth?: unknown }) => unknown) => async (request: NextRequest) => {
  auth.calls(); if (auth.delay) await new Promise((resolve) => setTimeout(resolve, auth.delay));
  return handler(Object.assign(request, { auth: { accessToken: auth.token } }));
} }));
import { GET, POST } from "./route";
const id = "12345678-1234-1234-1234-123456789abc";
const page = { items: [{ record_id: id, product_key: "p", snapshot_version: "1", country: "US", language: "en", created_at: "2026-09-06T00:00:00Z" }], next_cursor: null };
function request(query = "source=listing-local-preparation&limit=20", signal?: AbortSignal) { return new NextRequest(`http://localhost/api/workbench/completed-work?${query}`, { signal, headers: { cookie: "shuomi_effective_organization=200", "X-Expected-Organization-ID": "200", Authorization: "Bearer forged" } }); }
beforeEach(() => { auth.delay = 0; auth.token = "server-token"; auth.calls.mockClear(); vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:8181"); vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(page))); });
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
it("uses the actual authenticated bounded collection chain and projects its result", async () => {
  const response = await GET(request()); expect(response.status).toBe(200);
  expect(response.headers.get("cache-control")).toContain("no-store");
  expect((await response.json()).items[0].source_record_id).toBe(id);
  expect(fetch).toHaveBeenCalledOnce();
  const [url, init] = vi.mocked(fetch).mock.calls[0];
  expect(String(url)).toBe("http://127.0.0.1:8181/api/listing/shein-records?limit=20");
  expect(init).toMatchObject({ redirect: "manual", cache: "no-store", headers: { Authorization: "Bearer server-token", "X-Requested-Organization-ID": "200" } });
});
it.each(["", "source=agent", "source=listing-local-preparation&source=listing-local-preparation", "source=listing-local-preparation&status=running", "source=listing-local-preparation&limit=01", "source=listing-local-preparation&cursor=%", "source=listing-local-preparation;limit=20", `source=listing-local-preparation&cursor=${"a".repeat(1024)}`])("rejects unsupported or malformed raw query %s", async (query) => {
  expect((await GET(request(query))).status).toBe(400); expect(fetch).not.toHaveBeenCalled();
});
it("preserves revocation error and cleared organization cookie", async () => {
  vi.mocked(fetch).mockResolvedValue(Response.json({ code: "ORGANIZATION_ACCESS_REVOKED", message: "revoked", requestId: "proof", fieldErrors: [] }, { status: 403 }));
  const response = await GET(request()); expect(response.status).toBe(403);
  expect(response.headers.get("set-cookie")).toContain("Max-Age=0");
  expect((await response.json()).requestId).toBe("proof");
});
it("fails closed on invalid upstream, missing config and authentication", async () => {
  vi.mocked(fetch).mockResolvedValue(Response.json({ ...page, total: 1 })); expect((await GET(request())).status).toBe(502);
  vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", ""); expect((await GET(request())).status).toBe(502);
  auth.token = ""; expect((await GET(request())).status).toBe(401);
});
it("keeps the authentication deadline and pre-cancel boundary", async () => {
  vi.useFakeTimers(); auth.delay = 20000;
  const pending = GET(request()); await vi.advanceTimersByTimeAsync(15001); expect((await pending).status).toBe(504);
  await vi.advanceTimersByTimeAsync(5000); expect(fetch).not.toHaveBeenCalled();
  const controller = new AbortController(); controller.abort(); auth.calls.mockClear();
  expect((await GET(request(undefined, controller.signal))).status).toBe(504); expect(auth.calls).not.toHaveBeenCalled();
});
it("rejects mutation methods", async () => { expect(POST().status).toBe(405); });
