// @vitest-environment node
import { NextRequest } from "next/server";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({ delay: 0, reject: false, calls: vi.fn() }));
vi.mock("@/auth", () => ({
  serverAuth: (handler: (request: NextRequest & { auth?: unknown }, context: unknown) => unknown) => async (request: NextRequest, context: unknown) => {
    auth.calls();
    await new Promise((resolve) => setTimeout(resolve, auth.delay));
    if (auth.reject) throw new Error("private authentication failure");
    return handler(Object.assign(request, { auth: { accessToken: "server-token" } }), context);
  },
}));
import { GET } from "./route";
const id = "12345678-1234-4234-8234-123456789abc";
const context = { params: Promise.resolve({ record_id: id }) };
function incoming(signal?: AbortSignal) {
  return new NextRequest(`http://localhost/api/listing/shein-records/${id}/offline-diagnostic?action=publish`, { signal, headers: { cookie: "shuomi_effective_organization=200", "X-Expected-Organization-ID": "200" } });
}
beforeEach(() => { vi.useFakeTimers(); auth.calls.mockClear(); auth.delay = 20000; auth.reject = false; vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:9999"); vi.stubGlobal("fetch", vi.fn()); });
afterEach(() => { vi.useRealTimers(); vi.unstubAllEnvs(); vi.unstubAllGlobals(); });
it("bounds authentication wait and ignores a late authenticated success", async () => {
  let result: Response | undefined;
  const pending = Promise.resolve(GET(incoming(), context)).then((response) => { result = response as Response; });
  await vi.advanceTimersByTimeAsync(15001);
  const atDeadline = result;
  await vi.advanceTimersByTimeAsync(5000); await pending;
  expect(atDeadline?.status).toBe(504);
  expect(atDeadline?.headers.get("cache-control")).toContain("no-store");
  expect(fetch).not.toHaveBeenCalled();
});
it("ends the response during authentication cancellation, without late upstream work", async () => {
  const controller = new AbortController();
  let result: Response | undefined;
  const pending = Promise.resolve(GET(incoming(controller.signal), context)).then((response) => { result = response as Response; });
  controller.abort(); await vi.advanceTimersByTimeAsync(1);
  const atCancellation = result;
  await vi.advanceTimersByTimeAsync(20000); await pending;
  expect(atCancellation?.status).toBe(504);
  expect(fetch).not.toHaveBeenCalled();
});
it("does not start authentication for an already cancelled request", async () => {
  const controller = new AbortController(); controller.abort();
  const pending = Promise.resolve(GET(incoming(controller.signal), context));
  await vi.advanceTimersByTimeAsync(20001);
  expect((await pending as Response).status).toBe(504);
  expect(auth.calls).not.toHaveBeenCalled();
});
it("shares the remaining budget with proxy response streaming after authentication", async () => {
  auth.delay = 10000;
  const cancel = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(new ReadableStream({ cancel }), { headers: { "Content-Type": "application/json" } })));
  let result: Response | undefined;
  const pending = Promise.resolve(GET(incoming(), context)).then((response) => { result = response as Response; });
  await vi.advanceTimersByTimeAsync(15001);
  const atDeadline = result;
  await vi.advanceTimersByTimeAsync(10000); await pending;
  expect(atDeadline?.status).toBe(504);
  expect(cancel).toHaveBeenCalledOnce();
});
it("consumes a late authentication rejection without replacing the deadline response", async () => {
  auth.reject = true;
  const pending = GET(incoming(), context);
  await vi.advanceTimersByTimeAsync(15001);
  const response = await pending;
  expect(response.status).toBe(504);
  await vi.advanceTimersByTimeAsync(5000);
  expect(await response.text()).not.toContain("private");
  expect(fetch).not.toHaveBeenCalled();
});
