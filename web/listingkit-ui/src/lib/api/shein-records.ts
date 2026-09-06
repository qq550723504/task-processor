import { z } from "zod";
import { parseWorkbenchErrorEnvelopePayload, type WorkbenchErrorEnvelope } from "./workbench-context";

const boundedText = (maxBytes: number) => z.string().min(1).refine((value) => value.trim() === value && !/[\0\r\n\t]/.test(value) && new TextEncoder().encode(value).length <= maxBytes);
const recordId = z.string().regex(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
const positiveInt64 = z.string().refine((value) => /^[1-9][0-9]*$/.test(value) && BigInt(value) <= BigInt("9223372036854775807"));
const timestamp = z.iso.datetime().regex(/T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/);
export const sheinRecordListItemSchema = z.strictObject({
  record_id: recordId,
  product_key: boundedText(128),
  snapshot_version: positiveInt64,
  country: z.literal("US"),
  language: z.literal("en"),
  created_at: timestamp,
});
const list = z.strictObject({
  items: z.array(sheinRecordListItemSchema).max(100),
  next_cursor: boundedText(512).nullable(),
}).refine((value) => new Set(value.items.map((entry) => entry.record_id)).size === value.items.length);

export type SheinRecordListItem = z.infer<typeof sheinRecordListItemSchema>;
export type SheinRecordList = {
  items: SheinRecordListItem[];
  next_cursor: string | null;
};

const failure = z.strictObject({ error: z.enum(["invalid_request", "permission_denied", "unavailable", "deadline_exceeded"]) });
export type SheinRecordListFailure = z.infer<typeof failure> | WorkbenchErrorEnvelope;
const failureStatuses: Record<z.infer<typeof failure>["error"], number> = { invalid_request: 400, permission_denied: 403, unavailable: 503, deadline_exceeded: 504 };
const workbenchStatuses: Record<string, readonly number[]> = {
  INVALID_REQUEST: [400, 405], AUTHENTICATION_REQUIRED: [401], ORGANIZATION_SELECTION_REQUIRED: [409], ORGANIZATION_CONTEXT_CHANGED: [409],
  ORGANIZATION_ACCESS_DENIED: [403], ORGANIZATION_ACCESS_REVOKED: [403], ORGANIZATION_SUSPENDED: [403], PERMISSION_DENIED: [403],
  DEPENDENCY_UNAVAILABLE: [502, 503], INVALID_UPSTREAM_RESPONSE: [502], DEADLINE_EXCEEDED: [504],
};

export function parseSheinRecordList(value: unknown): SheinRecordList | null {
  const parsed = list.safeParse(value);
  return parsed.success ? parsed.data : null;
}

export function parseSheinRecordListFailure(value: unknown, status: number): SheinRecordListFailure | null {
  const parsed = failure.safeParse(value);
  if (parsed.success && failureStatuses[parsed.data.error] === status) return parsed.data;
  const workbench = parseWorkbenchErrorEnvelopePayload(value);
  return workbench.success && workbenchStatuses[workbench.data.code]?.includes(status) ? workbench.data : null;
}

export const sheinRecordOrganizationSchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
export const sheinRecordLimitSchema = z.number().int().min(1).max(100);
export const sheinRecordCursorSchema = boundedText(512);
