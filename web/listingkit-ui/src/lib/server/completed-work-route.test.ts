// @vitest-environment node
import { expect, it, vi } from "vitest";
vi.mock("@/auth", () => ({ serverAuth: vi.fn() }));
import { projectCompletedWorkResponse } from "./completed-work-route";
it("projects the actual Go collection store and action into v2 without remote publication claims", async () => {
  const record_id = "12345678-1234-4234-8234-123456789abc";
  const store_id = "11111111-1111-4111-8111-111111111111";
  const response = await projectCompletedWorkResponse(Response.json({ items: [{ record_id, product_key: "product", snapshot_version: "1", store_id, country: "US", language: "en", action: "publish", created_at: "2026-09-30T00:00:00Z" }], next_cursor: null }), new AbortController().signal);
  expect(response.status).toBe(200);
  const result = await response.json();
  expect(result).toMatchObject({ projection_version: "2", items: [{ source_record_id: record_id, store_id, action: "publish", work_scope: "store", completion_basis: "local_record_committed" }] });
  expect(JSON.stringify(result)).not.toContain("published");
});
it("limits actual mapped JSON bytes, including escaped metadata", async () => {
  const items = Array.from({ length: 100 }, (_, i) => ({ record_id: `12345678-1234-1234-1234-${String(i).padStart(12, "0")}`, product_key: "\u0001".repeat(128), snapshot_version: "9223372036854775807", store_id: "11111111-1111-4111-8111-111111111111", country: "US", language: "en", action: "publish", created_at: "2026-09-06T00:00:00.123456789Z" }));
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
