import { z } from "zod";
import { knowledgeRequest, type KnowledgeScope } from "./knowledge";

export const officialArticleID = z.string().regex(/^[a-z][a-z0-9-]{0,63}$/);
export const officialRevision = z.string().regex(/^[1-9][0-9]{0,8}$/);
export const officialSummarySchema = z.strictObject({
  id: officialArticleID, revision: officialRevision, title: z.string().min(1).max(512),
  summary: z.string().min(1).max(2048), category: z.string().min(1).max(256),
  updatedAt: z.iso.date(), maintainedBy: z.string().min(1).max(256), digest: z.string().regex(/^[0-9a-f]{64}$/),
});
export const officialListSchema = z.strictObject({ items: z.array(officialSummarySchema).max(100) })
  .refine(v => new Set(v.items.map(a => a.id)).size === v.items.length);
const sourceURL = z.string().max(2048).refine(value => {
  try { const u = new URL(value); return u.protocol === "https:" && u.host === "github.com" && !u.username && !u.password && !u.search && u.pathname.startsWith("/qq550723504/task-processor/blob/"); } catch { return false; }
});
export const officialArticleSchema = officialSummarySchema.extend({
  body: z.string().min(1).max(64 * 1024).refine(value => new TextEncoder().encode(value).byteLength <= 64 * 1024), sources: z.array(z.strictObject({ title: z.string().min(1).max(512), url: sourceURL })).min(1).max(10),
});
export type OfficialArticle = z.infer<typeof officialArticleSchema>;
export function readOfficialList(scope: KnowledgeScope, signal: AbortSignal) {
  return knowledgeRequest(scope, "official-knowledge", officialListSchema, { signal }, 128 * 1024);
}
export function readOfficialArticle(scope: KnowledgeScope, id: string, revision: string, signal: AbortSignal) {
  return knowledgeRequest(scope, `official-knowledge/${id}/revisions/${revision}`, officialArticleSchema, { signal }, 256 * 1024);
}
