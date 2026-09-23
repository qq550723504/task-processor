import { afterEach, expect, it, vi } from "vitest";
import { proxyCommercialBilling } from "./commercial-billing-proxy";

afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.unstubAllEnvs(); });

function postRequest(body: ReadableStream<Uint8Array>): Request {
  return new Request("http://localhost/api/workbench/commercial/orders", {
    method: "POST",
    headers: {
      cookie: "shuomi_effective_organization=org-a",
      "X-Expected-Organization-ID": "org-a",
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

  const response = await proxyCommercialBilling(request, "fixture-token");
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

  const pending = proxyCommercialBilling(request, "fixture-token");
  await vi.advanceTimersByTimeAsync(15_000);
  const response = await pending;
  expect(response.status).toBe(408);
  expect(cancelled).toBe(true);
  expect(fetchMock).not.toHaveBeenCalled();
});
