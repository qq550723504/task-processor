import { afterEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const harness = vi.hoisted(() => ({ authenticate: vi.fn(), proxy: vi.fn() }));
vi.mock("@/auth", () => ({
  serverAuth: (handler: (request: NextRequest) => Promise<Response>) =>
    async (request: NextRequest) => {
      await harness.authenticate();
      return handler(request);
    },
}));
vi.mock("./zitadel-server-token", () => ({ readZitadelServerAccessToken: () => "fixture-token" }));
vi.mock("./commercial-proxy", () => ({ proxyCommercialRead: harness.proxy }));
import { commercialGET } from "./commercial-route";

afterEach(() => { vi.useRealTimers(); vi.resetAllMocks(); });
const request = (signal?: AbortSignal) => new NextRequest("http://localhost/api/workbench/commercial/overview", { signal });

it("includes stalled authentication in the single deadline and stops late auth from reading", async () => {
  vi.useFakeTimers();
  let finishAuth!: () => void;
  harness.authenticate.mockReturnValue(new Promise<void>(resolve => { finishAuth = resolve; }));
  const pending = commercialGET(request());
  await vi.advanceTimersByTimeAsync(15000);
  const response = await pending;
  expect(response.status).toBe(504);
  expect((await response.json()).code).toBe("DEADLINE_EXCEEDED");
  finishAuth();
  await vi.advanceTimersByTimeAsync(0);
  expect(harness.proxy).not.toHaveBeenCalled();
  expect(vi.getTimerCount()).toBe(0);
});

it("cancels during authentication and does not start a read after cancellation", async () => {
  let finishAuth!: () => void;
  harness.authenticate.mockReturnValue(new Promise<void>(resolve => { finishAuth = resolve; }));
  const controller = new AbortController();
  const pending = commercialGET(request(controller.signal));
  controller.abort();
  expect((await pending).status).toBe(504);
  finishAuth();
  await Promise.resolve();
  await Promise.resolve();
  expect(harness.proxy).not.toHaveBeenCalled();
});

it("sanitizes authentication failures and passes successful requests to the scoped proxy", async () => {
  harness.authenticate.mockRejectedValueOnce(new Error("private session details"));
  const failed = await commercialGET(request());
  expect(failed.status).toBe(503);
  expect(await failed.text()).not.toContain("private session details");
  harness.authenticate.mockResolvedValue(undefined);
  harness.proxy.mockResolvedValue(Response.json({ fixture: true }));
  expect((await commercialGET(request())).status).toBe(200);
  expect(harness.proxy.mock.calls[0][0].signal.aborted).toBe(false);
  expect(harness.proxy.mock.calls[0][1]).toBe("fixture-token");
});
