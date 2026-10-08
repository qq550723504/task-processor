import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";

export const notificationUUID = z.string().regex(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/).refine(v => v !== "00000000-0000-0000-0000-000000000000");
const boundedID = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const text = (max: number) => z.string().min(1).max(max).refine(v => !/[\u0000-\u001f\u007f]/.test(v));
const refSchema = z.object({ source: text(64), entityId: text(128), type: text(64), revision: text(128), organizationId: boundedID.or(z.literal("")) }).strict();
export const notificationRef = z.string().min(1).max(2048).regex(/^[A-Za-z0-9_-]+$/).refine(token => {
  try {
    const bytes = Uint8Array.from(atob(token.replace(/-/g, "+").replace(/_/g, "/")), c => c.charCodeAt(0));
    const ref = refSchema.parse(JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)));
    const canonical = btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(ref)))).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
    return canonical === token;
  } catch { return false; }
});
const targetSchema = z.object({ kind: text(32), id: boundedID.or(z.literal("")) }).strict();
export function notificationHref(target: z.infer<typeof targetSchema>): string | null {
  const staticRoutes: Record<string, string> = { none: "", home: "/workbench", resources: "/workbench/plans/entitlements", "member-resources": "/workbench/account/organization/resources", members: "/workbench/account/organization/members", "personal-verification": "/workbench/account/profile/verification", "organization-verification": "/workbench/account/organization", earnings: "/workbench/account/referrals/earnings", withdrawals: "/workbench/account/referrals/withdrawals" };
  if (target.id === "" && Object.hasOwn(staticRoutes, target.kind)) return staticRoutes[target.kind];
  if (!boundedID.safeParse(target.id).success) return null;
  const prefixes: Record<string, string> = { task: "/workbench/ai/tasks/", chat: "/workbench/ai/chat/", acquisition: "/workbench/supply/acquisition/operation/", store: "/workbench/stores/", order: "/workbench/plans/orders/", knowledge: "/workbench/ai/knowledge/", invitation: "/invitations/" };
  // Go url.PathEscape preserves ':' in bounded native identifiers.
  if (Object.hasOwn(prefixes, target.kind)) return prefixes[target.kind] + encodeURIComponent(target.id).replace(/%3A/g, ":");
  if (target.kind === "review") return "/workbench/ai/tasks/pending/other?proposal_id=" + encodeURIComponent(target.id);
  return null;
}
export const notificationItemSchema = z.object({
  id: notificationRef, category: z.enum(["official", "business"]), type: text(64), title: text(256), summary: text(1024),
  paragraphs: z.array(z.string().max(32768)).max(32), occurredAt: z.iso.datetime({ offset: true }).nullable(), source: text(64),
  organizationId: boundedID.or(z.literal("")), attention: z.boolean(), read: z.boolean(), target: targetSchema, href: z.string().max(512),
}).strict().refine(item => {
  if (item.href !== notificationHref(item.target)) return false;
  const ref = JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(item.id.replace(/-/g, "+").replace(/_/g, "/")), c => c.charCodeAt(0))));
  return ref.source === item.source && ref.type === item.type && ref.organizationId === item.organizationId;
});
const count = z.number().int().min(0).max(10000);
export const notificationListSchema = z.object({ schemaVersion: z.literal("notification-center-v1"), items: z.array(notificationItemSchema).max(100),
  coverage: z.array(z.object({ source: text(64), state: z.enum(["AVAILABLE", "EMPTY", "DENIED", "UNAVAILABLE", "DEPENDENCY_MISSING"]), complete: z.boolean(), observedAt: z.iso.datetime({ offset: true }) }).strict()).max(32),
  exact: z.boolean(), count, unread: count, pending: count, next: z.string().max(4096).regex(/^[A-Za-z0-9_-]*$/),
}).strict().refine(v => v.unread <= v.count && v.pending <= v.count && v.items.length <= v.count && v.exact === v.coverage.every(c => c.complete) && v.coverage.every(c => c.complete === (c.state !== "UNAVAILABLE")));
export const notificationSnapshotSchema = z.object({ id: notificationUUID, fingerprint: z.string().regex(/^[0-9a-f]{64}$/), expiresAt: z.iso.datetime({ offset: true }), count }).strict();
export const notificationCommandSchema = z.object({ key: notificationUUID, operation: z.enum(["read", "snapshot", "read-all", "publish", "withdraw"]), committedAt: z.iso.datetime({ offset: true }), resultId: notificationUUID.optional() }).strict();
export type NotificationItem = z.infer<typeof notificationItemSchema>;
export type NotificationList = z.infer<typeof notificationListSchema>;
export type NotificationChannel = "official" | "business" | "personal";
export type NotificationScope = { userId: string; channel: NotificationChannel; organizationId?: string };
export class NotificationError extends Error { constructor(public code: string, public status = 503) { super(code); } }
export async function notificationRequest<T>(scope: NotificationScope, path: string, schema: z.ZodType<T>, options: { signal?: AbortSignal; key?: string; body?: unknown } = {}): Promise<T> {
  const headers = new Headers({ "X-Expected-User-ID": scope.userId });
  if (scope.channel === "business" && scope.organizationId) headers.set("X-Expected-Organization-ID", scope.organizationId);
  if (options.key) { headers.set("Idempotency-Key", options.key); headers.set("Content-Type", "application/json"); }
  let dispatched = false;
  try {
    dispatched = true;
    const response = await fetch(`/api/notifications/${scope.channel}${path}`, { method: options.key ? "POST" : "GET", headers, body: options.key ? JSON.stringify(options.body) : undefined, signal: options.signal, cache: "no-store", redirect: "error" });
    const payload = await readBoundedStrictJSON(response, 128 * 1024, options.signal);
    if (!response.ok) {
      const error = z.object({ code: z.string().regex(/^[A-Z_]{1,80}$/) }).strict().safeParse(payload);
      throw new NotificationError(error.success ? error.data.code : options.key ? "OUTCOME_UNKNOWN" : "NOTIFICATION_UNAVAILABLE", response.status);
    }
    const value = schema.parse(payload);
    const wireSchema: z.ZodType = schema;
    if (wireSchema === notificationListSchema || wireSchema === notificationItemSchema) {
      const items = wireSchema === notificationListSchema ? (value as NotificationList).items : [value as NotificationItem];
      if (items.some(v => v.category !== (scope.channel === "official" ? "official" : "business") || v.organizationId !== "" && v.organizationId !== scope.organizationId)) throw new Error("context mismatch");
    }
    return value;
  } catch (error) {
    if (error instanceof NotificationError) throw error;
    throw new NotificationError(options.key && dispatched ? "OUTCOME_UNKNOWN" : "NOTIFICATION_UNAVAILABLE");
  }
}
