// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { fetchSheinRecords, SheinRecordListError } from "./shein-records-client";
import type { SheinRecordListItem } from "./shein-records";

const item: SheinRecordListItem = { record_id: "12345678-1234-4234-8234-123456789abc", product_key: "source-product", snapshot_version: "1", country: "US", language: "en", created_at: "2026-09-06T01:02:03Z" };
const sheinRecordListFixture = () => ({ items: [item], next_cursor: "opaque-cursor_1" });

afterEach(() => vi.unstubAllGlobals());

it("uses same-origin GET with an expected organization assertion", async () => {
  const fetcher = vi.fn().mockResolvedValue(Response.json(sheinRecordListFixture()));
  vi.stubGlobal("fetch", fetcher);
  const controller = new AbortController();
  await expect(fetchSheinRecords({ organizationId: "200", limit: 20, cursor: "opaque-cursor_1", signal: controller.signal })).resolves.toEqual(sheinRecordListFixture());
  expect(fetcher).toHaveBeenCalledWith("/api/listing/shein-records?limit=20&cursor=opaque-cursor_1", {
    method: "GET", credentials: "same-origin", cache: "no-store", redirect: "error",
    headers: { Accept: "application/json", "X-Expected-Organization-ID": "200" }, signal: controller.signal,
  });
});

it("rejects invalid client input before fetch", async () => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  for (const input of [{ organizationId: "" }, { organizationId: "200", limit: 0 }, { organizationId: "200", limit: 101 }, { organizationId: "200", cursor: "x".repeat(513) }]) {
    await expect(fetchSheinRecords(input)).rejects.toBeInstanceOf(SheinRecordListError);
  }
  expect(fetcher).not.toHaveBeenCalled();
});

it("preserves typed failures and rejects invalid upstream shapes", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ error: "permission_denied" }, { status: 403 })));
  await expect(fetchSheinRecords({ organizationId: "200" })).rejects.toMatchObject({ status: 403, code: "permission_denied" });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ items: null, next_cursor: null })));
  await expect(fetchSheinRecords({ organizationId: "200" })).rejects.toMatchObject({ status: 502, code: "INVALID_UPSTREAM_RESPONSE" });
});
