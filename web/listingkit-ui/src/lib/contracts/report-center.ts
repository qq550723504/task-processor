import { z } from "zod";
export const reportID = z.string().regex(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/).refine(v => v !== "00000000-0000-0000-0000-000000000000");
const bytes = (v: string) => new TextEncoder().encode(v).length;
const text = (n: number) => z.string().min(1).refine(v => bytes(v) <= n && !/\p{Cc}/u.test(v));
const timestamp = z.iso.datetime().refine(v => !v.startsWith("0001-"));
export const reportKind = z.enum(["TITLE_REVIEW", "SHEIN_RECORD"]);
export const sourceRef = z.strictObject({ kind: reportKind, id: reportID, version: z.string().max(96) }).refine(v =>
  v.kind === "SHEIN_RECORD" ? /^sha256:[0-9a-f]{64}$/.test(v.version) : /^[1-9][0-9]{0,18}:(pending|accepted|rejected|applied)$/.test(v.version) && BigInt(v.version.split(":")[0]) <= BigInt("9223372036854775807"));
const metadata = { ref: sourceRef, title: text(256), productKey: text(128), storeId: z.union([z.literal(""), reportID]), sourceAt: timestamp.optional() };
export const reportSourceSchema = z.strictObject(metadata);
const summary = { ...metadata, id: reportID, capturedAt: timestamp, favorite: z.boolean() };
export const reportSummarySchema = z.strictObject(summary);
export const reportDocumentSchema = z.strictObject({ schemaVersion: z.literal(1), sections: z.array(z.strictObject({ title: text(256), fields: z.array(z.strictObject({ label: text(256), value: z.string().refine(v => bytes(v) <= 16000 && !/\p{Cc}/u.test(v.replace(/[\r\n\t]/g, ""))) })).min(1).max(256) })).min(1).max(16) }).refine(v => v.sections.reduce((n, s) => n + s.fields.length, 0) <= 256 && bytes(JSON.stringify(v)) <= 131072);
export const reportSchema = z.strictObject({ ...summary, content: reportDocumentSchema, digest: z.string().regex(/^[0-9a-f]{64}$/) });
export const reportsPageSchema = z.strictObject({ items: z.array(reportSummarySchema).max(50), nextCursor: z.string().max(512).regex(/^[A-Za-z0-9_-]*$/) }).refine(v => new Set(v.items.map(i => i.id)).size === v.items.length);
export const reportsSummarySchema = z.strictObject({ saved: z.number().int().nonnegative(), recent: z.number().int().nonnegative(), favorites: z.number().int().nonnegative(), stores: z.number().int().nonnegative() });
export const reportResultSchema = z.strictObject({ commandId: reportID, report: reportSchema, replayed: z.boolean() });
export const reportSaveBody = z.strictObject({ source: sourceRef });
export const reportFavoriteBody = z.strictObject({ favorite: z.boolean() });
export const reportIntentSchema = z.discriminatedUnion("operation", [z.strictObject({ operation: z.literal("save"), key: reportID, source: sourceRef }), z.strictObject({ operation: z.literal("favorite"), key: reportID, id: reportID, favorite: z.boolean() })]);
export type Report = z.infer<typeof reportSchema>;
export type ReportSource = z.infer<typeof reportSourceSchema>;
export type ReportIntent = z.infer<typeof reportIntentSchema>;
export type ReportScope = { userId: string; organizationId: string };
export function reportEndpoint(url: URL, method: string) {
  if (url.search.length > 1025 || url.hash || !url.pathname.startsWith("/api/report-center/reports")) return null;
  const suffix = url.pathname.slice("/api/report-center/reports".length);
  const parts = suffix === "" ? [] : suffix.startsWith("/") ? suffix.slice(1).split("/") : ["invalid"];
  const list = parts.length === 0 && method === "GET";
  const params = url.searchParams;
  for (const [key, value] of params) {
    if (!list || params.getAll(key).length !== 1 || !["view", "kind", "search", "cursor", "limit"].includes(key)) return null;
    if (key === "view" && !["all", "recent", "favorites"].includes(value) || key === "kind" && !reportKind.safeParse(value).success || key === "search" && (bytes(value) > 128 || /\p{Cc}/u.test(value)) || key === "cursor" && !/^[A-Za-z0-9_-]{1,512}$/.test(value) || key === "limit" && (!/^[1-9][0-9]?$/.test(value) || Number(value) > 50)) return null;
  }
  if (parts.length === 0) return method === "GET" ? { suffix, output: reportsPageSchema } : method === "POST" ? { suffix, output: reportResultSchema, input: reportSaveBody, operation: "save" } : null;
  if (parts.length === 1 && parts[0] === "summary" && method === "GET") return { suffix, output: reportsSummarySchema };
  if (parts.length === 3 && parts[0] === "sources" && reportKind.safeParse(parts[1]).success && reportID.safeParse(parts[2]).success && method === "GET") return { suffix, output: reportSourceSchema };
  if (!reportID.safeParse(parts[0]).success) return null;
  if (parts.length === 1 && method === "GET") return { suffix, output: reportSchema };
  if (parts.length === 2 && parts[1] === "favorite" && method === "POST") return { suffix, output: reportResultSchema, input: reportFavoriteBody, operation: "favorite" };
  return null;
}
export const sourceHref = (ref: ReportSource["ref"]) => ref.kind === "TITLE_REVIEW" ? `/workbench/ai/tasks/pending/other?proposal_id=${ref.id}` : `/workbench/shein-records/${ref.id}/diagnostic`;
export function reviewLocator(value: string, origin: string): string | null {
  const input = value.trim();
  if (reportID.safeParse(input).success) return input;
  if (input.length > 2048) return null;
  try {
    const url = new URL(input, origin);
    if (url.origin !== origin || url.username || url.password || url.hash || url.pathname !== "/workbench/ai/tasks/pending/other" || [...url.searchParams.keys()].length !== 1) return null;
    const id = url.searchParams.get("proposal_id"); return reportID.safeParse(id).success ? id : null;
  } catch { return null; }
}
export function reportText(report: Report) {
  return [report.title, "历史只读报告；不代表当前批准、执行或平台发布许可。", `来源：${report.ref.kind} / ${report.ref.id} / ${report.ref.version}`, `保存时间：${report.capturedAt}`, ...report.content.sections.flatMap(s => ["", s.title, ...s.fields.map(f => `${f.label}：${f.value}`)])].join("\n") + "\n";
}
