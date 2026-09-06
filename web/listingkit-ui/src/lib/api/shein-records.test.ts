import { describe, expect, it } from "vitest";
import { parseSheinRecordList, parseSheinRecordListFailure } from "./shein-records";

export const sheinRecordListFixture = () => ({
  items: [{
    record_id: "12345678-1234-4234-8234-123456789abc",
    product_key: "source-product",
    snapshot_version: "9007199254740993",
    country: "US",
    language: "en",
    created_at: "2026-09-06T01:02:03.123456Z",
  }],
  next_cursor: "opaque-cursor_1",
});

describe("SHEIN record list wire contract", () => {
  it("preserves exact bigint strings, timestamps and opaque cursor", () => {
    expect(parseSheinRecordList(sheinRecordListFixture())).toEqual(sheinRecordListFixture());
    expect(parseSheinRecordList({ items: [], next_cursor: null })).toEqual({ items: [], next_cursor: null });
  });

  it.each([
    { items: null },
    { items: [], next_cursor: undefined },
    { items: [], next_cursor: "" },
    { items: [], next_cursor: null, total: 1 },
    { items: [{ ...sheinRecordListFixture().items[0], snapshot_version: "9007199254740993" }, { ...sheinRecordListFixture().items[0] }], next_cursor: null },
    { items: [{ ...sheinRecordListFixture().items[0], snapshot_version: "01" }], next_cursor: null },
    { items: [{ ...sheinRecordListFixture().items[0], snapshot_version: "9223372036854775808" }], next_cursor: null },
    { items: [{ ...sheinRecordListFixture().items[0], created_at: "2026-02-30T01:00:00Z" }], next_cursor: null },
    { items: [{ ...sheinRecordListFixture().items[0], organization_id: "200" }], next_cursor: null },
  ])("rejects malformed or expanded response %#", (payload) => {
    expect(parseSheinRecordList(payload)).toBeNull();
  });

  it("preserves only status-compatible Go and Workbench errors", () => {
    expect(parseSheinRecordListFailure({ error: "invalid_request" }, 400)).toEqual({ error: "invalid_request" });
    expect(parseSheinRecordListFailure({ error: "unavailable" }, 503)).toEqual({ error: "unavailable" });
    const workbench = { code: "ORGANIZATION_ACCESS_REVOKED", message: "Denied", requestId: "r", fieldErrors: [] };
    expect(parseSheinRecordListFailure(workbench, 403)).toEqual(workbench);
    expect(parseSheinRecordListFailure({ error: "invalid_request", sql: "private" }, 400)).toBeNull();
    expect(parseSheinRecordListFailure({ error: "invalid_request" }, 200)).toBeNull();
  });
});
