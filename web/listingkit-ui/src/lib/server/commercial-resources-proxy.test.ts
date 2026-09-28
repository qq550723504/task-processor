import { afterEach, expect, it, vi } from "vitest";
import { commercialResourcesFixture } from "@/test/fixtures/commercial-resources";
import { proxyCommercialBilling } from "./commercial-billing-proxy";

afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
const request = (query = "", headers: Record<string, string> = {}) => new Request(`http://localhost/api/workbench/commercial/resources${query}`, { headers: { cookie: "shuomi_effective_organization=org-A", "X-Expected-User-ID": "user-A", "X-Expected-Organization-ID": "org-A", ...headers } });
it("forwards only the fixed org-bound resource GET and validates the complete schema", async () => {
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888");
  const value = commercialResourcesFixture("org-A");
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json(value)); vi.stubGlobal("fetch", fetcher);
  const response = await proxyCommercialBilling(request(), "fixture", "user-A");
  expect(response.status).toBe(200); expect(await response.json()).toEqual(value);
  expect(fetcher.mock.calls[0][0].toString()).toBe("http://localhost:8888/api/v1/workbench/commercial/resources");
  expect(new Headers(fetcher.mock.calls[0][1].headers).get("X-Requested-Organization-ID")).toBe("org-A");
  for (const payload of [{ ...value, organization_id: "org-B" }, { ...value, resources: value.resources.slice(1) }, { ...value, resources: [value.resources[0], value.resources[1], value.resources[1]] }, { ...value, resources: value.resources.map(v => ({ ...v, unit: "wrong" })) }]) {
    fetcher.mockResolvedValueOnce(Response.json(payload)); expect((await proxyCommercialBilling(request(), "fixture", "user-A")).status).toBe(502);
  }
  fetcher.mockResolvedValueOnce(new Response(" ".repeat(16385) + JSON.stringify(value), { headers: { "content-type": "application/json" } }));
  expect((await proxyCommercialBilling(request(), "fixture", "user-A")).status).toBe(502);
  fetcher.mockResolvedValueOnce(new Response(JSON.stringify(value).replace('"organization_id":"org-A"', '"organization_id":"org-A","organization_id":"org-A"'), { headers: { "content-type": "application/json" } }));
  expect((await proxyCommercialBilling(request(), "fixture", "user-A")).status).toBe(502);
});
it("classifies malformed upstream quantities as invalid responses, not unavailable", async () => {
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888"); const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  const value = commercialResourcesFixture("org-A");
  for (const field of ["available", "reserved", "consumed", "debt"]) for (const invalid of ["bad", "1e3", "1.5"]) {
    fetcher.mockResolvedValueOnce(Response.json({ ...value, resources: value.resources.map(v => v.resource_type === "ai_point" ? { ...v, [field]: invalid } : v) }));
    const response = await proxyCommercialBilling(request(), "fixture", "user-A");
    expect(response.status).toBe(502); expect(await response.json()).toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
  }
  for (const invalid of ["2026-13-01T00:00:00Z", "2026-09-28T99:00:00Z", "2026bad"]) {
    for (const payload of [{ ...value, observed_at: invalid }, { ...value, resources: value.resources.map(v => v.resource_type === "ai_point" ? { ...v, updated_at: invalid } : v) }]) {
      fetcher.mockResolvedValueOnce(Response.json(payload)); expect((await proxyCommercialBilling(request(), "fixture", "user-A")).status).toBe(502);
    }
  }
});
it("preserves a validated permission denial and rejects malformed error envelopes", async () => {
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888"); const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  fetcher.mockResolvedValueOnce(Response.json({ code: "PERMISSION_DENIED", message: "Permission is denied", requestId: "request-1", fieldErrors: [] }, { status: 403 }));
  expect((await proxyCommercialBilling(request(), "fixture", "user-A")).status).toBe(403);
  fetcher.mockResolvedValueOnce(Response.json({ arbitrary: "private upstream detail" }, { status: 503 }));
  const failed = await proxyCommercialBilling(request(), "fixture", "user-A");
  expect(failed.status).toBe(502); expect(await failed.text()).not.toContain("private upstream");
});
it("denies stale identities, duplicate assertions/cookies and query input before fetching", async () => {
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888");const fetcher = vi.fn();vi.stubGlobal("fetch", fetcher);
  expect((await proxyCommercialBilling(request(), "fixture", "user-B")).status).toBe(409);
  const assertions: Record<string, string>[] = [{ "X-Expected-User-ID": "user-A, user-A" }, { "X-Expected-Organization-ID": "org-A, org-A" }, { cookie: "shuomi_effective_organization=org-A; shuomi_effective_organization=org-A" }];
  for (const headers of assertions) expect((await proxyCommercialBilling(request("", headers), "fixture", "user-A")).status).toBe(409);
  for (const query of ["?organization_id=org-B", "?"]) expect((await proxyCommercialBilling(request(query), "fixture", "user-A")).status).toBe(400);
  expect(fetcher).not.toHaveBeenCalled();
});
