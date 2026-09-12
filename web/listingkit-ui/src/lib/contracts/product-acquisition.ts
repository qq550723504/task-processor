import { z } from "zod";

export const ACQUISITION_BODY_MAX_BYTES = 8192;
export const ACQUISITION_RESPONSE_MAX_BYTES = 2 * 1024 * 1024;
export const ACQUISITION_BASE = "/api/workbench/sourcing/1688/acquisitions";

export function isAcquisitionUUID(value: string) {
  return value !== "00000000-0000-0000-0000-000000000000" && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(value);
}

export function canonical1688Source(value: string): string | null {
  if (new TextEncoder().encode(value).length > 2048) return null;
  value = value.trim();
  if (/^[1-9][0-9]{0,19}$/.test(value)) return `https://detail.1688.com/offer/${value}.html`;
  const match = /^([A-Za-z]+):\/\/([^/?#]+)(\/[^?#]*)(?:\?([^#]*))?(?:#(.*))?$/.exec(value);
  if (!match) return null;
  const scheme = match[1]!.toLowerCase();
  if (scheme !== "http" && scheme !== "https") return null;
  const host = match[2]!.toLowerCase();
  if (host !== "detail.1688.com" && host !== `detail.1688.com:${scheme === "https" ? "443" : "80"}`) return null;
  const offer = /^\/offer\/([1-9][0-9]{0,19})\.html$/.exec(match[3]!);
  if (!offer) return null;
  try { decodeURIComponent(match[4] ?? ""); decodeURIComponent(match[5] ?? ""); } catch { return null; }
  return `https://detail.1688.com/offer/${offer[1]}.html`;
}

export const acquisitionRequestSchema = z.object({ source: z.string().refine((v) => canonical1688Source(v) !== null) }).strict();
const bounded = z.string().refine((v) => new TextEncoder().encode(v).length <= 8192);
const uuid = z.string().refine(isAcquisitionUUID);
const version = z.string().regex(/^[1-9][0-9]{0,18}$/).refine((v) => BigInt(v) <= BigInt("9223372036854775807"));
export const acquisitionResultSchema = z.object({
  schemaVersion: z.literal(1), operationId: uuid,
  outcome: z.enum(["acquiring", "prepared", "outcome_unknown", "published", "failed"]),
  replayed: z.boolean(), productKey: z.string().regex(/^crawler:1688:[1-9][0-9]{0,19}$/).optional(),
  publicationId: z.string().max(128).optional(), catalogVersion: version.optional(),
  warnings: z.array(z.object({ code: bounded, field: bounded }).strict()).max(256),
  missingFacts: z.array(z.object({ field: bounded, reason: bounded }).strict()).max(256),
}).strict().superRefine((value, ctx) => {
  const published = value.outcome === "published";
  if (published ? !value.productKey || !value.catalogVersion || value.publicationId !== `source-run:acquisition:${value.operationId}` : value.productKey !== undefined || value.publicationId !== undefined || value.catalogVersion !== undefined) {
    ctx.addIssue({ code: "custom", message: "Publication binding is inconsistent" });
  }
});
export type AcquisitionResult = z.infer<typeof acquisitionResultSchema>;

export const acquisitionErrorStatuses: Readonly<Record<string, number>> = {
  INVALID_ACQUISITION: 400, FORBIDDEN: 403, SOURCE_TOO_LARGE: 413,
  ACQUISITION_CAPACITY: 429, IDEMPOTENCY_CONFLICT: 409, ACQUISITION_NOT_FOUND: 404,
  OUTCOME_UNKNOWN: 503, ACQUISITION_UNAVAILABLE: 503, SOURCE_UNAVAILABLE: 502,
  INVALID_REQUEST: 400, AUTHENTICATION_REQUIRED: 401, ORGANIZATION_SELECTION_REQUIRED: 409,
  ORGANIZATION_ACCESS_DENIED: 403, ORGANIZATION_ACCESS_REVOKED: 403,
  ORGANIZATION_SUSPENDED: 403, PERMISSION_DENIED: 403, DEPENDENCY_UNAVAILABLE: 503,
  ORGANIZATION_CONTEXT_CHANGED: 409, IDENTITY_CONTEXT_CHANGED: 409, DEADLINE_EXCEEDED: 504,
};
