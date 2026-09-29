import { z } from "zod";
import {
  readCommercialBilling,
  CommercialBillingReadError,
} from "./commercial-billing";
const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const signed = z.string().refine((v) => {
  if (!/^(0|-?[1-9][0-9]{0,18})$/.test(v)) return false;
  const n = BigInt(v);
  return (
    n >= BigInt("-9223372036854775808") && n <= BigInt("9223372036854775807")
  );
});
const nonnegative = signed.refine((v) => /^(0|[1-9][0-9]*)$/.test(v));
const timestamp = z
  .string()
  .max(40)
  .regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/)
  .refine((v) => Number.isFinite(Date.parse(v)));
const resourceType = z.enum(["store_renewal_period", "ai_point", "data_row"]);
const text = z
  .string()
  .min(1)
  .max(512)
  .refine((v) => !/[\u0000-\u001f\u007f-\u009f]/.test(v));
const event = z
  .object({
    event_id: id,
    operation_id: text,
    resource_type: resourceType,
    quantity: nonnegative,
    available_delta: signed,
    allocated_delta: signed,
    reserved_delta: signed,
    consumed_delta: signed,
    available_after: nonnegative,
    allocated_after: nonnegative,
    reserved_after: nonnegative,
    consumed_after: nonnegative,
    reason: text,
    source_type: text,
    source_identity: text,
    occurred_at: timestamp,
  })
  .strict();
const page = z
  .object({
    organization_id: id,
    items: z.array(event).max(50),
    next_cursor: z.string().min(1).max(1024).nullable(),
  })
  .strict()
  .refine(
    (v) => new Set(v.items.map((e) => e.event_id)).size === v.items.length,
  );
export type ResourceEventPage = z.infer<typeof page>;
export type ResourceEventFilters = {
  resourceType?: z.infer<typeof resourceType>;
  from?: string;
  until?: string;
  cursor?: string;
};
export const RESOURCE_EVENTS_MAX_BYTES = 128 * 1024;
export function parseResourceEvents(v: unknown) {
  const result = page.safeParse(v);
  return result.success ? result.data : null;
}
export function getResourceEvents(
  user: string,
  org: string,
  filters: ResourceEventFilters = {},
  signal?: AbortSignal,
) {
  if (
    (filters.resourceType &&
      !resourceType.safeParse(filters.resourceType).success) ||
    (filters.from && !timestamp.safeParse(filters.from).success) ||
    (filters.until && !timestamp.safeParse(filters.until).success) ||
    (filters.from &&
      filters.until &&
      Date.parse(filters.from) >= Date.parse(filters.until)) ||
    (filters.cursor && filters.cursor.length > 1024)
  )
    throw new CommercialBillingReadError(400, "INVALID_REQUEST");
  const query = new URLSearchParams({ limit: "50" });
  if (filters.resourceType) query.set("resource_type", filters.resourceType);
  if (filters.from) query.set("from", filters.from);
  if (filters.until) query.set("until", filters.until);
  if (filters.cursor) query.set("cursor", filters.cursor);
  return readCommercialBilling(
    `/api/workbench/commercial/resources/events?${query}`,
    user,
    org,
    parseResourceEvents,
    signal,
    RESOURCE_EVENTS_MAX_BYTES,
  );
}
