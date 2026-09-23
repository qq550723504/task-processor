import { afterEach, expect, it, vi } from "vitest";
import { proxyCommercialBilling } from "./commercial-billing-proxy";

afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.unstubAllEnvs(); });

function postRequest(body: ReadableStream<Uint8Array>): Request {
  return new Request("http://localhost/api/workbench/commercial/orders", {
    method: "POST",
    headers: {
      cookie: "shuomi_effective_organization=org-a",
      "X-Expected-Organization-ID": "org-a",
      "X-Expected-User-ID": "user-a",
      "content-type": "application/json",
    },
    body,
    duplex: "half",
  } as RequestInit);
}

it("cancels oversized request streams before forwarding", async () => {
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888");
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  let cancelled = false;
  const request = postRequest(new ReadableStream<Uint8Array>({
    pull(controller) { controller.enqueue(new Uint8Array(16 * 1024 + 1)); },
    cancel() { cancelled = true; },
  }));

  const response = await proxyCommercialBilling(request, "fixture-token", "user-a");
  expect(response.status).toBe(413);
  expect(cancelled).toBe(true);
  expect(fetchMock).not.toHaveBeenCalled();
});

it("times out and cancels a stalled request stream", async () => {
  vi.useFakeTimers();
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888");
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  let cancelled = false;
  const request = postRequest(new ReadableStream<Uint8Array>({ cancel() { cancelled = true; } }));

  const pending = proxyCommercialBilling(request, "fixture-token", "user-a");
  await vi.advanceTimersByTimeAsync(15_000);
  const response = await pending;
  expect(response.status).toBe(504);
  expect(cancelled).toBe(true);
  expect(fetchMock).not.toHaveBeenCalled();
});

it("aborts a stalled upstream request at the fixed deadline", async () => {
  vi.useFakeTimers();
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888");
  const fetchMock = vi.fn((_url: URL, init: RequestInit) => new Promise<Response>((_resolve, reject) => {
    init.signal?.addEventListener("abort", () => reject(new Error("aborted")), { once: true });
  }));
  vi.stubGlobal("fetch", fetchMock);
  const request = new Request("http://localhost/api/workbench/commercial/orders", {
    method: "POST",
    headers: { cookie: "shuomi_effective_organization=org-a", "X-Expected-Organization-ID": "org-a", "X-Expected-User-ID": "user-a", "content-type": "application/json" },
    body: "{}",
  });

  const pending = proxyCommercialBilling(request, "fixture-token", "user-a");
  for (let index = 0; index < 10 && fetchMock.mock.calls.length === 0; index++) await Promise.resolve();
  expect(fetchMock).toHaveBeenCalledOnce();
  await vi.advanceTimersByTimeAsync(15_000);
  const response = await pending;
  expect(response.status).toBe(504);
  expect((fetchMock.mock.calls[0][1] as RequestInit).signal?.aborted).toBe(true);
});

it("rejects a stale user assertion before forwarding billing requests", async () => {
  vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8888");
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const request = postRequest(new ReadableStream<Uint8Array>({ start(controller) { controller.enqueue(new TextEncoder().encode("{}")); controller.close(); } }));

  const response = await proxyCommercialBilling(request, "fixture-token", "user-b");
  expect(response.status).toBe(409);
  expect(await response.json()).toMatchObject({ code: "IDENTITY_CONTEXT_CHANGED" });
  expect(fetchMock).not.toHaveBeenCalled();
});
