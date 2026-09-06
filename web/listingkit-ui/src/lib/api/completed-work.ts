import { z } from "zod";
import { sheinRecordListItemSchema, sheinRecordCursorSchema, type SheinRecordList, type SheinRecordListFailure } from "./shein-records";

const diagnosticHref = (id: string) => `/workbench/shein-records/${id}/diagnostic`;
const semantics = {
  source_type: "listing-local-preparation",
  source_kind: "shein-local-record",
  title: "准备商品上架资料",
  summary: "本地资料已创建；诊断和发布是后续独立操作",
  platform: "SHEIN",
  work_scope: "general",
  completion_basis: "local_record_committed",
} as const;
const item = sheinRecordListItemSchema.omit({ record_id: true }).extend({
  source_record_id: sheinRecordListItemSchema.shape.record_id,
  source_type: z.literal(semantics.source_type), source_kind: z.literal(semantics.source_kind),
  title: z.literal(semantics.title), summary: z.literal(semantics.summary),
  platform: z.literal(semantics.platform), work_scope: z.literal(semantics.work_scope),
  completion_basis: z.literal(semantics.completion_basis),
  result: z.strictObject({ kind: z.literal("shein-diagnostic"), href: z.string().max(128) }),
}).refine((value) => value.result.href === diagnosticHref(value.source_record_id));
const list = z.strictObject({
  projection_version: z.literal("1"), coverage: z.literal("listing-local-preparation-only"),
  items: z.array(item).max(100), next_cursor: sheinRecordCursorSchema.nullable(),
}).refine((value) => new Set(value.items.map((entry) => entry.source_record_id)).size === value.items.length);
export type CompletedWorkItem = z.infer<typeof item>;
export type CompletedWorkList = {
  projection_version: "1";
  coverage: "listing-local-preparation-only";
  items: CompletedWorkItem[];
  next_cursor: string | null;
};
export type CompletedWorkFailure = SheinRecordListFailure;

export function parseCompletedWorkList(value: unknown): CompletedWorkList | null {
  const parsed = list.safeParse(value);
  return parsed.success ? parsed.data : null;
}

// Only the explicit, authorized Listing collection is a producer for this projection.
// Its rows represent committed creation operations; this is not Task/Agent state.
export function projectCompletedWork(source: SheinRecordList): CompletedWorkList {
  return {
    projection_version: "1", coverage: "listing-local-preparation-only", next_cursor: source.next_cursor,
    items: source.items.map(({ record_id, ...metadata }) => ({
      ...metadata, ...semantics, source_record_id: record_id,
      result: { kind: "shein-diagnostic", href: diagnosticHref(record_id) },
    })),
  };
}
