import { afterEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
const harness = vi.hoisted(() => ({ auth: vi.fn(), proxy: vi.fn() }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (request: NextRequest) => Promise<Response>) => async (request: NextRequest) => { await harness.auth(); return handler(request); } }));
vi.mock("@/lib/server/zitadel-server-token", () => ({ readZitadelServerAccessToken: () => "fixture" }));
vi.mock("@/lib/server/zitadel-auth", () => ({ readZitadelIdentityFromSession: () => ({ userId: "user-A" }) }));
vi.mock("@/lib/server/commercial-billing-proxy", () => ({ proxyCommercialBilling: harness.proxy }));
import { GET } from "./route";
afterEach(() => { vi.useRealTimers(); vi.resetAllMocks(); });
it("bounds authentication and prevents late authentication from reading resources", async () => {
  vi.useFakeTimers(); let finish!: () => void;
  harness.auth.mockReturnValue(new Promise<void>(resolve => { finish = resolve; }));
  const pending = GET(new NextRequest("http://localhost/api/workbench/commercial/resources"));
  await vi.advanceTimersByTimeAsync(15000);
  expect((await pending).status).toBe(504);
  finish(); await vi.advanceTimersByTimeAsync(0);
  expect(harness.proxy).not.toHaveBeenCalled();
});
it("passes the actual session actor and stops cancellation during authentication", async () => {
  harness.auth.mockResolvedValue(undefined); harness.proxy.mockResolvedValue(Response.json({ fixture: true }));
  expect((await GET(new NextRequest("http://localhost/api/workbench/commercial/resources"))).status).toBe(200);
  expect(harness.proxy).toHaveBeenCalledWith(expect.any(NextRequest), "fixture", "user-A");
  harness.proxy.mockClear(); let finish!: () => void;
  harness.auth.mockReturnValue(new Promise<void>(resolve => { finish = resolve; }));
  const controller = new AbortController();
  const pending = GET(new NextRequest("http://localhost/api/workbench/commercial/resources", { signal: controller.signal }));
  controller.abort(); expect((await pending).status).toBe(504);
  finish(); await Promise.resolve(); await Promise.resolve(); expect(harness.proxy).not.toHaveBeenCalled();
});
