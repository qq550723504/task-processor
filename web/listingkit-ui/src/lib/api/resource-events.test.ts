import { afterEach, expect, it, vi } from "vitest";
import { getResourceEvents, parseResourceEvents } from "./resource-events";
afterEach(() => vi.unstubAllGlobals());
const row = {
  event_id: "event-1",
  operation_id: "operation-1",
  resource_type: "ai_point",
  quantity: "7",
  available_delta: "23",
  allocated_delta: "0",
  reserved_delta: "-30",
  consumed_delta: "7",
  available_after: "93",
  allocated_after: "0",
  reserved_after: "0",
  consumed_after: "7",
  reason: "consumed",
  source_type: "model_invocation_v1",
  source_identity: "invocation-1",
  occurred_at: "2026-09-29T00:00:00Z",
};
it("keeps native resource units and exact signed quantities without accepting invalid integers", () => {
  expect(
    parseResourceEvents({
      organization_id: "org-B",
      items: [row],
      next_cursor: null,
    }),
  ).not.toBeNull();
  for (const changed of [
    { ...row, quantity: "invalid" },
    { ...row, available_after: "-1" },
    { ...row, consumed_delta: "9223372036854775808" },
  ]) {
    expect(() =>
      parseResourceEvents({
        organization_id: "org-B",
        items: [changed],
        next_cursor: null,
      }),
    ).not.toThrow();
    expect(
      parseResourceEvents({
        organization_id: "org-B",
        items: [changed],
        next_cursor: null,
      }),
    ).toBeNull();
  }
});
it("binds bounded filters and event pages to the current enterprise", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValue(
      Response.json({
        organization_id: "org-A",
        items: [row],
        next_cursor: null,
      }),
    );
  vi.stubGlobal("fetch", fetcher);
  await expect(
    getResourceEvents("user-B", "org-B", {
      resourceType: "ai_point",
      cursor: "original",
    }),
  ).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
  expect(fetcher.mock.calls[0][0]).toBe(
    "/api/workbench/commercial/resources/events?limit=50&resource_type=ai_point&cursor=original",
  );
});
