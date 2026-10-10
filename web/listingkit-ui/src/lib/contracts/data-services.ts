import { z } from "zod";
import { collectionItemSchema } from "@/lib/contracts/product-collection";
export const DATA_MAX_BYTES = 2 * 1024 * 1024;
export const CUSTOM_INPUT_BYTE_LIMITS = { name: 200, purpose: 1000, timeRange: 1000, notes: 4000 } as const;
const id = z.string().uuid();
const count = z.number().int().nonnegative().max(1e11);
const text = z.string().max(65536);
const bytes = (max: number, required = false) => z.string().refine(v => new TextEncoder().encode(v).byteLength <= max && (!required || v.trim().length > 0) && !v.includes("\0"));
const date = z.string().datetime({ offset: true });
const siteSchema = z.object({ code: z.enum(["us", "uk", "de", "fr", "it", "es", "ca", "jp", "au", "mx", "br", "in", "ae", "sa"]), domain: z.string().max(80), name: z.string().max(40) }).strict();
const dataField = z.enum(["asin", "site", "title", "description", "brand", "images", "price", "currency", "availability", "attributes", "variants", "rating", "reviewCount", "sourceUrl", "capturedAt"]);
export const querySchema = z.object({ site: siteSchema.shape.code, mode: z.enum(["asin", "keyword", "category"]), keyword: z.string().max(200).optional(), categoryNode: z.string().regex(/^\d{1,20}$/).optional(), asins: z.array(z.string().max(2048)).max(200).optional(), limit: z.number().int().min(1).max(200), fields: z.array(dataField).max(32).optional() }).strict();
export const keyLimitsSchema = z.object({ name: z.string().min(1).max(80), expiresAt: date, dailyRows: count.refine(n => n > 0 && n <= 1e9), monthlyCostFen: count.refine(n => n > 0), permissions: z.array(z.enum(["amazon.acquire", "amazon.result.read"])).min(1).max(2), cidrs: z.array(z.string().max(80)).max(20).optional() }).strict();
export const keySchema = z.object({ id, limits: keyLimitsSchema, suffix: z.string().length(4), state: z.enum(["ACTIVE", "DISABLED", "REVOKED"]), revision: count, createdAt: date }).strict();
export const keyCreatedSchema = z.object({ key: keySchema, secret: z.string().regex(/^[A-Za-z0-9_-]{43}$/).optional(), replayed: z.boolean() }).strict();
export const keyHistorySchema = z.object({ items: z.array(keySchema).max(100), nextCursor: id.optional() }).strict();
export const jobSchema = z.object({ id, credentialId: id.optional(), commandKey: id, query: querySchema, state: z.enum(["ADMITTED", "RUNNING", "SUCCEEDED", "PARTIAL", "FAILED", "CANCELED"]), reason: z.string().max(64).optional(), discovered: z.boolean(), canceled: z.boolean(), saved: count, failed: count, pending: count, confirmedFen: count, pendingFen: count, batchId: id.optional(), createdAt: date, deadline: date }).strict();
export const resultPageSchema = z.object({ items: z.array(z.object({ id, state: z.enum(["PREPARED", "FETCHING", "PREPARED_EVIDENCE", "SAVED", "FAILED"]), reason: z.string().max(64).optional(), source: collectionItemSchema.shape.source.optional(), data: z.partialRecord(dataField, z.unknown()).optional(), missing: z.array(dataField).max(32).optional() }).strict()).max(100), nextCursor: id.optional() }).strict();
export const optionsSchema = z.object({ sites: z.array(siteSchema).max(14), customSites: z.array(siteSchema).max(14), fields: z.array(dataField).max(32), priceFen: z.literal(5), maximumRows: z.literal(200), formats: z.array(z.enum(["csv", "json", "xlsx"])).max(3), acquisitionReady: z.boolean(), unavailableReason: z.string().max(160).optional() }).strict();
const usageSchema = z.object({ dayRows: count, monthConfirmedFen: count, monthPendingFen: count, finishedJobs: count, succeededJobs: count, successRate: z.number().min(0).max(1).nullable(), window: z.string().max(120) }).strict();
const keyQuotaSchema = z.object({ keyId: id, dayConsumedRows: count, dayReservedRows: count, monthConsumedFen: count, monthReservedFen: count }).strict();
export const overviewSchema = z.object({ usage: usageSchema, keys: z.array(keySchema).max(20), jobs: z.array(jobSchema).max(10), options: optionsSchema, keyQuotas: z.array(keyQuotaSchema).max(20) }).strict();
const customInputSchema = z.object({ name: bytes(CUSTOM_INPUT_BYTE_LIMITS.name, true), query: querySchema, purpose: bytes(CUSTOM_INPUT_BYTE_LIMITS.purpose, true), timeRange: bytes(CUSTOM_INPUT_BYTE_LIMITS.timeRange).optional(), format: z.enum(["csv", "json", "xlsx"]), notes: bytes(CUSTOM_INPUT_BYTE_LIMITS.notes).optional() }).strict();
export const customSpecSchema = z.object({ description: bytes(4000, true), quoteNote: bytes(2000, true), confirmationNote: bytes(2000, true), format: z.enum(["csv", "json", "xlsx"]), maximumRows: z.number().int().min(1).max(200) }).strict();
const customState = z.enum(["SUBMITTED", "EVALUATING", "SPEC_CONFIRMED", "PREPARING", "DELIVERED", "CLOSED"]);
export const customSchema = z.object({ id, input: customInputSchema, state: customState, revision: count, spec: customSpecSchema.optional(), specRevision: count, batchId: id.optional(), deliveredRows: count, createdAt: date, events: z.array(z.object({ revision: count, state: customState, operatorId: z.string().max(128), note: text, at: date }).strict()).max(100), nextEventBefore: count.optional() }).strict();
export const customSummarySchema = z.object({ id, name: bytes(200, true), site: siteSchema.shape.code, mode: querySchema.shape.mode, state: customState, createdAt: date }).strict();
const applicantSchema = z.object({ organizationId: z.string().max(128), actorId: z.string().max(128) }).strict();
export const adminCustomSchema = customSchema.extend({ applicant: applicantSchema });
export const adminSummarySchema = customSummarySchema.extend({ applicant: applicantSchema });
export const adminPageSchema = z.object({ items: z.array(adminSummarySchema).max(100), nextCursor: id.optional() }).strict();
const jobCreateSchema = z.object({ query: querySchema, maximumRows: z.number().int().min(1).max(200), maximumCostFen: count }).strict().refine(v => v.maximumRows === v.query.limit && v.maximumCostFen === 5 * v.maximumRows);
const keyChangeSchema = z.object({ expectedRevision: count.refine(n => n > 0), patch: z.object({ state: z.enum(["ACTIVE", "DISABLED", "REVOKED"]), limits: keyLimitsSchema.optional() }).strict() }).strict();
const customChangeSchema = z.object({ expectedRevision: count.refine(n => n > 0), patch: z.object({ state: customState, note: bytes(2000, true), spec: customSpecSchema.optional() }).strict() }).strict();
export type DataOptions = z.infer<typeof optionsSchema>;
export type DataKey = z.infer<typeof keySchema>;
export type DataJob = z.infer<typeof jobSchema>;
export type DataQuery = z.infer<typeof querySchema>;
export type CustomRequest = z.infer<typeof customSchema>;
export type Overview = z.infer<typeof overviewSchema>;
export function dataRoute(method: string, path: string[], specialist = false) {
    const p = path.join("/");
    const uid = "[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}";
    const match = (pattern: string) => new RegExp(`^${pattern}$`, "i").test(p);
    if (specialist) {
        if (method === "GET" && p === "")
            return { action: "admin-list", response: adminPageSchema };
        if (method === "GET" && match(`by-command/${uid}`))
            return { action: "admin-command", response: adminCustomSchema };
        if (method === "GET" && match(uid))
            return { action: "admin-read", response: adminCustomSchema };
        if (method === "POST" && match(`${uid}/changes`))
            return { action: "admin-change", request: customChangeSchema, response: adminCustomSchema };
        if (method === "POST" && match(`${uid}/delivery`))
            return { action: "admin-deliver", binary: true, response: adminCustomSchema };
        return null;
    }
    if (method === "GET") {
        if (p === "options")
            return { action: "options", response: optionsSchema };
        if (p === "overview")
            return { action: "overview", response: overviewSchema };
        if (p === "keys")
            return { action: "keys", response: z.array(keySchema).max(20) };
        if (p === "keys/history")
            return { action: "key-history", response: keyHistorySchema };
        if (match(`keys/by-command/${uid}`))
            return { action: "key-command", response: keySchema };
        if (p === "amazon/jobs")
            return { action: "jobs", response: z.array(jobSchema).max(100) };
        if (match(`amazon/jobs/(by-command/)?${uid}`))
            return { action: "job-read", response: jobSchema };
        if (match(`amazon/jobs/${uid}/results`))
            return { action: "job-results", response: resultPageSchema };
        if (p === "custom")
            return { action: "custom-list", response: z.array(customSummarySchema).max(100) };
        if (match(`custom/(by-command/)?${uid}`))
            return { action: "custom-read", response: customSchema };
    }
    if (method === "POST") {
        if (p === "keys")
            return { action: "key-create", request: keyLimitsSchema, response: keyCreatedSchema };
        if (match(`keys/${uid}/changes`))
            return { action: "key-change", request: keyChangeSchema, response: keySchema };
        if (p === "amazon/jobs")
            return { action: "job-create", request: jobCreateSchema, response: jobSchema };
        if (match(`amazon/jobs/${uid}/cancel`))
            return { action: "job-cancel", request: z.object({}).strict(), response: jobSchema };
        if (p === "custom")
            return { action: "custom-submit", request: customInputSchema, response: customSchema };
    }
    ;
    return null;
}
