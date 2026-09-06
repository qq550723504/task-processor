import { z } from "zod";
import { parseWorkbenchErrorEnvelopePayload, type WorkbenchErrorEnvelope } from "./workbench-context";

export const PRODUCT_REVIEW_RESPONSE_BYTES = 128 * 1024;
const bytes = (s: string) => new TextEncoder().encode(s).length;
const unicode = (s: string) => !/[\uD800-\uDFFF]/u.test(s);
const text = z.string().refine(unicode);
export const productTitleIDSchema = z.string().regex(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/)
  .refine((v) => v !== "00000000-0000-0000-0000-000000000000");
const version = z.string().max(19).regex(/^[1-9][0-9]*$/).refine((v) => /^[1-9][0-9]*$/.test(v) && BigInt(v) <= BigInt("9223372036854775807"));
export const productTitleOrganizationSchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
export const productTitleKeySchema = text.min(1).refine((v) => bytes(v) <= 128 && v.trim() === v && !/\p{Cc}/u.test(v));
// A browser Header cannot preserve Unicode or distinguish a comma-joined pair
// from one value. Use a single visible ASCII key (UUID recommended), unchanged.
export const productTitleIdempotencyKeySchema = z.string().min(1).max(128).regex(/^[\x21-\x7e]+$/).refine((v) => !v.includes(","));
export const productTitleCursorSchema = z.string().min(1).max(256).regex(/^[A-Za-z0-9_-]+$/);
const title = text.min(1).refine((v) => bytes(v) <= 4096 && v.trim() === v && !/\p{Cc}/u.test(v));
const time = z.iso.datetime({ precision: undefined }).regex(/T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/).refine((v) => !v.startsWith("0001-"));
const basis = { schema_version: z.literal(1), coverage: z.literal("product-title-proposals-only") };
const item = z.strictObject({ proposal_id: productTitleIDSchema, product_key: productTitleKeySchema, base_version: version, proposal_revision: version, state: z.enum(["pending", "accepted"]) });
const collection = z.strictObject({ ...basis, items: z.array(item).max(100), next_cursor: productTitleCursorSchema.nullable() })
  .refine((v) => new Set(v.items.map((i) => i.proposal_id)).size === v.items.length);
const decision = z.strictObject({ action: z.enum(["accept", "edit", "reject"]), actor: text.min(1), revision: version, before: text, after: text, at: time });
const receipt = z.strictObject({ proposal_id: productTitleIDSchema, revision: version, product_version: version, publication_id: text.min(1), actor: text.min(1), at: time });
const proposal = z.strictObject({
  ...basis, proposal_id: productTitleIDSchema, owner: text.min(1),
  input: z.strictObject({ product_key: productTitleKeySchema, base_version: version }),
  before: text, after: text, original_title: text, policy: z.literal("title-review-v1"),
  state: z.enum(["pending", "accepted", "rejected", "applied"]), revision: version,
  evidence: z.array(z.strictObject({ id: text, reference_type: text, reference_id: text, snapshot_id: text, checksum: text })),
  quality: z.strictObject({ overall: z.number(), evidence_coverage: z.number(), required_field_coverage: z.number() }),
  unresolved: z.array(text), decisions: z.array(decision), apply_receipt: receipt.optional(),
}).refine((v) => v.state === "applied" ? !!v.apply_receipt && v.apply_receipt.proposal_id === v.proposal_id : !v.apply_receipt);

export const productTitleDecisionSchema = z.discriminatedUnion("action", [
  z.strictObject({ action: z.literal("accept"), expected_revision: version }),
  z.strictObject({ action: z.literal("reject"), expected_revision: version }),
  z.strictObject({ action: z.literal("edit"), expected_revision: version, title }),
]);
export const productTitleApplySchema = z.strictObject({ expected_revision: version });
export type ProductTitleProposalListItem = z.infer<typeof item>;
export type ProductTitleProposalList = Omit<z.infer<typeof collection>, "items"> & { items: ProductTitleProposalListItem[] };
export type ProductTitleProposal = z.infer<typeof proposal>;
export type ProductTitleDecisionInput = z.infer<typeof productTitleDecisionSchema>;
export type ProductTitleApplyInput = z.infer<typeof productTitleApplySchema>;
export function parseProductTitleProposalList(value: unknown): ProductTitleProposalList | null { const p = collection.safeParse(value); return p.success ? p.data : null; }
export function parseProductTitleProposal(value: unknown): ProductTitleProposal | null { const p = proposal.safeParse(value); return p.success ? p.data : null; }

const domain = z.strictObject({ error: z.enum(["invalid_request", "permission_denied", "not_found", "stale_product_version", "operation_conflict", "input_too_large", "deadline_exceeded", "unavailable"]) });
const domainStatuses = { invalid_request: 400, permission_denied: 403, not_found: 404, stale_product_version: 409, operation_conflict: 409, input_too_large: 413, deadline_exceeded: 504, unavailable: 503 };
const statuses: Record<string, readonly number[]> = {
  INVALID_REQUEST: [400, 405], AUTHENTICATION_REQUIRED: [401], ORGANIZATION_SELECTION_REQUIRED: [409], ORGANIZATION_CONTEXT_CHANGED: [409],
  ORGANIZATION_ACCESS_DENIED: [403], ORGANIZATION_ACCESS_REVOKED: [403], ORGANIZATION_SUSPENDED: [403], PERMISSION_DENIED: [403],
  DEPENDENCY_UNAVAILABLE: [502, 503], INVALID_UPSTREAM_RESPONSE: [502], DEADLINE_EXCEEDED: [504], RESULT_UNVERIFIED: [502, 504],
};
const communication = z.strictObject({ code: z.string(), message: text.max(2048), requestId: text.max(256), fieldErrors: z.array(z.never()), outcome: z.enum(["not_sent", "unknown"]) });
export type ProductTitleReviewFailure = z.infer<typeof domain> | WorkbenchErrorEnvelope | z.infer<typeof communication>;
export function parseProductTitleReviewFailure(value: unknown, status: number): ProductTitleReviewFailure | null {
  const d = domain.safeParse(value);
  if (d.success && domainStatuses[d.data.error] === status) return d.data;
  const c = communication.safeParse(value);
  if (c.success && statuses[c.data.code]?.includes(status) && (c.data.outcome === "unknown") === (c.data.code === "RESULT_UNVERIFIED")) return c.data;
  const w = parseWorkbenchErrorEnvelopePayload(value);
  return w.success && w.data.code !== "RESULT_UNVERIFIED" && statuses[w.data.code]?.includes(status) ? w.data : null;
}
