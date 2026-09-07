// @vitest-environment node
import { expect, it, vi } from "vitest";
import { readProductReviewJSON } from "./product-title-review-json";

const response = (raw: string | Uint8Array) => new Response(raw as BodyInit, { headers: { "Content-Type": "application/json", "Content-Length": "1" } });
it("preserves exact decimal strings and valid surrogate pairs", async () => {
  expect(await readProductReviewJSON(response('{"v":"9007199254740993","t":"\\uD83D\\uDE00"}'), 1024)).toEqual({ v: "9007199254740993", t: "😀" });
});
it.each(['{"t":"\\uD800"}', '{"t":"\\uDC00"}', '{"t":["\\uD800x"]}', '{"\\uD800":true}', '{"t":1,"t":2}', '{"t":1,}', '{/*comment*/"t":1}'])("rejects invalid Unicode/strict JSON: %s", async (raw) => {
  await expect(readProductReviewJSON(response(raw), 1024)).rejects.toThrow();
});
it("bounds actual bytes, rejects malformed UTF-8 and cancels stalled reads", async () => {
  await expect(readProductReviewJSON(response('"' + "中".repeat(400) + '"'), 1024)).rejects.toThrow();
  await expect(readProductReviewJSON(response(new Uint8Array([34, 0xff, 34])), 1024)).rejects.toThrow();
  const controller = new AbortController(); const cancel = vi.fn();
  const pending = readProductReviewJSON(new Response(new ReadableStream({ cancel }), { headers: { "Content-Type": "application/json" } }), 1024, controller.signal);
  controller.abort();
  await expect(pending).rejects.toThrow(); expect(cancel).toHaveBeenCalledOnce();
});
