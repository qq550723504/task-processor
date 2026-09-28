import { afterEach, it, expect, vi } from "vitest";
import { proxyAccountFacts } from "./account-facts-proxy";
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
it("saves only self preferences without an enterprise and refuses duplicate fields", async () => {
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000");
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  const fetch = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) =>
    Promise.resolve(
      Response.json({
        schemaVersion: "account-preferences-v1",
        userId: "self",
        country: "China",
        province: "",
        city: "杭州",
        updatedAt: "2026-09-28T00:00:00Z",
        readAt: "2026-09-28T00:00:00Z",
        source: "account_profile",
      }),
    ),
  );
  vi.stubGlobal("fetch", fetch);
  const make = (body: string, origin = "http://localhost:3000") =>
    new Request("http://localhost:3000/api/account/preferences", {
      method: "PUT",
      headers: {
        Origin: origin,
        "X-Expected-User-ID": "self",
        "Content-Type": "application/json",
      },
      body,
    });
  expect(
    (
      await proxyAccountFacts(
        make('{"country":"China","province":"","city":"杭州"}'),
        "token",
        "self",
      )
    ).status,
  ).toBe(200);
  expect(
    new Headers(fetch.mock.calls[0][1]?.headers).has(
      "X-Requested-Organization-ID",
    ),
  ).toBe(false);
  for (const body of [
    '{"country":"China","country":"foreign","province":"","city":""}',
    '{"country":"China","province":"","city":"","userId":"foreign"}',
  ])
    expect((await proxyAccountFacts(make(body), "token", "self")).status).toBe(
      400,
    );
  expect(
    (
      await proxyAccountFacts(
        make("{}", "http://foreign.test"),
        "token",
        "self",
      )
    ).status,
  ).toBe(403);
  expect(fetch).toHaveBeenCalledTimes(1);
});
