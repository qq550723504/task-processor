import { describe, expect, it } from "vitest";
import { parseCompletedWorkList, projectCompletedWork } from "./completed-work";

const id = "12345678-1234-1234-1234-123456789abc";
const source = { items: [{ record_id: id, product_key: "product", snapshot_version: "9223372036854775807", country: "US" as const, language: "en" as const, created_at: "2026-09-06T00:00:00Z" }], next_cursor: null };
describe("completed local preparation projection", () => {
  it("preserves exact source facts with only the approved completion meaning", () => {
    const result = projectCompletedWork(source);
    expect(parseCompletedWorkList(result)).toEqual(result);
    expect(result.items[0]).toMatchObject({ source_record_id: id, snapshot_version: source.items[0].snapshot_version, completion_basis: "local_record_committed", work_scope: "general", result: { href: `/workbench/shein-records/${id}/diagnostic` } });
    expect(result.items[0]).not.toHaveProperty("task_id");
    expect(result.items[0]).not.toHaveProperty("store_id");
    expect(result).not.toHaveProperty("total");
  });
  it("keeps explicit coverage even for an empty page", () => {
    expect(projectCompletedWork({ items: [], next_cursor: null })).toEqual({ projection_version: "1", coverage: "listing-local-preparation-only", items: [], next_cursor: null });
  });
  it.each(["https://evil.invalid", `/workbench/shein-records/22345678-1234-1234-1234-123456789abc/diagnostic`, `/workbench/shein-diagnostics/${id}`])("rejects an unbound result href %s", (href) => {
    const result = projectCompletedWork(source); result.items[0].result.href = href;
    expect(parseCompletedWorkList(result)).toBeNull();
  });
  it("rejects invented lifecycle, store and completion claims", () => {
    const result = projectCompletedWork(source);
    for (const extra of [{ task_id: id }, { store_id: "shop" }, { status: "published" }, { completion_basis: "approved" }, { title: "已发布" }]) {
      expect(parseCompletedWorkList({ ...result, items: [{ ...result.items[0], ...extra }] })).toBeNull();
    }
  });
});
