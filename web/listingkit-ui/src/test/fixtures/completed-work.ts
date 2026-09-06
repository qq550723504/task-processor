import type { CompletedWorkList } from "@/lib/api/completed-work";

// Approved #340 C340-H1 synthetic component DTO only, never a runtime fallback.
export function completedWorkFixture(next_cursor: string | null = null, suffix = "1"): CompletedWorkList {
  const id = `12345678-1234-4234-8234-${suffix.padStart(12, "0")}`;
  return { projection_version: "1", coverage: "listing-local-preparation-only", next_cursor, items: [{
    source_type: "listing-local-preparation", source_kind: "shein-local-record",
    title: "准备商品上架资料", summary: "本地资料已创建；诊断和发布是后续独立操作",
    platform: "SHEIN", work_scope: "general", completion_basis: "local_record_committed",
    source_record_id: id, product_key: `synthetic-product-${suffix}`, snapshot_version: "9007199254740993",
    country: "US", language: "en", created_at: "2026-09-06T00:00:00Z",
    result: { kind: "shein-diagnostic", href: `/workbench/shein-records/${id}/diagnostic` },
  }] };
}
