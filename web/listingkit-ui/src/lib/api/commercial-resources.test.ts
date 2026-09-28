import { afterEach, expect, it, vi } from "vitest";
import { commercialResourcesFixture } from "@/test/fixtures/commercial-resources";
import { getCommercialResources, parseCommercialResources } from "./commercial-billing";

afterEach(() => vi.unstubAllGlobals());
it("preserves owner zero, absent records, debt and int64 precision", () => {
  const value = commercialResourcesFixture();
  expect(parseCommercialResources(value)?.resources[1].available).toBe("9007199254740993");
  expect(parseCommercialResources(value)?.resources[2].state).toBe("not_recorded");
  expect(parseCommercialResources({ ...value, resources: value.resources.map(v => v.resource_type === "ai_point" ? { ...v, available: "0", debt: "7" } : v) })?.resources[1].available).toBe("0");
});
it("rejects partial, duplicate, malformed and internally inconsistent balances", () => {
  const value = commercialResourcesFixture();
  const invalid = [
    { ...value, resources: value.resources.slice(1) }, { ...value, resources: [value.resources[0], value.resources[1], value.resources[1]] },
    ...[{ unit: "row" }, { available: "01" }, { available: "-1" }, { available: 1 }, { available: "9223372036854775808" }, { debt: "1" }, { updated_at: null }, { updated_at: "2026-02-30T00:00:00Z" }, { extra: true }].map(change => ({ ...value, resources: value.resources.map(v => v.resource_type === "ai_point" ? { ...v, ...change } : v) })),
    { ...value, resources: value.resources.map(v => v.resource_type === "data_row" ? { ...v, available: "0" } : v) }, { ...value, extra: true },
  ];
  for (const bad of invalid) expect(parseCommercialResources(bad)).toBeNull();
});
it("binds resources to identity and org, rejects duplicate JSON and the 16KiB limit", async () => {
  const value = commercialResourcesFixture("org-A");
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json(value)).mockResolvedValueOnce(Response.json({ ...value, organization_id: "org-B" })).mockResolvedValueOnce(new Response(JSON.stringify(value).replace('"organization_id":"org-A"', '"organization_id":"org-A","organization_id":"org-A"'), { headers: { "content-type": "application/json" } })).mockResolvedValueOnce(new Response(" ".repeat(16385) + JSON.stringify(value), { headers: { "content-type": "application/json" } }));
  vi.stubGlobal("fetch", fetcher);
  expect((await getCommercialResources("user-A", "org-A")).resources[1].available).toBe("9007199254740993");
  expect(fetcher.mock.calls[0]).toEqual(["/api/workbench/commercial/resources", expect.objectContaining({ cache: "no-store", redirect: "manual", headers: expect.objectContaining({ "X-Expected-User-ID": "user-A", "X-Expected-Organization-ID": "org-A" }) })]);
  for (let i = 0; i < 3; i++) await expect(getCommercialResources("user-A", "org-A")).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
});
it("never throws for malformed quantities or timestamps", () => {
  const value = commercialResourcesFixture();
  for (const field of ["available", "reserved", "consumed", "debt"]) for (const invalid of ["bad", "1e3", "1.5"]) {
    expect(parseCommercialResources({ ...value, resources: value.resources.map(v => v.resource_type === "ai_point" ? { ...v, [field]: invalid } : v) })).toBeNull();
  }
  for (const invalid of ["2026-invalid", "2026bad", "2026-13-01T00:00:00Z", "2026-09-28T99:00:00Z", "2026-02-30T00:00:00Z"]) {
    expect(parseCommercialResources({ ...value, observed_at: invalid })).toBeNull();
    expect(parseCommercialResources({ ...value, resources: value.resources.map(v => v.resource_type === "ai_point" ? { ...v, updated_at: invalid } : v) })).toBeNull();
  }
});
