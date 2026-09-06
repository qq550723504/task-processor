import type { SheinRecordList } from "@/lib/api/shein-records";

// Synthetic #327 contract data for UI tests only; never imported by runtime pages.
export function recordListFixture(next_cursor: string | null = null, suffix = "1") {
  return {
    items: [{ record_id: `c78285de-a1a8-4cc5-8aae-${suffix.padStart(12, "0")}`, product_key: `source-product-${suffix}`, snapshot_version: "9007199254740993", country: "US", language: "en", created_at: "2026-09-06T06:00:00Z" }],
    next_cursor,
  } satisfies SheinRecordList;
}
