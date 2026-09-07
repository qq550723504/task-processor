// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { withProductReviewDeadline } from "./product-title-review-deadline";

afterEach(() => vi.useRealTimers());
it("one total deadline includes auth, prohibits late dispatch, and reports not_sent", async () => {
  vi.useFakeTimers(); let finishAuth!: () => void; const send = vi.fn();
  const response = withProductReviewDeadline(new Request("http://local", { method: "POST" }), async (signal, state) => {
    await new Promise<void>((resolve) => { finishAuth = resolve; });
    signal.throwIfAborted(); state.forwarded = true; send(); return Response.json({});
  });
  await vi.advanceTimersByTimeAsync(15000);
  expect(await (await response).json()).toMatchObject({ code: "DEADLINE_EXCEEDED", outcome: "not_sent" });
  finishAuth(); await vi.advanceTimersByTimeAsync(1); expect(send).not.toHaveBeenCalled();
});
it("cancellation after write dispatch reports unknown without replay", async () => {
  const controller = new AbortController(); const send = vi.fn();
  const response = withProductReviewDeadline(new Request("http://local", { method: "POST", signal: controller.signal }), async (_, state) => {
    state.forwarded = true; send(); await new Promise(() => {}); return Response.json({});
  });
  controller.abort(); const result = await response;
  expect(result.status).toBe(504);
  expect(await result.json()).toMatchObject({ code: "RESULT_UNVERIFIED", outcome: "unknown" });
  expect(result.headers.get("cache-control")).toContain("no-store"); expect(send).toHaveBeenCalledOnce();
});
it("uses only the remaining budget after authentication and bounds a noncooperative operation", async () => {
  vi.useFakeTimers(); const send = vi.fn();
  const response = withProductReviewDeadline(new Request("http://local", { method: "POST" }), async (_, state) => {
    await new Promise((resolve) => setTimeout(resolve, 9000)); state.forwarded = true; send();
    await new Promise(() => {}); return Response.json({});
  });
  await vi.advanceTimersByTimeAsync(15000);
  expect(await (await response).json()).toMatchObject({ code: "RESULT_UNVERIFIED", outcome: "unknown" });
  expect(send).toHaveBeenCalledOnce(); expect(vi.getTimerCount()).toBe(0);
});
it("read cancellation is a read failure; pre-aborted requests do not start auth", async () => {
  const controller = new AbortController(); controller.abort(); const execute = vi.fn();
  const response = await withProductReviewDeadline(new Request("http://local", { signal: controller.signal }), execute);
  expect(await response.json()).toMatchObject({ code: "DEADLINE_EXCEEDED", outcome: "not_sent" });
  expect(execute).not.toHaveBeenCalled();
});
