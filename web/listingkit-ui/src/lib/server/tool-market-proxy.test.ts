import { beforeEach, describe, expect, it, vi } from "vitest";
import { proxyToolMarket } from "./tool-market-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const headers = {
  "X-Expected-User-ID": "actor",
  "X-Expected-Organization-ID": "org-a",
  cookie: `${WORKBENCH_COOKIE_NAME}=org-a`,
  origin: "https://app.example.com",
  "Content-Type": "application/json",
  "Idempotency-Key": "1510eced-9831-49de-a28c-098cb16deba1",
  "If-None-Match": "*",
};
const request = (body: string) =>
  new Request("https://app.example.com/api/tool-market/requests", {
    method: "POST",
    headers,
    body,
  });
describe("tool market trusted proxy", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    process.env.LISTINGKIT_SERVICE_API_BASE = "http://localhost:8080/api/v1";
    process.env.LISTINGKIT_PUBLIC_BASE_URL = "https://app.example.com";
  });
  it("rejects duplicate fields, organization override and stale scope before dispatch", async () => {
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    for (const body of [
      '{"kind":"DATA","kind":"DATA","title":"x","description":"x"}',
      '{"kind":"DATA","title":"x","description":"x","organizationId":"org-b"}',
    ])
      expect(
        (await proxyToolMarket(request(body), "token", "actor")).status,
      ).toBe(400);
    const drift = request('{"kind":"DATA","title":"x","description":"x"}');
    drift.headers.set("X-Expected-Organization-ID", "org-b");
    expect((await proxyToolMarket(drift, "token", "actor")).status).toBe(409);
    expect(fetch).not.toHaveBeenCalled();
  });
  it("retains uncertain dispatched write outcomes", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("lost")));
    const r = await proxyToolMarket(
      request('{"kind":"DATA","title":"x","description":"x"}'),
      "token",
      "actor",
    );
    expect(await r.json()).toEqual({ code: "OUTCOME_UNKNOWN" });
  });
  it("platform queries do not require a customer organization cookie", async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue(Response.json({ items: [], nextCursor: "" }));
    vi.stubGlobal("fetch", fetch);
    const r = await proxyToolMarket(
      new Request("https://app.example.com/api/tool-market/admin/requests", {
        headers: { "X-Expected-User-ID": "actor" },
      }),
      "token",
      "actor",
    );
    expect(r.status).toBe(200);
    expect(fetch.mock.calls[0][0]).toBe(
      "http://localhost:8080/api/v1/admin/tool-market/requests",
    );
    expect(
      new Headers(fetch.mock.calls[0][1].headers).has(
        "X-Requested-Organization-ID",
      ),
    ).toBe(false);
  });
  it("forwards only exact bounded event cursors for enterprise and platform details", async () => {
    const id = "aa1043df-c64c-499d-9c99-57598f7bff18";
    const payload = {
      request: {
        id,
        organizationId: "org-a",
        kind: "DATA",
        title: "需求",
        description: "说明",
        stage: "SUBMITTED",
        revision: "100",
        createdAt: "2026-10-09T00:00:00Z",
        updatedAt: "2026-10-09T00:00:00Z",
      },
      events: [],
      nextEventsBefore: "",
    };
    const fetch = vi.fn(async (url: string) => {
      expect(url).toContain("?eventsBefore=85");
      return Response.json(payload);
    });
    vi.stubGlobal("fetch", fetch);
    for (const prefix of ["", "admin/"]) {
      const send = (query: string) =>
        proxyToolMarket(
          new Request(
            `https://app.example.com/api/tool-market/${prefix}requests/${id}?${query}`,
            { headers: prefix ? { "X-Expected-User-ID": "actor" } : headers },
          ),
          "token",
          "actor",
        );
      expect((await send("eventsBefore=85")).status).toBe(200);
      expect(fetch.mock.lastCall?.[0]).toBe(
        `http://localhost:8080/api/v1/${prefix ? "admin" : "workbench"}/tool-market/requests/${id}?eventsBefore=85`,
      );
      const calls = fetch.mock.calls.length;
      for (const query of [
        "eventsBefore=0",
        "eventsBefore=01",
        "eventsBefore=-1",
        "eventsBefore=9223372036854775808",
        "eventsBefore=85&eventsBefore=69",
        "eventsBefore=85&organizationId=other",
        "pageSize=50",
      ])
        expect((await send(query)).status).toBe(400);
      expect(fetch).toHaveBeenCalledTimes(calls);
    }
  });
});
