// @vitest-environment node
import { NextRequest } from "next/server";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({ delay: 0, calls: vi.fn() }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (request: NextRequest & { auth?: unknown }) => unknown) => async (request: NextRequest) => {
  auth.calls(); await new Promise((resolve) => setTimeout(resolve, auth.delay));
  return handler(Object.assign(request, { auth: { accessToken: "server-token" } }));
} }));
import { GET } from "./route";

function incoming(signal?: AbortSignal) { return new NextRequest("http://localhost/api/listing/shein-records?limit=20", { signal, headers: { cookie: "shuomi_effective_organization=200", "X-Expected-Organization-ID": "200" } }); }
beforeEach(() => { vi.useFakeTimers(); auth.delay = 20000; auth.calls.mockClear(); vi.stubGlobal("fetch", vi.fn()); });
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.unstubAllEnvs(); });

it("bounds authentication before business forwarding", async () => {
  let result: Response | undefined;
  const pending = GET(incoming()).then((response) => { result = response; });
  await vi.advanceTimersByTimeAsync(15001);
  expect(result?.status).toBe(504);
  await vi.advanceTimersByTimeAsync(5000); await pending;
  expect(fetch).not.toHaveBeenCalled();
});

it("does not start authentication for an already cancelled request", async () => {
  const controller = new AbortController(); controller.abort();
  expect((await GET(incoming(controller.signal))).status).toBe(504);
  expect(auth.calls).not.toHaveBeenCalled();
});

it("does not interpret the Next route context as a response projector", async () => {
  auth.delay = 0; vi.useRealTimers(); vi.stubEnv("SHEIN_RECORDS_API_ORIGIN", "http://127.0.0.1:8181");
  vi.mocked(fetch).mockResolvedValue(Response.json({ items: [], next_cursor: null }));
  const response = await Reflect.apply(GET, undefined, [incoming(), { params: Promise.resolve({}) }]);
  expect(response.status).toBe(200);
});
