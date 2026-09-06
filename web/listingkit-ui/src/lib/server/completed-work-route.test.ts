// @vitest-environment node
import { expect, it, vi } from "vitest";
vi.mock("@/auth", () => ({ serverAuth: vi.fn() }));
import { projectCompletedWorkResponse } from "./completed-work-route";
it("limits actual mapped JSON bytes, including escaped metadata", async () => {
  const items = Array.from({ length: 100 }, (_, i) => ({ record_id: `12345678-1234-1234-1234-${String(i).padStart(12, "0")}`, product_key: "\u0001".repeat(128), snapshot_version: "9223372036854775807", country: "US", language: "en", created_at: "2026-09-06T00:00:00.123456789Z" }));
  const raw = JSON.stringify({ items, next_cursor: null });
  expect(new TextEncoder().encode(raw).length).toBeLessThan(128 * 1024);
  const response = await projectCompletedWorkResponse(Response.json(JSON.parse(raw)), new AbortController().signal);
  expect(response.status).toBe(502);
  expect((await response.json()).code).toBe("INVALID_UPSTREAM_RESPONSE");
});
it("checks cancellation while reading the successful collection", async () => {
  const controller = new AbortController();
  const pending = projectCompletedWorkResponse(new Response(new ReadableStream({ start() {} }), { headers: { "Content-Type": "application/json" } }), controller.signal);
  controller.abort(); expect((await pending).status).toBe(504);
});
it("returns non-success response without consuming or replacing its headers", async () => {
  const source = new Response("failure", { status: 403, headers: { "Set-Cookie": "org=; Max-Age=0", "Cache-Control": "no-store" } });
  expect(await projectCompletedWorkResponse(source, new AbortController().signal)).toBe(source);
});
