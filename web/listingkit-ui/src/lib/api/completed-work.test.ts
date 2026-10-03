import { describe, expect, it } from "vitest";
import { parseCompletedWorkList, projectCompletedWork, type CompletedWorkItem } from "./completed-work";

const id = "12345678-1234-1234-1234-123456789abc";
const storeId = "11111111-1111-4111-8111-111111111111";
const source = { items: [{ record_id: id, product_key: "product", snapshot_version: "9223372036854775807", store_id: storeId, country: "US" as const, language: "en" as const, action: "publish" as const, created_at: "2026-09-06T00:00:00Z" }], next_cursor: null };
describe("completed local preparation projection", () => {
  it("preserves exact source facts with only the approved completion meaning", () => {
    const result = projectCompletedWork(source);
    const item: CompletedWorkItem = result.items[0];
    expect(parseCompletedWorkList(result)).toEqual(result);
    expect(item).toMatchObject({ source_record_id: id, snapshot_version: source.items[0].snapshot_version, store_id: storeId, action: "publish", completion_basis: "local_record_committed", work_scope: "store", result: { href: `/workbench/shein-records/${id}/diagnostic` } });
    expect(result.items[0]).not.toHaveProperty("task_id");
    expect(result.items[0]).not.toHaveProperty("store_name");
    expect(result).not.toHaveProperty("total");
  });
  it("preserves the local draft action without inferring a remote result", () => {
    const draft = projectCompletedWork({ ...source, items: [{ ...source.items[0], action: "save_draft" }] });
    expect(draft.items[0]).toMatchObject({ store_id: storeId, action: "save_draft", work_scope: "store", completion_basis: "local_record_committed" });
    expect(draft.items[0]).not.toHaveProperty("submission_status");
  });
  it("keeps explicit coverage even for an empty page", () => {
    expect(projectCompletedWork({ items: [], next_cursor: null })).toEqual({ projection_version: "2", coverage: "listing-local-preparation-only", items: [], next_cursor: null });
  });
  it.each(["https://evil.invalid", `/workbench/shein-records/22345678-1234-1234-1234-123456789abc/diagnostic`, `/workbench/shein-diagnostics/${id}`])("rejects an unbound result href %s", (href) => {
    const result = projectCompletedWork(source); result.items[0].result.href = href;
    expect(parseCompletedWorkList(result)).toBeNull();
  });
  it("rejects invented lifecycle, store names and completion claims", () => {
    const result = projectCompletedWork(source);
    for (const extra of [{ task_id: id }, { store_name: "一号店" }, { status: "published" }, { completion_basis: "approved" }, { title: "已发布" }, { work_scope: "general" }, { action: "preview" }]) {
      expect(parseCompletedWorkList({ ...result, items: [{ ...result.items[0], ...extra }] })).toBeNull();
    }
  });
});
