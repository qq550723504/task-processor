import { afterEach, expect, it, vi } from "vitest";
import { proxyStoreObservations } from "./store-observations-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
const id = "d6f6ca0a-27e2-4c4a-b1aa-4505110ae635";
const headers = {
  "X-Expected-Organization-ID": "org-a",
  "X-Expected-User-ID": "actor-a",
  cookie: WORKBENCH_COOKIE_NAME + "=org-a",
  Origin: "http://localhost:3000",
};
function configure() {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000");
}
it("keeps original scopes and only forwards readonly endpoints", async () => {
  configure();
  const fetch = vi
    .fn()
    .mockResolvedValue(
      Response.json({
        organizationId: "org-a",
        userId: "actor-a",
        data: {
          available: true,
          canSync: true,
          kind: "orders",
          site: "shein-us",
          platformUrl: "https://sellerhub.shein.com/",
        },
      }),
    );
  vi.stubGlobal("fetch", fetch);
  expect(
    (
      await proxyStoreObservations(
        new Request(
          "http://localhost:3000/api/workbench/store-observations/orders/capabilities",
          { headers },
        ),
        "private",
        "actor-a",
      )
    ).status,
  ).toBe(200);
  expect(fetch.mock.calls[0][0].pathname).toBe(
    "/api/v1/workbench/store-observations/orders/capabilities",
  );
  expect(
    new Headers(fetch.mock.calls[0][1].headers).get(
      "X-Requested-Organization-ID",
    ),
  ).toBe("org-a");
  fetch.mockResolvedValueOnce(
    Response.json({
      organizationId: "other",
      userId: "actor-a",
      data: {
        available: true,
        canSync: true,
        kind: "orders",
        site: "shein-us",
        platformUrl: "https://sellerhub.shein.com/",
      },
    }),
  );
  expect(
    (
      await proxyStoreObservations(
        new Request(
          "http://localhost:3000/api/workbench/store-observations/orders/capabilities",
          { headers },
        ),
        "private",
        "actor-a",
      )
    ).status,
  ).toBe(409);
});
it("refuses mutation, untrusted transport and stale selected scopes before dispatch", async () => {
  configure();
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  for (const [path, init, status] of [
    ["orders/ship", { method: "POST" }, 400],
    ["orders?waybillNo=foreign", {}, 400],
    ["orders?storeId=" + id + "&storeId=" + id, {}, 400],
    [
      "orders",
      { headers: { ...headers, cookie: WORKBENCH_COOKIE_NAME + "=other" } },
      409,
    ],
    [
      "orders/syncs",
      {
        method: "POST",
        headers: { ...headers, Origin: "https://foreign.test" },
      },
      403,
    ],
  ] as const) {
    expect(
      (
        await proxyStoreObservations(
          new Request(
            "http://localhost:3000/api/workbench/store-observations/" + path,
            { headers, ...init },
          ),
          "private",
          "actor-a",
        )
      ).status,
    ).toBe(status);
  }
  expect(fetch).not.toHaveBeenCalled();
});
it("rejects overposted owner and duplicate JSON rather than issuing a new operation", async () => {
  configure();
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  for (const body of [
    '{"kind":"orders","stores":[],"organizationId":"other"}',
    '{"kind":"orders","kind":"products","stores":[]}',
  ]) {
    expect(
      (
        await proxyStoreObservations(
          new Request(
            "http://localhost:3000/api/workbench/store-observations/orders/syncs",
            {
              method: "POST",
              headers: {
                ...headers,
                "Content-Type": "application/json",
                "Idempotency-Key": id,
              },
              body,
            },
          ),
          "private",
          "actor-a",
        )
      ).status,
    ).toBe(400);
  }
  expect(fetch).not.toHaveBeenCalled();
});
