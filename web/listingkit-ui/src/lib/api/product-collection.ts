import { z } from "zod";
import { COLLECTION_MAX_BYTES, collectionID, collectionCommandSchema, collectionBatchesSchema, collectionItemsSchema, collectionDetailSchema, collectionReceiptSchema, type CollectionCommand } from "../contracts/product-collection";
import { readBoundedStrictJSON } from "./strict-json-response";

export type CollectionScope = Readonly<{ userId: string; organizationId: string }>;
export type CollectionIntent = Readonly<CollectionScope & { key: string; command: CollectionCommand }>;
export class CollectionAPIError extends Error {
  constructor(public readonly code: string, public readonly status: number) { super(code); }
}
const base = "/api/workbench/collections";
type Query = { after?: string; keyword?: string; limit?: number };
const queryString = (query: Query) => {
  const params = new URLSearchParams();
  if (query.after) params.set("after", query.after);
  if (query.keyword) params.set("keyword", query.keyword);
  params.set("limit", String(query.limit ?? 50));
  return `?${params}`;
};
export const listCollectionBatches = (scope: CollectionScope, query: Query = {}, signal?: AbortSignal) => send(`${base}/batches${queryString(query)}`, scope, collectionBatchesSchema, signal);
export const listOwnProducts = (scope: CollectionScope, query: Query = {}, signal?: AbortSignal) => send(`${base}/own-products${queryString(query)}`, scope, collectionItemsSchema, signal);
export function listCollectionItems(scope: CollectionScope, batchId: string, query: Query = {}, signal?: AbortSignal) {
  collectionID.parse(batchId);
  return send(`${base}/batches/${batchId}/items${queryString(query)}`, scope, collectionItemsSchema, signal);
}
export async function readCollectionItem(scope: CollectionScope, itemId: string, signal?: AbortSignal) {
  collectionID.parse(itemId);
  const detail = await send(`${base}/items/${itemId}`, scope, collectionDetailSchema, signal);
  if (detail.item.id !== itemId) throw new CollectionAPIError("DEPENDENCY_UNAVAILABLE", 502);
  return detail;
}
export function mutateCollection(intent: CollectionIntent, signal?: AbortSignal) {
  const key = collectionID.parse(intent.key);
  const command = collectionCommandSchema.parse(intent.command);
  return send(`${base}/commands`, intent, collectionReceiptSchema, signal, { key, command });
}
export function readCollectionOperation(intent: Pick<CollectionIntent, "key" | "userId" | "organizationId">, signal?: AbortSignal) {
  collectionID.parse(intent.key);
  return send(`${base}/by-key/${intent.key}`, intent, collectionReceiptSchema, signal);
}
async function send<T>(path: string, scope: CollectionScope, schema: z.ZodType<T>, signal?: AbortSignal, mutation?: { key: string; command: CollectionCommand }): Promise<T> {
  const boundedID = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
  if (!boundedID.test(scope.userId) || !boundedID.test(scope.organizationId)) throw new CollectionAPIError("INVALID_REQUEST", 400);
  if (signal?.aborted) throw new CollectionAPIError("DEADLINE_EXCEEDED", 504);
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  const timeout = setTimeout(abort, 17_000);
  const unavailable = () => new CollectionAPIError(mutation ? "OUTCOME_UNKNOWN" : "DEPENDENCY_UNAVAILABLE", mutation ? 503 : 502);
  try {
    const headers = new Headers({ Accept: "application/json", "X-Expected-Organization-ID": scope.organizationId, "X-Expected-User-ID": scope.userId });
    if (mutation) { headers.set("Content-Type", "application/json"); headers.set("Idempotency-Key", mutation.key); }
    const response = await fetch(path, { method: mutation ? "POST" : "GET", headers, body: mutation ? JSON.stringify(mutation.command) : undefined,
      credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    const payload = await readBoundedStrictJSON(response, COLLECTION_MAX_BYTES, controller.signal);
    if (!response.ok) {
      const error = z.object({ code: z.string().regex(/^[A-Z][A-Z0-9_]{0,79}$/) }).safeParse(payload);
      if (error.success) throw new CollectionAPIError(error.data.code, response.status);
      throw unavailable();
    }
    const checked = schema.safeParse(payload);
    if (response.status !== 200 || !checked.success) throw unavailable();
    return checked.data;
  } catch (error) {
    if (error instanceof CollectionAPIError) throw error;
    throw unavailable();
  } finally { clearTimeout(timeout); signal?.removeEventListener("abort", abort); }
}
