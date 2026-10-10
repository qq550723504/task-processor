import { beforeEach, expect, it, vi } from "vitest";
import { proxyReportCenter } from "./report-center-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const id = "1510eced-9831-49de-a28c-098cb16deba1";
const source = { kind: "TITLE_REVIEW", id, version: "2:accepted" };
const headers = { "X-Expected-User-ID": "actor", "X-Expected-Organization-ID": "org-a", cookie: `${WORKBENCH_COOKIE_NAME}=org-a`, origin: "https://app.example.com", "Content-Type": "application/json", "Idempotency-Key": id };
const write = (body = JSON.stringify({ source })) => new Request("https://app.example.com/api/report-center/reports", { method: "POST", headers, body });
beforeEach(() => { vi.unstubAllGlobals(); process.env.LISTINGKIT_SERVICE_API_BASE = "http://localhost:8080/api/v1"; process.env.LISTINGKIT_PUBLIC_BASE_URL = "https://app.example.com"; });
it("rejects spoofed scope, duplicate/unknown JSON and unsupported paths before dispatch", async () => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  for (const body of [`{"source":${JSON.stringify(source)},"source":${JSON.stringify(source)}}`, JSON.stringify({ source, organizationId: "other" }), "{\"favorite\":true}"]) expect((await proxyReportCenter(write(body), "token", "actor")).status).toBe(400);
  const drift = write(); drift.headers.set("X-Expected-Organization-ID", "other"); expect((await proxyReportCenter(drift, "token", "actor")).status).toBe(409);
  const crossOrigin = write(); crossOrigin.headers.set("origin", "https://other.example.com"); expect((await proxyReportCenter(crossOrigin, "token", "actor")).status).toBe(403);
  for (const path of ["/../accounts", "?limit=20&limit=30", "?actor=other", "?search=" + "中".repeat(50)]) expect((await proxyReportCenter(new Request("https://app.example.com/api/report-center/reports" + path, { headers }), "token", "actor")).status).toBe(400);
  expect(fetch).not.toHaveBeenCalled();
});
it("preserves unknown writes on lost responses, malformed results and receipt mismatch", async () => {
  for (const response of [Promise.reject(new Error("lost")), Promise.resolve(Response.json({ commandId: id })), Promise.resolve(Response.json({ code: "DEPENDENCY_UNAVAILABLE" }, { status: 503 }))]) {
    const fetch = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(() => response); vi.stubGlobal("fetch", fetch);
    const result = await proxyReportCenter(write(), "token", "actor"); expect(await result.json()).toEqual({ code: "OUTCOME_UNKNOWN" });
    const init = fetch.mock.calls[0][1] as RequestInit; expect(init.redirect).toBe("manual"); expect(new Headers(init.headers).get("Authorization")).toBe("Bearer token"); expect(new Headers(init.headers).get("X-Requested-Organization-ID")).toBe("org-a");
  }
});
it("validates the requested detail and does not forward browser identity headers", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json({ saved: 0, recent: 0, favorites: 0, stores: 0 })); vi.stubGlobal("fetch", fetch);
  const response = await proxyReportCenter(new Request("https://app.example.com/api/report-center/reports/summary", { headers }), "token", "actor"); expect(response.status).toBe(200); expect(response.headers.get("cache-control")).toBe("private, no-store"); expect(new Headers(fetch.mock.calls[0][1].headers).has("X-Expected-User-ID")).toBe(false);
});
