import { afterEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
const harness = vi.hoisted(() => ({ auth: vi.fn(), proxy: vi.fn() }));
vi.mock("@/auth", () => ({
  serverAuth:
    (handler: (request: NextRequest) => Promise<Response>) =>
    async (request: NextRequest) => {
      await harness.auth();
      return handler(request);
    },
}));
vi.mock("@/lib/server/zitadel-server-token", () => ({
  readZitadelServerAccessToken: () => "fixture",
}));
vi.mock("@/lib/server/zitadel-auth", () => ({
  readZitadelIdentityFromSession: () => ({ userId: "user-A" }),
}));
vi.mock("@/lib/server/commercial-billing-proxy", () => ({
  proxyCommercialBilling: harness.proxy,
}));
import { GET as resourcesGET } from "./route";
import { GET as offersGET } from "../resource-offers/route";
import { GET as eventsGET } from "./events/route";
const routes = [
  [
    "resources",
    resourcesGET,
    "http://localhost/api/workbench/commercial/resources",
  ],
  [
    "resource offers",
    offersGET,
    "http://localhost/api/workbench/commercial/resource-offers",
  ],
  [
    "resource events",
    eventsGET,
    "http://localhost/api/workbench/commercial/resources/events",
  ],
] as const;
afterEach(() => {
  vi.useRealTimers();
  vi.resetAllMocks();
});
it.each(routes)(
  "%s bounds authentication and prevents late authentication from reading upstream",
  async (_name, GET, url) => {
    vi.useFakeTimers();
    let finish!: () => void;
    harness.auth.mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const pending = GET(new NextRequest(url));
    await vi.advanceTimersByTimeAsync(15000);
    expect((await pending).status).toBe(504);
    finish();
    await vi.advanceTimersByTimeAsync(0);
    expect(harness.proxy).not.toHaveBeenCalled();
  },
);
it.each(routes)(
  "%s passes the actual session actor and stops cancellation during authentication",
  async (_name, GET, url) => {
    harness.auth.mockResolvedValue(undefined);
    harness.proxy.mockResolvedValue(Response.json({ fixture: true }));
    expect((await GET(new NextRequest(url))).status).toBe(200);
    expect(harness.proxy).toHaveBeenCalledWith(
      expect.any(NextRequest),
      "fixture",
      "user-A",
    );
    harness.proxy.mockClear();
    let finish!: () => void;
    harness.auth.mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const controller = new AbortController();
    const pending = GET(new NextRequest(url, { signal: controller.signal }));
    controller.abort();
    expect((await pending).status).toBe(504);
    finish();
    await Promise.resolve();
    await Promise.resolve();
    expect(harness.proxy).not.toHaveBeenCalled();
  },
);
