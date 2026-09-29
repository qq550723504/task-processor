import { afterEach, describe, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { getCommercialOverview, parseCommercialOverview } from "./commercial";
afterEach(() => vi.unstubAllGlobals());
describe("unified commercial contract", () => {
  it("reads fixed base plan and independent native components", () => {
    const value = commercialOverviewFixture();
    expect(parseCommercialOverview(value)).toEqual(value);
    expect(
      parseCommercialOverview({
        ...value,
        resources: { state: "unavailable", value: null },
      })?.resources.state,
    ).toBe("unavailable");
    expect(
      parseCommercialOverview({ ...value, plans: [], subscription: null }),
    ).toBeNull();
    expect(
      parseCommercialOverview({
        ...value,
        base_plan: { ...value.base_plan, subscription_required: true },
      }),
    ).toBeNull();
    expect(
      parseCommercialOverview({
        ...value,
        resources: {
          state: "available",
          value: { ...value.resources.value, organization_id: "other" },
        },
      }),
    ).toBeNull();
    expect(
      parseCommercialOverview({
        ...value,
        store_services: { state: "unavailable", value: { records: 0 } },
      }),
    ).toBeNull();
    expect(
      parseCommercialOverview({
        ...value,
        store_services: {
          state: "available",
          value: { records: 1, active: 2, expired: 0, expiring_soon: 1 },
        },
      }),
    ).toBeNull();
  });
  it("preserves exact resource quantities and rejects malformed counters", () => {
    const value = commercialOverviewFixture();
    expect(
      parseCommercialOverview(value)?.resources.value?.resources[1].available,
    ).toBe("9007199254740993");
    for (const invalid of ["bad", "01", "-1", "9223372036854775808"]) {
      const input = structuredClone(value);
      if (input.resources.state === "available")
        input.resources.value.resources[1].available = invalid;
      expect(() => parseCommercialOverview(input)).not.toThrow();
      expect(parseCommercialOverview(input)).toBeNull();
    }
  });
  it("rejects another organization and cancelled responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(Response.json(commercialOverviewFixture("other"))),
    );
    await expect(getCommercialOverview("org-B")).rejects.toMatchObject({
      code: "INVALID_UPSTREAM_RESPONSE",
    });
    const aborted = new AbortController();
    aborted.abort();
    await expect(
      getCommercialOverview("org-B", aborted.signal),
    ).rejects.toMatchObject({ code: "DEADLINE_EXCEEDED" });
  });
  it("requires bounded strict JSON and never follows a redirect", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(Response.json(commercialOverviewFixture()));
    vi.stubGlobal("fetch", fetcher);
    await expect(getCommercialOverview("org-B")).resolves.toMatchObject({
      organization_id: "org-B",
    });
    expect(fetcher.mock.calls[0][1]).toMatchObject({
      redirect: "manual",
      cache: "no-store",
    });
    fetcher.mockResolvedValue(
      new Response('{"organization_id":"org-B","organization_id":"other"}', {
        headers: { "content-type": "application/json" },
      }),
    );
    await expect(getCommercialOverview("org-B")).rejects.toMatchObject({
      code: "INVALID_UPSTREAM_RESPONSE",
    });
  });
});
