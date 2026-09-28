import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";

export const knowledgeId = z.string().uuid().refine(v => v === v.toLowerCase() && v !== "00000000-0000-0000-0000-000000000000");
const version = z.number().int().positive().max(Number.MAX_SAFE_INTEGER);
const timestamp = z.string().datetime({ offset: true });
const actor = z.string().min(1).max(256);
const name = z.string().max(512);
const state = z.enum(["ACTIVE", "DISABLING", "DISABLED"]);
export const revisionSchema = z.object({
 id: knowledgeId, number: version, filename: z.string().max(255), contentType: z.enum(["text/plain","text/markdown","application/pdf","application/vnd.openxmlformats-officedocument.wordprocessingml.document"]),
 sizeBytes: z.number().int().positive().max(10*1024*1024), state: z.enum(["ADMITTED","OBJECT_STORED","PROCESSING","AVAILABLE","PARTIAL","FAILED"]),
 failure: z.string().regex(/^[A-Z_]{1,64}$/).optional(), warning: z.string().regex(/^[A-Z_]{1,64}$/).optional(), createdAt: timestamp, updatedAt: timestamp,
}).strict();
export const baseSchema = z.object({ id: knowledgeId, name, state, version, createdBy: actor, updatedBy: actor, createdAt: timestamp, updatedAt: timestamp }).strict();
export const sourceSchema = z.object({ id: knowledgeId, knowledgeBaseId: knowledgeId, name, state, version, latestRevision: revisionSchema.nullable(), currentReadableRevision: revisionSchema.nullable(), createdBy: actor, updatedBy: actor, createdAt: timestamp, updatedAt: timestamp }).strict().refine(s => !s.currentReadableRevision || ["AVAILABLE","PARTIAL"].includes(s.currentReadableRevision.state));
export const basesSchema = z.object({ items: z.array(baseSchema).max(100), pagination: z.object({page: version, pageSize: version.max(100), total: z.number().int().nonnegative()}).strict() }).strict();
export const sourcesSchema = z.object({items:z.array(sourceSchema).max(100)}).strict();
export const previewSchema = z.object({ revisionId: knowledgeId, text: z.string().min(1).max(2*1024*1024), warning: z.string().regex(/^[A-Z_]{1,64}$/).optional() }).strict();
export const resultSchema = z.object({ knowledgeBase: baseSchema.optional(), source: sourceSchema.optional(), revision: revisionSchema.optional() }).strict().refine(result => !!result.knowledgeBase || !!result.source);
export type KnowledgeBase = z.infer<typeof baseSchema>;
export type KnowledgeSource = z.infer<typeof sourceSchema>;
export type KnowledgeScope = {userId:string;organizationId:string};
export class KnowledgeError extends Error { constructor(public code:string, public status=500) {super(code)} }
export async function knowledgeRequest<T>(scope:KnowledgeScope, path:string, schema:z.ZodType<T>, init:RequestInit = {}):Promise<T> {
 const headers = new Headers(init.headers); headers.set("X-Expected-User-ID",scope.userId); headers.set("X-Expected-Organization-ID",scope.organizationId); headers.set("Accept","application/json");
 let response: Response;
 try { response = await fetch("/api/workbench/"+path,{...init,headers,cache:"no-store",credentials:"same-origin"}); }
 catch { if(init.signal?.aborted) throw new KnowledgeError("REQUEST_CANCELLED"); throw new KnowledgeError(init.method && init.method !== "GET" ? "OUTCOME_UNKNOWN" : "KNOWLEDGE_UNAVAILABLE"); }
 const payload = await readBoundedStrictJSON(response,13*1024*1024,init.signal ?? undefined).catch(() => {throw new KnowledgeError(init.method && init.method !== "GET" ? "OUTCOME_UNKNOWN" : "INVALID_UPSTREAM_RESPONSE",502)});
 if(!response.ok) { const code = z.object({code:z.string().regex(/^[A-Z_]{1,80}$/)}).passthrough().safeParse(payload); throw new KnowledgeError(code.success ? code.data.code : "KNOWLEDGE_UNAVAILABLE",response.status); }
 const result = schema.safeParse(payload); if(!result.success) throw new KnowledgeError(init.method && init.method !== "GET" ? "OUTCOME_UNKNOWN" : "INVALID_UPSTREAM_RESPONSE",502);
 return result.data;
}
